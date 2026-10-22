package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"
)

type Config struct {
	Network       string
	Region        string
	Stack         string
	Cluster       string
	Subnet        string
	SecurityGroup string
	FileSystem    string
	ExecutionRole string
	LogGroup      string
	AssetsBucket  string
	Repository    string
}

func LoadSDK(ctx context.Context, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

func Load(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Network == "" || c.Region == "" || c.Stack == "" || c.Cluster == "" || c.Subnet == "" || c.SecurityGroup == "" || c.FileSystem == "" || c.ExecutionRole == "" || c.LogGroup == "" || c.AssetsBucket == "" {
		return c, fmt.Errorf("incomplete AWS environment config")
	}
	return c, nil
}

func MissingStack(err error) bool {
	var api smithy.APIError
	return errors.As(err, &api) && api.ErrorCode() == "ValidationError" && strings.Contains(api.ErrorMessage(), "does not exist")
}

type StackDescriber interface {
	DescribeStacks(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

func DescribeStack(ctx context.Context, client StackDescriber, name, kind, owner string) (*cf.Stack, error) {
	result, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if len(result.Stacks) != 1 {
		return nil, fmt.Errorf("expected one stack for %s", name)
	}
	stack := &result.Stacks[0]
	foundKind, foundOwner := false, owner == ""
	for _, tag := range stack.Tags {
		if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == kind {
			foundKind = true
		}
		if aws.ToString(tag.Key) == "insta-game:owner" && aws.ToString(tag.Value) == owner {
			foundOwner = true
		}
	}
	if !foundKind || !foundOwner {
		return nil, fmt.Errorf("refusing unmanaged CloudFormation stack %s", name)
	}
	return stack, nil
}

func Outputs(stack *cf.Stack) map[string]string {
	result := map[string]string{}
	for _, output := range stack.Outputs {
		result[aws.ToString(output.OutputKey)] = aws.ToString(output.OutputValue)
	}
	return result
}
