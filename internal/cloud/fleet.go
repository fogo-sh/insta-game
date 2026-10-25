package cloud

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/fogo-sh/insta-game/internal/fleet"
)

//go:embed fleet.json
var FleetTemplate string

func PlanFleet(ctx context.Context, sdk aws.Config, name string, options PlanOptions) (string, error) {
	if !stackName.MatchString(name) || options.PublicKey == "" || options.OperatorCIDR == "" || options.DiskGiB < 24 {
		return "", fmt.Errorf("fleet requires a valid stack, operator CIDR, SSH public key, and disk >=24 GiB")
	}
	if options.LobbyTokenHash == "" {
		return "", fmt.Errorf("fleet requires an invitation key")
	}
	if options.MaxHosts < 1 || options.MaxHosts > 16 {
		return "", fmt.Errorf("max-hosts must be 1–16")
	}
	ip, network, err := net.ParseCIDR(options.OperatorCIDR)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("operator-cidr must be IPv4")
	}
	if prefix, _ := network.Mask.Size(); prefix < 24 {
		return "", fmt.Errorf("operator-cidr must be /24 or narrower")
	}
	if ip, _, err := net.ParseCIDR(options.GameCIDR); err != nil || ip.To4() == nil {
		return "", fmt.Errorf("game-cidr must be IPv4")
	}
	if options.DiskGiB > 256 || options.UDPFrom < 1 || options.UDPTo < options.UDPFrom || options.UDPTo > 65535 {
		return "", fmt.Errorf("invalid disk or game port range")
	}
	switch options.InstanceType {
	case "t4g.micro", "t4g.small", "t4g.medium", "t3a.micro", "t3a.small", "t3a.medium", "t3.micro", "t3.small", "t3.medium":
	default:
		return "", fmt.Errorf("fleet supports micro, small, or medium hosts")
	}
	client := cloudformation.NewFromConfig(sdk)
	kind := cf.ChangeSetTypeCreate
	var previous *cf.Stack
	stack, err := describeEnvironment(ctx, client, name)
	if err == nil {
		previous = stack
		released := false
		oldType, oldDisk := "", ""
		for _, p := range stack.Parameters {
			switch aws.ToString(p.ParameterKey) {
			case "Release":
				released = aws.ToString(p.ParameterValue) != ""
			case "InstanceType":
				oldType = aws.ToString(p.ParameterValue)
			case "DiskGiB":
				oldDisk = aws.ToString(p.ParameterValue)
			}
		}
		if released && (oldType != options.InstanceType || oldDisk != fmt.Sprint(options.DiskGiB)) {
			return "", fmt.Errorf("hardware changes require a separate fleet; existing capacities and saved data remain pinned")
		}

		if Outputs(stack)["Fleet"] != "true" && stack.StackStatus != cf.StackStatusReviewInProgress {
			return "", fmt.Errorf("use a new stack for the fleet; existing saved hosts will not be replaced")
		}
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
	image, err := hostImage(ctx, sdk, previous, architecture)
	if err != nil {
		return "", err
	}
	params := []cf.Parameter{}
	values := map[string]string{"UbuntuImage": image, "OperatorCIDR": options.OperatorCIDR, "GameCIDR": options.GameCIDR, "PublicKey": options.PublicKey, "InstanceType": options.InstanceType, "DiskGiB": fmt.Sprint(options.DiskGiB), "UDPFrom": fmt.Sprint(options.UDPFrom), "UDPTo": fmt.Sprint(options.UDPTo), "LobbyTokenHash": options.LobbyTokenHash, "MaxHosts": fmt.Sprint(options.MaxHosts)}
	for k, v := range values {
		params = append(params, cf.Parameter{ParameterKey: aws.String(k), ParameterValue: aws.String(v)})
	}
	for _, k := range []string{"LobbyCodeKey", "Release", "LobbyArchitecture"} {
		value := ""
		if k == "LobbyArchitecture" {
			value = "arm64"
		}
		params = append(params, preserveParameter(previous, k, value))
	}
	out, err := client.CreateChangeSet(ctx, &cloudformation.CreateChangeSetInput{StackName: aws.String(name), ChangeSetName: aws.String(fmt.Sprintf("insta-game-%d", time.Now().UnixNano())), ChangeSetType: kind, TemplateBody: aws.String(FleetTemplate), Parameters: params, Capabilities: []cf.Capability{cf.CapabilityCapabilityIam}, Tags: []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("shared-host")}}})
	if err != nil {
		return "", err
	}
	if err = cloudformation.NewChangeSetCreateCompleteWaiter(client).Wait(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: out.Id}, 5*time.Minute); err != nil {
		return "", err
	}
	return aws.ToString(out.Id), nil
}

func destroyFleetHosts(ctx context.Context, sdk aws.Config, stack *cf.Stack) error {
	table := &fleet.Table{Client: dynamodb.NewFromConfig(sdk), Name: Outputs(stack)["LobbyTable"]}
	state, err := table.Load(ctx)
	if err != nil {
		return err
	}
	state.Deleting = true
	if err = table.Save(ctx, state, state.Revision); err != nil {
		return err
	}
	scheduler := &fleet.Scheduler{Store: table, Machines: &fleet.EC2{Client: ec2.NewFromConfig(sdk), Stack: aws.ToString(stack.StackId)}}
	for {
		state, err = table.Load(ctx)
		if err != nil {
			return err
		}
		if len(state.Hosts) == 0 {
			return nil
		}
		err = scheduler.Reconcile(ctx)
		if err != nil {
			log.Printf("waiting for fleet cleanup: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}
