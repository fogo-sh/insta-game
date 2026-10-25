package cloud

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/smithy-go"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fogo-sh/insta-game/internal/lobby"
)

var ErrLobbyCurrent = errors.New("launch page already uses this binary")

type LobbyOptions struct {
	Binary, HostsFile string
	Disable           bool
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
		if options.Binary != "" || options.HostsFile != "" {
			return "", fmt.Errorf("disable cannot be combined with binary or host configuration")
		}
		updates["LobbyCodeKey"] = ""
	} else {
		if options.HostsFile != "" {
			data, err := os.ReadFile(options.HostsFile)
			if err != nil {
				return "", err
			}
			var specs []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Stack string `json:"stack"`
			}
			if err = json.Unmarshal(data, &specs); err != nil {
				return "", err
			}
			bindings := []lobby.Binding{}
			hosts, tables := []string{}, []string{}
			for _, spec := range specs {
				c, err := ReadConfig(ctx, sdk, spec.Stack)
				if err != nil {
					return "", err
				}
				if c.LobbyTable == "" {
					return "", fmt.Errorf("host %s has no lobby table", spec.ID)
				}
				parts := strings.Split(c.Stack, ":")
				primary := strings.Split(aws.ToString(stack.StackId), ":")
				if len(parts) != 6 || parts[3] != sdk.Region || parts[4] != primary[4] {
					return "", fmt.Errorf("fleet hosts must share a region and account")
				}
				host := &lobby.Host{Client: ec2.NewFromConfig(sdk), InstanceID: c.InstanceID, StackID: c.Stack}
				if _, err := host.State(ctx); err != nil {
					return "", err
				}
				ids, err := lobby.ParseIDs(c.LobbyInstances)
				if err != nil {
					return "", err
				}
				bindings = append(bindings, lobby.Binding{ID: spec.ID, Name: spec.Name, InstanceID: c.InstanceID, StackID: c.Stack, Table: c.LobbyTable, IDs: ids})
				hosts = append(hosts, fmt.Sprintf("arn:%s:ec2:%s:%s:instance/%s", parts[1], parts[3], parts[4], c.InstanceID))
				tables = append(tables, fmt.Sprintf("arn:%s:dynamodb:%s:%s:table/%s", parts[1], parts[3], parts[4], c.LobbyTable))
			}
			encoded, err := json.Marshal(bindings)
			if err != nil {
				return "", err
			}
			if _, err = lobby.ParseBindings(string(encoded)); err != nil {
				return "", err
			}
			if len(encoded) > 4096 {
				return "", fmt.Errorf("fleet configuration exceeds CloudFormation's parameter limit")
			}
			updates["LobbyHosts"], updates["LobbyHostResources"], updates["LobbyTableResources"] = string(encoded), strings.Join(hosts, ","), strings.Join(tables, ",")
		}
		executable, err := elf.Open(options.Binary)
		if err != nil {
			return "", err
		}
		for _, program := range executable.Progs {
			if program.Type == elf.PT_INTERP {
				executable.Close()
				return "", fmt.Errorf("Lambda requires a static binary; build with CGO_ENABLED=0")
			}
		}
		architecture := ""
		switch executable.Machine {
		case elf.EM_AARCH64:
			architecture = "arm64"
		case elf.EM_X86_64:
			architecture = "x86_64"
		}
		executable.Close()
		if architecture == "" {
			return "", fmt.Errorf("lobby requires a Linux ARM64 or AMD64 binary")
		}
		input, err := os.Open(options.Binary)
		if err != nil {
			return "", err
		}
		defer input.Close()
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
		header.SetMode(0755)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(entry, input); err != nil {
			return "", err
		}
		if err := archive.Close(); err != nil {
			return "", err
		}
		sum := sha256.Sum256(buffer.Bytes())
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

		if _, err := s3.NewFromConfig(sdk).PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: bytes.NewReader(buffer.Bytes())}); err != nil {
			return "", err
		}
		updates["LobbyCodeKey"], updates["LobbyArchitecture"] = key, architecture
	}
	params := make([]cf.Parameter, 0, len(stack.Parameters))
	for _, p := range stack.Parameters {
		param := cf.Parameter{ParameterKey: p.ParameterKey, UsePreviousValue: aws.Bool(true)}
		if value, ok := updates[aws.ToString(p.ParameterKey)]; ok {
			param.UsePreviousValue = nil
			param.ParameterValue = aws.String(value)
			delete(updates, aws.ToString(p.ParameterKey))
		}
		params = append(params, param)
	}
	for key, value := range updates {
		params = append(params, cf.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(value)})
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
