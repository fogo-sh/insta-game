package cloud

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

func TestCodeDeploymentCannotChangeGameInfrastructure(t *testing.T) {
	for _, id := range []string{"Host", "SecurityGroup", "HostRole", "Network"} {
		err := validateLobbyChanges([]cf.Change{{ResourceChange: &cf.ResourceChange{LogicalResourceId: aws.String(id), Action: cf.ChangeActionModify}}})
		if err == nil {
			t.Fatalf("code deployment permitted changes to %s", id)
		}
	}
	if err := validateLobbyChanges([]cf.Change{{ResourceChange: &cf.ResourceChange{LogicalResourceId: aws.String("LobbyFunction"), Action: cf.ChangeActionModify}}}); err != nil {
		t.Fatal(err)
	}
}
func TestNewParametersUseDefaultsWithoutReusingMissingValues(t *testing.T) {
	previous := &cf.Stack{StackStatus: cf.StackStatusCreateComplete, Parameters: []cf.Parameter{{ParameterKey: aws.String("LobbyArchitecture"), ParameterValue: aws.String("x86_64")}}}
	if p := preserveParameter(previous, "LobbyArchitecture", "arm64"); !aws.ToBool(p.UsePreviousValue) || p.ParameterValue != nil {
		t.Fatal("lost existing architecture")
	}
	if p := preserveParameter(previous, "LobbyCodeKey", ""); aws.ToBool(p.UsePreviousValue) || p.ParameterValue == nil {
		t.Fatal("reused a nonexistent parameter")
	}
}

func TestExistingHostImageStaysPinned(t *testing.T) {
	for _, parameter := range []cf.Parameter{
		{ParameterKey: aws.String("UbuntuImage"), ParameterValue: aws.String("/public/latest/ubuntu"), ResolvedValue: aws.String("ami-0123456789abcdef0")},
		{ParameterKey: aws.String("UbuntuImage"), ParameterValue: aws.String("ami-0123456789abcdef0")},
	} {
		stack := &cf.Stack{StackStatus: cf.StackStatusCreateComplete, Parameters: []cf.Parameter{parameter, {ParameterKey: aws.String("InstanceType"), ParameterValue: aws.String("t4g.small")}}}
		image, err := hostImage(context.Background(), aws.Config{}, stack, "arm64")
		if err != nil || image != "ami-0123456789abcdef0" {
			t.Fatalf("existing AMI changed: %s %v", image, err)
		}
		if _, err := hostImage(context.Background(), aws.Config{}, stack, "amd64"); err == nil {
			t.Fatal("architecture change would replace the host")
		}
	}
}
