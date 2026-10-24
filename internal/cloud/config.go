package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"
)

type Config struct {
	Region         string
	Stack          string
	InstanceID     string
	KeyPath        string
	TokenPath      string
	LobbyTable     string `json:",omitempty"`
	LobbyInstances string `json:",omitempty"`
	LobbyURL       string `json:",omitempty"`
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
	if c.Region == "" || c.Stack == "" || c.InstanceID == "" {
		return c, fmt.Errorf("incomplete shared EC2 host config")
	}
	for _, p := range []*string{&c.KeyPath, &c.TokenPath} {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(filepath.Dir(path), *p)
		}
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

func DescribeStack(ctx context.Context, client StackDescriber, name string) (*cf.Stack, error) {
	result, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if len(result.Stacks) != 1 {
		return nil, fmt.Errorf("expected one stack for %s", name)
	}
	stack := &result.Stacks[0]
	if err := requireStackOwnership(stack); err != nil {
		return nil, err
	}
	return stack, nil
}

func requireStackOwnership(stack *cf.Stack) error {
	foundKind := false
	for _, tag := range stack.Tags {
		if aws.ToString(tag.Key) == "insta-game:kind" && aws.ToString(tag.Value) == "shared-host" {
			foundKind = true
		}
	}
	if !foundKind {
		return fmt.Errorf("refusing unmanaged CloudFormation stack %s", aws.ToString(stack.StackName))
	}
	return nil
}

func Outputs(stack *cf.Stack) map[string]string {
	result := map[string]string{}
	for _, output := range stack.Outputs {
		result[aws.ToString(output.OutputKey)] = aws.ToString(output.OutputValue)
	}
	return result
}
