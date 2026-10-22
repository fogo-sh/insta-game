package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"golang.org/x/crypto/ssh"
)

func awsCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("aws requires plan, apply, config, install, status, wake, stop, or destroy")
	}
	action := args[0]
	flags := flag.NewFlagSet("aws "+action, flag.ContinueOnError)
	flags.SetOutput(out)
	region := flags.String("region", "ca-central-1", "AWS region")
	name := flags.String("stack", "insta-game-host", "Owned shared-host stack")
	cidr := flags.String("operator-cidr", "", "Operator public IPv4 CIDR for SSH and HTTPS")
	publicKey := flags.String("public-key", "", "SSH public key file")
	instanceType := flags.String("instance-type", "t3a.micro", "AMD64 burstable EC2 instance type")
	disk := flags.Int("disk-gib", 8, "Encrypted gp3 root disk size")
	udpFrom := flags.Int("udp-from", 32768, "First public game UDP port")
	udpTo := flags.Int("udp-to", 60999, "Last public game UDP port")
	changeSet := flags.String("change-set", "", "Reviewed change set ARN from aws plan")
	configFile := flags.String("config", "", "Shared host JSON config")
	output := flags.String("out", "", "Write host config here")
	key := flags.String("key", "", "Local SSH private key path (saved in config)")
	token := flags.String("token-file", "", "Local controller token path (created if missing)")
	binary := flags.String("binary", "", "Linux AMD64 insta-game binary to install")
	engine := flags.String("engine", "", "Native engine executable to install")
	assets := flags.String("assets", "", "Game assets directory to install")
	nativeConfig := flags.String("native-config", "", "Native engine definitions using remote /opt/insta-game paths")
	catalog := flags.String("catalog", "", "Game catalog directory to install")
	if err := flags.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	var c cloud.Config
	if *configFile != "" {
		loaded, err := cloud.Load(*configFile)
		if err != nil {
			return err
		}
		c = loaded
		conflict := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "region" && *region != c.Region {
				conflict = true
			}
			if f.Name == "stack" && *name != c.Stack && *name != stackShortName(c.Stack) {
				conflict = true
			}
		})
		if conflict {
			return fmt.Errorf("--region or --stack conflicts with host config")
		}
		*region, *name = c.Region, c.Stack
	}
	sdk, err := cloud.LoadSDK(ctx, *region)
	if err != nil {
		return err
	}
	switch action {
	case "plan":
		material, err := os.ReadFile(*publicKey)
		if err != nil {
			return fmt.Errorf("read --public-key: %w", err)
		}
		parsed, _, _, _, err := ssh.ParseAuthorizedKey(material)
		if err != nil {
			return fmt.Errorf("parse SSH public key: %w", err)
		}
		if parsed.Type() != ssh.KeyAlgoED25519 && parsed.Type() != ssh.KeyAlgoRSA {
			return fmt.Errorf("EC2 requires an ED25519 or RSA SSH key")
		}
		id, err := cloud.Plan(ctx, sdk, stackShortName(*name), cloud.PlanOptions{OperatorCIDR: *cidr, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(parsed))), InstanceType: *instanceType, DiskGiB: *disk, UDPFrom: *udpFrom, UDPTo: *udpTo})
		if err != nil {
			return err
		}
		changes, err := cloudformation.NewFromConfig(sdk).DescribeChangeSet(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: aws.String(id)})
		if err != nil {
			return err
		}
		fmt.Fprintln(out, id)
		for _, change := range changes.Changes {
			if r := change.ResourceChange; r != nil {
				fmt.Fprintf(out, "%s %s (%s), replacement=%s\n", r.Action, aws.ToString(r.LogicalResourceId), aws.ToString(r.ResourceType), r.Replacement)
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
		fresh, err := cloud.ReadConfig(ctx, sdk, *name)
		if err != nil {
			return err
		}
		fresh.KeyPath, fresh.TokenPath = c.KeyPath, c.TokenPath
		if *key != "" {
			fresh.KeyPath, err = filepath.Abs(*key)
			if err != nil {
				return err
			}
		}
		if *token != "" {
			fresh.TokenPath, err = filepath.Abs(*token)
			if err != nil {
				return err
			}
			if err := ensureHostToken(fresh.TokenPath); err != nil {
				return err
			}
		}
		data, err := json.MarshalIndent(fresh, "", "  ")
		if err != nil {
			return err
		}
		if *output != "" {
			return os.WriteFile(*output, append(data, '\n'), 0600)
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	case "destroy":
		return cloud.Destroy(ctx, sdk, *name)
	case "install", "status", "wake", "stop":
		if c.InstanceID == "" {
			return fmt.Errorf("%s requires --config", action)
		}
		host, err := cloud.NewHost(ctx, c)
		if err != nil {
			return err
		}
		switch action {
		case "install":
			return installHost(ctx, host, installOptions{Binary: *binary, Engine: *engine, Assets: *assets, NativeConfig: *nativeConfig, Catalog: *catalog}, out)
		case "wake":
			ip, err := host.Wake(ctx)
			if err == nil {
				fmt.Fprintln(out, ip)
			}
			return err
		case "stop":
			return host.Stop(ctx)
		default:
			state, err := host.Inspect(ctx)
			if err != nil {
				return err
			}
			return json.NewEncoder(out).Encode(state)
		}
	default:
		return fmt.Errorf("unknown aws command %q", action)
	}
}

func stackShortName(name string) string {
	parts := strings.Split(name, "/")
	if strings.HasPrefix(name, "arn:") && len(parts) == 3 {
		return parts[1]
	}
	return name
}

// A deployed host needs an actual token file, independent of the local CLI session token.
func ensureHostToken(path string) error {
	data, err := os.ReadFile(path)
	if err == nil {
		if strings.TrimSpace(string(data)) == "" {
			return fmt.Errorf("empty host token file %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(file, control.NewToken()+"\n")
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
