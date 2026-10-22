package command

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fogo-sh/insta-game/internal/cloud"
)

func awsCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("aws requires plan, apply, config, upload, push, or destroy")
	}
	action := args[0]
	flags := flag.NewFlagSet("aws "+action, flag.ContinueOnError)
	flags.SetOutput(out)
	region := flags.String("region", "ca-central-1", "AWS region")
	name := flags.String("stack", "insta-game-test", "Owned environment stack name")
	cidr := flags.String("controller-cidr", "", "Controller public IPv4 CIDR for TLS management")
	changeSet := flags.String("change-set", "", "Reviewed change set ARN")
	configFile := flags.String("config", "", "AWS environment JSON file")
	output := flags.String("out", "", "Write environment config here")
	file := flags.String("file", "", "Local asset archive")
	key := flags.String("key", "", "Private S3 object key")
	image := flags.String("image", "", "Local image to publish")
	tag := flags.String("tag", "", "Image tag")
	deleteAssets := flags.Bool("delete-assets", false, "Delete uploaded assets during environment destruction")
	if err := flags.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	var environment cloud.Config
	if *configFile != "" {
		c, err := cloud.Load(*configFile)
		if err != nil {
			return err
		}
		environment = c
		*region = c.Region
		explicitStack := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "stack" {
				explicitStack = true
			}
		})
		if explicitStack {
			parts := strings.Split(c.Stack, "/")
			if *name != c.Stack && (len(parts) < 2 || *name != parts[1]) {
				return fmt.Errorf("--stack conflicts with the environment config")
			}
		} else {
			*name = c.Stack
		}
	}
	sdk, err := cloud.LoadSDK(ctx, *region)
	if err != nil {
		return err
	}
	switch action {
	case "plan":
		if parts := strings.Split(*name, "/"); strings.HasPrefix(*name, "arn:") && len(parts) == 3 {
			*name = parts[1]
		}
		id, err := cloud.Plan(ctx, sdk, *name, *cidr)
		if err != nil {
			return err
		}
		result, err := cloudformation.NewFromConfig(sdk).DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: aws.String(id)})
		if err != nil {
			return err
		}
		fmt.Fprintln(out, id)
		for _, change := range result.Changes {
			if change.ResourceChange != nil {
				fmt.Fprintf(out, "%s %s (%s)\n", change.ResourceChange.Action, aws.ToString(change.ResourceChange.LogicalResourceId), aws.ToString(change.ResourceChange.ResourceType))
			}
		}
		return nil
	case "apply":
		if *changeSet == "" {
			return fmt.Errorf("apply requires --change-set from aws plan")
		}
		if err := cloud.Apply(ctx, sdk, *name, *changeSet); err != nil {
			return err
		}
		fallthrough
	case "config":
		config, err := cloud.ReadConfig(ctx, sdk, *name)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		if *output != "" {
			return os.WriteFile(*output, append(data, '\n'), 0600)
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	case "destroy":
		return cloud.Destroy(ctx, sdk, *name, *deleteAssets)
	case "upload":
		if environment.AssetsBucket == "" || *file == "" || *key == "" || strings.ContainsAny(*key, "=;\r\n") {
			return fmt.Errorf("upload requires --config, --file and --key (without = or ;)")
		}
		input, err := os.Open(*file)
		if err != nil {
			return err
		}
		defer input.Close()
		info, err := input.Stat()
		if err != nil {
			return err
		}
		_, err = s3.NewFromConfig(sdk).PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(environment.AssetsBucket), Key: key, Body: input, ContentLength: aws.Int64(info.Size())})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "s3://%s/%s\n", environment.AssetsBucket, *key)
		return err
	case "push":
		if environment.Repository == "" || *image == "" || *tag == "" || strings.ContainsAny(*tag, "/\r\n ") {
			return fmt.Errorf("push requires --config, --image and --tag")
		}
		result, err := ecr.NewFromConfig(sdk).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
		if err != nil {
			return err
		}
		if len(result.AuthorizationData) != 1 {
			return fmt.Errorf("expected one registry authorization")
		}
		auth := result.AuthorizationData[0]
		decoded, err := base64.StdEncoding.DecodeString(aws.ToString(auth.AuthorizationToken))
		if err != nil {
			return err
		}
		user, password, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return fmt.Errorf("invalid registry authorization")
		}
		login := exec.CommandContext(ctx, "docker", "login", "--username", user, "--password-stdin", aws.ToString(auth.ProxyEndpoint))
		login.Stdin = strings.NewReader(password)
		login.Stdout = out
		login.Stderr = out
		if err := login.Run(); err != nil {
			return err
		}
		destination := environment.Repository + ":" + *tag
		for _, args := range [][]string{{"tag", *image, destination}, {"push", destination}} {
			cmd := exec.CommandContext(ctx, "docker", args...)
			cmd.Stdout = out
			cmd.Stderr = out
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(out, destination)
		return err
	default:
		return fmt.Errorf("unknown aws command %q", action)
	}
}
