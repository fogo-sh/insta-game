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
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

//go:embed environment.json
var EnvironmentTemplate string

var stackName = regexp.MustCompile(`^insta-game-[a-zA-Z0-9-]{1,60}$`)

func Plan(ctx context.Context, sdk aws.Config, name, controllerCIDR string) (string, error) {
	if !stackName.MatchString(name) {
		return "", fmt.Errorf("stack name must start with insta-game- and contain letters, numbers or hyphens")
	}
	ip, network, err := net.ParseCIDR(controllerCIDR)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("controller-cidr must be an IPv4 CIDR")
	}
	if ones, _ := network.Mask.Size(); ones < 24 {
		return "", fmt.Errorf("controller-cidr must be /24 or narrower")
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
	result, err := client.CreateChangeSet(ctx, &cloudformation.CreateChangeSetInput{
		StackName: aws.String(name), ChangeSetName: aws.String(fmt.Sprintf("insta-game-%d", time.Now().UnixNano())),
		ChangeSetType: kind, TemplateBody: aws.String(EnvironmentTemplate), Capabilities: []cf.Capability{cf.CapabilityCapabilityIam},
		Parameters: []cf.Parameter{{ParameterKey: aws.String("ControllerCIDR"), ParameterValue: aws.String(controllerCIDR)}},
		Tags:       []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("environment")}},
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
		if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == "environment" {
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
		if _, err := DescribeStack(ctx, client, name, "environment", ""); err != nil {
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
	stack, err := DescribeStack(ctx, cloudformation.NewFromConfig(sdk), name, "environment", "")
	if err != nil {
		return Config{}, err
	}
	if stack.StackStatus != cf.StackStatusCreateComplete && stack.StackStatus != cf.StackStatusUpdateComplete {
		return Config{}, fmt.Errorf("environment is %s", stack.StackStatus)
	}
	o := Outputs(stack)
	return Config{Region: sdk.Region, Stack: aws.ToString(stack.StackId), Network: o["Network"], Cluster: o["Cluster"], Subnet: o["Subnet"], SecurityGroup: o["SecurityGroup"], FileSystem: o["FileSystem"], ExecutionRole: o["ExecutionRole"], LogGroup: o["LogGroup"], AssetsBucket: o["AssetsBucket"], Repository: o["Repository"]}, nil
}

func Destroy(ctx context.Context, sdk aws.Config, name string, deleteAssets bool) error {
	cfClient := cloudformation.NewFromConfig(sdk)
	stack, err := describeEnvironment(ctx, cfClient, name)
	if MissingStack(err) {
		return nil
	}
	if err != nil {
		return err
	}
	o := Outputs(stack)
	if o["Cluster"] != "" {
		tasks, err := ecs.NewFromConfig(sdk).ListTasks(ctx, &ecs.ListTasksInput{Cluster: aws.String(o["Cluster"])})
		if err != nil {
			return err
		}
		if len(tasks.TaskArns) > 0 {
			return fmt.Errorf("delete game instances before destroying their environment")
		}
	}
	pages := cloudformation.NewListStacksPaginator(cfClient, &cloudformation.ListStacksInput{})
	for pages.HasMorePages() {
		result, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, summary := range result.StackSummaries {
			if summary.StackStatus == cf.StackStatusDeleteComplete || !strings.HasPrefix(aws.ToString(summary.StackName), aws.ToString(stack.StackName)+"-game-") {
				continue
			}
			return fmt.Errorf("game stack %s still exists; delete its instance first", aws.ToString(summary.StackName))
		}
	}
	if o["AssetsBucket"] != "" {
		s3client := s3.NewFromConfig(sdk)
		pages := s3.NewListObjectsV2Paginator(s3client, &s3.ListObjectsV2Input{Bucket: aws.String(o["AssetsBucket"])})
		for pages.HasMorePages() {
			result, err := pages.NextPage(ctx)
			if err != nil {
				return err
			}
			if len(result.Contents) > 0 && !deleteAssets {
				return fmt.Errorf("asset bucket is not empty; pass --delete-assets to delete its contents")
			}
			objects := []s3types.ObjectIdentifier{}
			for _, object := range result.Contents {
				objects = append(objects, s3types.ObjectIdentifier{Key: object.Key})
			}
			if len(objects) > 0 {
				deleted, err := s3client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(o["AssetsBucket"]), Delete: &s3types.Delete{Objects: objects}})
				if err != nil {
					return err
				}
				if len(deleted.Errors) > 0 {
					return fmt.Errorf("asset deletion failed: %s", aws.ToString(deleted.Errors[0].Message))
				}
			}
		}
	}
	if _, err := cfClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: stack.StackId}); err != nil {
		return err
	}
	return cloudformation.NewStackDeleteCompleteWaiter(cfClient).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: stack.StackId}, 20*time.Minute)
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
		return DescribeStack(ctx, client, name, "environment", "")
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
				if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == "environment" {
					return stack, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("refusing unmanaged review stack %s", name)
}
