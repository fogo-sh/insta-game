package cloud

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func hostImage(ctx context.Context, sdk aws.Config, previous *cf.Stack, architecture string) (string, error) {
	if previous != nil && previous.StackStatus != cf.StackStatusReviewInProgress {
		var image, instanceType string
		for _, parameter := range previous.Parameters {
			switch aws.ToString(parameter.ParameterKey) {
			case "UbuntuImage":
				image = aws.ToString(parameter.ResolvedValue)
				if image == "" {
					image = aws.ToString(parameter.ParameterValue)
				}
			case "InstanceType":
				instanceType = aws.ToString(parameter.ParameterValue)
			}
		}
		if strings.HasPrefix(instanceType, "t4g.") != (architecture == "arm64") {
			return "", fmt.Errorf("changing host architecture requires a separate stack")
		}
		if !strings.HasPrefix(image, "ami-") {
			return "", fmt.Errorf("existing stack has no pinned host image")
		}
		return image, nil
	}
	result, err := ssm.NewFromConfig(sdk).GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String("/aws/service/canonical/ubuntu/server/24.04/stable/current/" + architecture + "/hvm/ebs-gp3/ami-id")})
	if err != nil {
		return "", err
	}
	if result.Parameter == nil || !strings.HasPrefix(aws.ToString(result.Parameter.Value), "ami-") {
		return "", fmt.Errorf("Ubuntu image parameter contains no AMI")
	}
	return aws.ToString(result.Parameter.Value), nil
}
