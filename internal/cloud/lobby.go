package cloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var ErrLobbyCurrent = errors.New("launch page already uses this binary")

type LobbyOptions struct {
	Binary  string
	Disable bool
}

// PlanLobby prepares a reviewed Lambda code or fleet configuration update.
func PlanLobby(ctx context.Context, sdk aws.Config, name string, options LobbyOptions) (string, error) {
	stacks := cloudformation.NewFromConfig(sdk)
	stack, err := DescribeStack(ctx, stacks, name)
	if err != nil {
		return "", err
	}
	bucket := Outputs(stack)["LobbyCodeBucket"]
	if bucket == "" {
		return "", fmt.Errorf("provision --lobby-instances first")
	}
	updates := map[string]string{}
	if options.Disable {
		if options.Binary != "" {
			return "", fmt.Errorf("disable cannot be combined with binary or host configuration")
		}
		updates["LobbyCodeKey"] = ""
	} else {
		architecture := preparedArchitecture(options.Binary)
		if architecture == "" {
			return "", fmt.Errorf("Lambda requires a static Linux ARM64 or AMD64 binary; build with CGO_ENABLED=0")
		}
		input, err := os.Open(options.Binary)
		if err != nil {
			return "", err
		}
		defer input.Close()
		packed, err := lambdaArchive(input)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(packed)
		key := "bootstrap-" + hex.EncodeToString(sum[:]) + ".zip"
		var previousKey, previousArchitecture string
		for _, parameter := range stack.Parameters {
			switch aws.ToString(parameter.ParameterKey) {
			case "LobbyCodeKey":
				previousKey = aws.ToString(parameter.ParameterValue)
			case "LobbyArchitecture":
				previousArchitecture = aws.ToString(parameter.ParameterValue)
			}
		}
		if previousKey == key && previousArchitecture == architecture && len(updates) == 0 {
			return "", ErrLobbyCurrent
		}

		if _, err := s3.NewFromConfig(sdk).PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(packed)}); err != nil {
			return "", err
		}
		updates["LobbyCodeKey"], updates["LobbyArchitecture"] = key, architecture
	}
	params, err := updateParameters(EnvironmentTemplate, stack.Parameters, updates)
	if err != nil {
		return "", err
	}
	result, err := stacks.CreateChangeSet(ctx, &cloudformation.CreateChangeSetInput{StackName: stack.StackId, ChangeSetName: aws.String(fmt.Sprintf("insta-game-lobby-%d", time.Now().UnixNano())), ChangeSetType: cf.ChangeSetTypeUpdate, TemplateBody: aws.String(EnvironmentTemplate), Capabilities: []cf.Capability{cf.CapabilityCapabilityIam}, Parameters: params, Tags: stack.Tags})
	if err != nil {
		return "", err
	}
	if err := cloudformation.NewChangeSetCreateCompleteWaiter(stacks).Wait(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: result.Id}, 5*time.Minute); err != nil {
		return "", err
	}
	changes, err := stacks.DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: result.Id})
	if err != nil {
		return "", err
	}
	if err := validateLobbyChanges(changes.Changes); err != nil {
		return "", err
	}
	return aws.ToString(result.Id), nil
}

// CloudFormation cannot delete a nonempty artifact bucket. Only empty the bucket named by this owned stack.
func emptyLobbyCode(ctx context.Context, sdk aws.Config, stack *cf.Stack) error {
	bucket := Outputs(stack)["LobbyCodeBucket"]
	if bucket == "" {
		return nil
	}
	client := s3.NewFromConfig(sdk)
	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			var api smithy.APIError
			if errors.As(err, &api) && api.ErrorCode() == "NoSuchBucket" {
				return nil
			}
			return err
		}
		for _, object := range page.Contents {
			if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: object.Key}); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLobbyChanges(changes []cf.Change) error {
	for _, change := range changes {
		if r := change.ResourceChange; r != nil && !strings.HasPrefix(aws.ToString(r.LogicalResourceId), "Lobby") {
			return fmt.Errorf("refusing a lobby code update that changes %s; review infrastructure separately with aws plan", aws.ToString(r.LogicalResourceId))
		}
	}
	return nil
}
