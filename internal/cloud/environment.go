package cloud

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

//go:embed environment.json
var EnvironmentTemplate string

var stackName = regexp.MustCompile(`^insta-game-[a-zA-Z0-9-]{1,60}$`)

type PlanOptions struct {
	OperatorCIDR string
	GameCIDR     string
	PublicKey    string
	InstanceType string
	DiskGiB      int
	UDPFrom      int
	UDPTo        int
}

func Plan(ctx context.Context, sdk aws.Config, name string, options PlanOptions) (string, error) {
	if !stackName.MatchString(name) {
		return "", fmt.Errorf("stack name must start with insta-game- and contain letters, numbers or hyphens")
	}
	ip, network, err := net.ParseCIDR(options.OperatorCIDR)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("operator-cidr must be an IPv4 CIDR")
	}
	if ones, _ := network.Mask.Size(); ones < 24 {
		return "", fmt.Errorf("operator-cidr must be /24 or narrower")
	}
	if ip, _, err := net.ParseCIDR(options.GameCIDR); err != nil || ip.To4() == nil {
		return "", fmt.Errorf("game-cidr must be an IPv4 CIDR")
	}
	if options.PublicKey == "" || options.InstanceType == "" || options.DiskGiB < 8 || options.UDPFrom < 1 || options.UDPTo < options.UDPFrom || options.UDPTo > 65535 {
		return "", fmt.Errorf("public key, instance type, disk >= 8 GiB, and valid UDP range are required")
	}
	client := cloudformation.NewFromConfig(sdk)
	kind := cf.ChangeSetTypeCreate
	if stack, err := describeEnvironment(ctx, client, name); err == nil {
		if stack.StackStatus != cf.StackStatusReviewInProgress {
			kind = cf.ChangeSetTypeUpdate
		}
	} else if !MissingStack(err) {
		return "", err
	}
	architecture := "amd64"
	if strings.HasPrefix(options.InstanceType, "t4g.") {
		architecture = "arm64"
	}
	result, err := client.CreateChangeSet(ctx, &cloudformation.CreateChangeSetInput{
		StackName: aws.String(name), ChangeSetName: aws.String(fmt.Sprintf("insta-game-%d", time.Now().UnixNano())),
		ChangeSetType: kind, TemplateBody: aws.String(EnvironmentTemplate),
		Parameters: []cf.Parameter{
			{ParameterKey: aws.String("UbuntuImage"), ParameterValue: aws.String("/aws/service/canonical/ubuntu/server/24.04/stable/current/" + architecture + "/hvm/ebs-gp3/ami-id")},
			{ParameterKey: aws.String("GameCIDR"), ParameterValue: aws.String(options.GameCIDR)},
			{ParameterKey: aws.String("OperatorCIDR"), ParameterValue: aws.String(options.OperatorCIDR)},
			{ParameterKey: aws.String("PublicKey"), ParameterValue: aws.String(options.PublicKey)},
			{ParameterKey: aws.String("InstanceType"), ParameterValue: aws.String(options.InstanceType)},
			{ParameterKey: aws.String("DiskGiB"), ParameterValue: aws.String(fmt.Sprint(options.DiskGiB))},
			{ParameterKey: aws.String("UDPFrom"), ParameterValue: aws.String(fmt.Sprint(options.UDPFrom))},
			{ParameterKey: aws.String("UDPTo"), ParameterValue: aws.String(fmt.Sprint(options.UDPTo))},
		},
		Tags: []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("shared-host")}},
	})
	if err != nil {
		return "", err
	}
	waiter := cloudformation.NewChangeSetCreateCompleteWaiter(client)
	if err := waiter.Wait(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: result.Id}, 5*time.Minute); err != nil {
		details, describeErr := client.DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: result.Id})
		if describeErr == nil && details.Status == cf.ChangeSetStatusFailed {
			return "", fmt.Errorf("change set %s: %s", aws.ToString(result.Id), aws.ToString(details.StatusReason))
		}
		return "", fmt.Errorf("change set %s: %w", aws.ToString(result.Id), err)
	}
	return aws.ToString(result.Id), nil
}

