package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"golang.org/x/crypto/ssh"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
)

func awsCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("aws requires plan, apply, config, install, lobby, status, wake, stop, or destroy")
	}
	action := args[0]
	flags := flag.NewFlagSet("aws "+action, flag.ContinueOnError)
	flags.SetOutput(out)
	region := flags.String("region", "ca-central-1", "AWS region")
	name := flags.String("stack", "insta-game-host", "Owned shared-host stack")
	gameCIDR := flags.String("game-cidr", "0.0.0.0/0", "Allowed player or gateway public IPv4 CIDR")
	cidr := flags.String("operator-cidr", "", "Operator public IPv4 CIDR for SSH and HTTPS")
	publicKey := flags.String("public-key", "", "SSH public key file")
	instanceType := flags.String("instance-type", "t3a.micro", "Burstable EC2 instance type (t3/t3a AMD64 or t4g ARM64)")
	disk := flags.Int("disk-gib", 8, "Encrypted gp3 root disk size")
	udpFrom := flags.Int("udp-from", 32768, "First public game UDP port")
	udpTo := flags.Int("udp-to", 60999, "Last public game UDP port")
	changeSet := flags.String("change-set", "", "Reviewed change set ARN from aws plan")
	configFile := flags.String("config", "", "Shared host JSON config")
	output := flags.String("out", "", "Write host config here")
	key := flags.String("key", "", "Local SSH private key path (saved in config)")
	token := flags.String("token-file", "", "Local controller token path (created if missing)")
	disableLobby := flags.Bool("disable", false, "Disable this stack’s launch page, retaining its host and lobby data")
	binary := flags.String("binary", "", "Linux insta-game binary matching the host architecture")
	engines := flags.String("engines", "", "Optional directory of native engines and runtime files")
	assets := flags.String("assets", "", "Optional directory of native game assets")
	envFile := flags.String("env-file", "", "Optional private systemd environment file (asset URLs, Discord settings)")
	nativeConfig := flags.String("native-config", "", "Native engine definitions using remote /opt/insta-game paths")
	hostMaxRuntime := flags.Duration("host-max-runtime", 6*time.Hour, "Maximum boot lifetime (0 disables the independent poweroff timer)")
	pullImages := flags.Bool("pull-images", false, "Pull the catalog Docker images during installation")
	lobbyIDs := flags.String("lobby-instances", "", "Saved instance IDs used as AWS launch presets (comma-separated)")
	lobbyToken := flags.String("lobby-token-file", "", "Separate invitation key file for the launch page (created if missing)")
	maxHosts := flags.Int("max-hosts", 0, "Use a catalog-first fleet with this maximum number of cheap hosts")
	fleetConfig := flags.String("fleet-config", "", "Optional fleet resource requirements and runtime limits (JSON)")
	deadline := flags.Duration("expires-after", 0, "Fleet test lifetime, including setup (0 keeps the deployment available)")
	catalog := flags.String("catalog", "", "Game catalog directory to install")
	if err := flags.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	specified := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { specified[f.Name] = true })
	if *maxHosts > 0 && !specified["instance-type"] {
		*instanceType = "t4g.small"
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
	if *configFile == "" && (action == "lobby" || action == "status" || action == "wake" || action == "stop") {
		c, err = cloud.ReadConfig(ctx, sdk, *name)
		if err != nil {
			return err
		}
	}
	if c.Fleet && action == "plan" {
		previous, err := cloud.DescribeStack(ctx, cloudformation.NewFromConfig(sdk), *name)
		if err != nil {
			return err
		}
		for _, p := range previous.Parameters {
			switch aws.ToString(p.ParameterKey) {
			case "InstanceType":
				if !specified["instance-type"] {
					*instanceType = aws.ToString(p.ParameterValue)
				}
			case "DiskGiB":
				if !specified["disk-gib"] {
					*disk, err = strconv.Atoi(aws.ToString(p.ParameterValue))
					if err != nil {
						return err
					}
				}
			case "MaxHosts":
				if !specified["max-hosts"] {
					*maxHosts, err = strconv.Atoi(aws.ToString(p.ParameterValue))
					if err != nil {
						return err
					}
				}
			}
		}
	}
	switch action {
	case "lobby":
		if c.Fleet {
			if *disableLobby {
				return fmt.Errorf("use aws destroy to remove a fleet deployment")
			}
			cfg, err := cloud.FleetDefaults(ctx, sdk, *name)
			if err != nil {
				return err
			}
			if *maxHosts > 0 {
				cfg.MaxHosts = *maxHosts
			}
			if *fleetConfig != "" {
				data, err := os.ReadFile(*fleetConfig)
				if err != nil {
					return err
				}
				decoder := json.NewDecoder(bytes.NewReader(data))
				decoder.DisallowUnknownFields()
				if err = decoder.Decode(&cfg); err != nil {
					return err
				}
				if decoder.Decode(new(any)) != io.EOF {
					return fmt.Errorf("fleet config must contain one JSON object")
				}
			}
			if specified["expires-after"] {
				if *deadline < 0 {
					return fmt.Errorf("expires-after must not be negative")
				}
				cfg.Deadline = 0
				if *deadline > 0 {
					cfg.Deadline = time.Now().Add(*deadline).Unix()
				}
			}
			id, err := cloud.PlanFleetRelease(ctx, sdk, *name, cloud.FleetReleaseOptions{Binary: *binary, Catalog: *catalog, Assets: *assets, Config: cfg})
			if errors.Is(err, cloud.ErrLobbyCurrent) {
				fmt.Fprintln(out, "Fleet release is already current.")
				return nil
			}
			if err != nil {
				return err
			}
			return printChangeSet(ctx, sdk, id, out)
		}
		id, err := cloud.PlanLobby(ctx, sdk, *name, cloud.LobbyOptions{Binary: *binary, Disable: *disableLobby})
		if errors.Is(err, cloud.ErrLobbyCurrent) {
			fmt.Fprintln(out, "Launch page already uses this binary.")
			return nil
		}
		if err != nil {
			return err
		}
		return printChangeSet(ctx, sdk, id, out)
	case "plan":
		if *lobbyIDs == "" {
			*lobbyIDs = c.LobbyInstances
		}
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
		hash := ""
		if *lobbyIDs != "" || *maxHosts > 0 {
			if *lobbyToken == "" {
				return fmt.Errorf("launch page requires --lobby-token-file")
			}
			if err := ensureHostToken(*lobbyToken); err != nil {
				return err
			}
			data, err := os.ReadFile(*lobbyToken)
			if err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(strings.TrimSpace(string(data))))
			hash = hex.EncodeToString(sum[:])
		}
		options := cloud.PlanOptions{MaxHosts: *maxHosts, LobbyInstances: *lobbyIDs, LobbyTokenHash: hash, GameCIDR: *gameCIDR, OperatorCIDR: *cidr, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(parsed))), InstanceType: *instanceType, DiskGiB: *disk, UDPFrom: *udpFrom, UDPTo: *udpTo}
		var id string
		if *maxHosts > 0 {
			if *disk < 24 {
				options.DiskGiB = 32
			}
			id, err = cloud.PlanFleet(ctx, sdk, stackShortName(*name), options)
		} else {
			id, err = cloud.Plan(ctx, sdk, stackShortName(*name), options)
		}
		if err != nil {
			return err
		}
		return printChangeSet(ctx, sdk, id, out)
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
		if c.Fleet {
			fresh, err := cloud.ReadConfig(ctx, sdk, c.Stack)
			if err != nil {
				return err
			}
			if !fresh.Fleet || fresh.LobbyTable != c.LobbyTable {
				return fmt.Errorf("fleet config does not match the owned stack")
			}
			c = fresh
			switch action {
			case "status":
				return fleetStatus(ctx, sdk, c, out)
			case "stop":
				return fleetStop(ctx, sdk, c)
			default:
				return fmt.Errorf("fleet hosts install and wake automatically; create or start a server in the launch page")
			}
		}
		if c.InstanceID == "" {
			return fmt.Errorf("%s requires --config", action)
		}
		host, err := cloud.NewHost(ctx, c)
		if err != nil {
			return err
		}
		switch action {
		case "install":
			return installHost(ctx, host, installOptions{Binary: *binary, Engines: *engines, Assets: *assets, NativeConfig: *nativeConfig, Catalog: *catalog, PullImages: *pullImages, MaxHostRuntime: *hostMaxRuntime, EnvFile: *envFile}, out)
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

func printChangeSet(ctx context.Context, sdk aws.Config, id string, out io.Writer) error {
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
}