func Apply(ctx context.Context, sdk aws.Config, name, changeSet string) error {
	client := cloudformation.NewFromConfig(sdk)
	changes, err := client.DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: aws.String(changeSet), StackName: aws.String(name)})
	if err != nil {
		return err
	}
	managed := false
	for _, tag := range changes.Tags {
		if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == "shared-host" {
			managed = true
		}
	}
	if !managed {
		return fmt.Errorf("refusing unmanaged change set")
	}
	described, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(name)})
	if err != nil {
		return err
	}
	if len(described.Stacks) != 1 {
		return fmt.Errorf("expected one environment stack")
	}
	stack := &described.Stacks[0]
	// CREATE change sets have no resources or stack tags until execution.
	if stack.StackStatus != cf.StackStatusReviewInProgress {
		if err := requireStackOwnership(stack); err != nil {
			return err
		}
	}
	if aws.ToString(changes.StackId) != aws.ToString(stack.StackId) {
		return fmt.Errorf("change set belongs to a different environment")
	}
	if _, err := client.ExecuteChangeSet(ctx, &cloudformation.ExecuteChangeSetInput{ChangeSetName: changes.ChangeSetId, StackName: stack.StackId}); err != nil {
		return err
	}
	input := &cloudformation.DescribeStacksInput{StackName: stack.StackId}
	if stack.StackStatus == cf.StackStatusReviewInProgress {
		return cloudformation.NewStackCreateCompleteWaiter(client).Wait(ctx, input, 20*time.Minute)
	}
	return cloudformation.NewStackUpdateCompleteWaiter(client).Wait(ctx, input, 20*time.Minute)
}

func ReadConfig(ctx context.Context, sdk aws.Config, name string) (Config, error) {
	stack, err := DescribeStack(ctx, cloudformation.NewFromConfig(sdk), name)
	if err != nil {
		return Config{}, err
	}
	if stack.StackStatus != cf.StackStatusCreateComplete && stack.StackStatus != cf.StackStatusUpdateComplete {
		return Config{}, fmt.Errorf("environment is %s", stack.StackStatus)
	}
	o := Outputs(stack)
	return Config{Region: sdk.Region, Stack: aws.ToString(stack.StackId), InstanceID: o["InstanceID"]}, nil
}

func Destroy(ctx context.Context, sdk aws.Config, name string) error {
	client := cloudformation.NewFromConfig(sdk)
	stack, err := describeEnvironment(ctx, client, name)
	if MissingStack(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := client.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: stack.StackId}); err != nil {
		return err
	}
	return cloudformation.NewStackDeleteCompleteWaiter(client).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: stack.StackId}, 15*time.Minute)
}

func describeEnvironment(ctx context.Context, client *cloudformation.Client, name string) (*cf.Stack, error) {
	result, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if len(result.Stacks) != 1 {
		return nil, fmt.Errorf("expected one environment stack")
	}
	stack := &result.Stacks[0]
	if stack.StackStatus != cf.StackStatusReviewInProgress {
		if err := requireStackOwnership(stack); err != nil {
			return nil, err
		}
		return stack, nil
	}
	// Unexecuted CREATE plans carry ownership tags on the change set only.
	pages := cloudformation.NewListChangeSetsPaginator(client, &cloudformation.ListChangeSetsInput{StackName: stack.StackId})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, summary := range page.Summaries {
			change, err := client.DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: summary.ChangeSetId, StackName: stack.StackId})
			if err != nil {
				return nil, err
			}
			if aws.ToString(change.StackId) != aws.ToString(stack.StackId) {
				continue
			}
			for _, tag := range change.Tags {
				if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == "shared-host" {
					return stack, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("refusing unmanaged review stack %s", name)
}
