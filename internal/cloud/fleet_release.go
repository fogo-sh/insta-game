package cloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/fleet"
)

type FleetRelease struct {
	Config  fleet.Config      `json:"config"`
	Catalog []control.Game    `json:"catalog"`
	Assets  map[string]string `json:"assets"`
}
type FleetReleaseOptions struct {
	Binary, Catalog, Assets string
	Config                  fleet.Config
}

func DefaultFleetConfig(maxHosts, diskGiB int) fleet.Config {
	return fleet.Config{MaxHosts: maxHosts, MaxServers: 128, Capacity: fleet.Resources{CPU: 350, MemoryMiB: 1400, DiskMiB: (diskGiB - 12) * 1024}, IdleMinutes: 15, MaxMinutes: 360}
}

var defaultRequirements = map[string]fleet.Resources{
	"fteqw":     {CPU: 60, MemoryMiB: 96, DiskMiB: 256},
	"q2repro":   {CPU: 40, MemoryMiB: 160, DiskMiB: 512},
	"ioquake3":  {CPU: 140, MemoryMiB: 192, DiskMiB: 1024},
	"openarena": {CPU: 150, MemoryMiB: 256, DiskMiB: 1024},
	"ut99":      {CPU: 320, MemoryMiB: 256, DiskMiB: 1024},
	"xonotic":   {CPU: 150, MemoryMiB: 576, DiskMiB: 2048},
	"bzflag":    {CPU: 20, MemoryMiB: 128, DiskMiB: 128},
}

func PlanFleetRelease(ctx context.Context, sdk aws.Config, name string, options FleetReleaseOptions) (string, error) {
	stacks := cloudformation.NewFromConfig(sdk)
	stack, err := DescribeStack(ctx, stacks, name)
	if err != nil {
		return "", err
	}
	if Outputs(stack)["Fleet"] != "true" {
		return "", fmt.Errorf("not a fleet stack")
	}
	limits, err := fleetDefaults(stack)
	if err != nil {
		return "", err
	}
	if options.Config.MaxHosts > limits.MaxHosts || options.Config.Capacity.CPU > limits.Capacity.CPU || options.Config.Capacity.MemoryMiB > limits.Capacity.MemoryMiB || options.Config.Capacity.DiskMiB > limits.Capacity.DiskMiB {
		return "", fmt.Errorf("fleet configuration exceeds the reviewed stack's host or capacity limits")
	}
	architecture := preparedArchitecture(options.Binary)
	if architecture == "" {
		return "", fmt.Errorf("fleet requires a static Linux ARM64 or AMD64 binary")
	}
	for _, p := range stack.Parameters {
		if aws.ToString(p.ParameterKey) == "InstanceType" && (strings.HasPrefix(aws.ToString(p.ParameterValue), "t4g.") != (architecture == "arm64")) {
			return "", fmt.Errorf("binary architecture differs from deployed host")
		}
	}
	previous, err := deployedFleetRelease(ctx, sdk, stack)
	if err != nil {
		return "", err
	}
	prepared, err := prepareFleetRelease(options, previous)
	if err != nil {
		return "", err
	}
	if options.Catalog != "" {
		for i := range prepared.Catalog {
			image, err := pinImage(ctx, prepared.Catalog[i].Image, architecture)
			if err != nil {
				return "", err
			}
			prepared.Catalog[i].Image = image
		}
	}
	bucket := Outputs(stack)["FleetBucket"]
	client := s3.NewFromConfig(sdk)
	// Immutable keys let a host created before an update finish booting with its original release.
	binary, err := os.Open(options.Binary)
	if err != nil {
		return "", err
	}
	defer binary.Close()
	sum := sha256.New()
	if _, err = io.Copy(sum, binary); err != nil {
		return "", err
	}
	if options.Assets != "" {
		entries, err := os.ReadDir(options.Assets)
		if err != nil {
			return "", err
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				return "", fmt.Errorf("assets must contain regular game-ID.zip archives")
			}
			id := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
			if filepath.Ext(entry.Name()) != ".zip" {
				return "", fmt.Errorf("asset %s must be a ZIP rooted at the game's data directory", entry.Name())
			}
			known := false
			for _, g := range prepared.Catalog {
				if g.ID == id {
					known = true
				}
			}
			if !known {
				return "", fmt.Errorf("unknown asset game %s", id)
			}
			f, err := os.Open(filepath.Join(options.Assets, entry.Name()))
			if err != nil {
				return "", err
			}
			digest := sha256.New()
			_, err = io.Copy(digest, f)
			if err != nil {
				f.Close()
				return "", err
			}
			key := "assets/" + hex.EncodeToString(digest.Sum(nil)) + ".zip"
			_, err = f.Seek(0, 0)
			if err == nil {
				_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: f})
			}
			f.Close()
			if err != nil {
				return "", err
			}
			prepared.Assets[id] = key
		}
	}
	prepared.Config.Workload = workloadFingerprint(prepared)
	// Include asset identities in release ID, so changed assets never mutate a pinned release.
	manifest, err := json.Marshal(prepared)
	if err != nil {
		return "", err
	}
	sum.Write(manifest)
	release := "releases/" + hex.EncodeToString(sum.Sum(nil))
	current := map[string]string{}
	for _, p := range stack.Parameters {
		current[aws.ToString(p.ParameterKey)] = aws.ToString(p.ParameterValue)
	}
	if current["Release"] == release && current["LobbyCodeKey"] == release+"/bootstrap.zip" && current["LobbyArchitecture"] == architecture {
		return "", ErrLobbyCurrent
	}
	if _, err = binary.Seek(0, 0); err != nil {
		return "", err
	}
	if _, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(release + "/insta-game"), Body: binary}); err != nil {
		return "", err
	}
	if _, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(release + "/config.json"), Body: bytes.NewReader(manifest)}); err != nil {
		return "", err
	}
	packed, err := lambdaArchive(binary)
	if err != nil {
		return "", err
	}
	code := release + "/bootstrap.zip"
	if _, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(code), Body: bytes.NewReader(packed)}); err != nil {
		return "", err
	}
	params, err := updateParameters(FleetTemplate, stack.Parameters, map[string]string{"Release": release, "LobbyCodeKey": code, "LobbyArchitecture": preparedArchitecture(options.Binary)})
	if err != nil {
		return "", err
	}
	out, err := stacks.CreateChangeSet(ctx, &cloudformation.CreateChangeSetInput{StackName: stack.StackId, ChangeSetName: aws.String(fmt.Sprintf("insta-game-release-%d", time.Now().UnixNano())), ChangeSetType: cf.ChangeSetTypeUpdate, TemplateBody: aws.String(FleetTemplate), Parameters: params, Capabilities: []cf.Capability{cf.CapabilityCapabilityIam}, Tags: stack.Tags})
	if err != nil {
		return "", err
	}
	if err = cloudformation.NewChangeSetCreateCompleteWaiter(stacks).Wait(ctx, &cloudformation.DescribeChangeSetInput{ChangeSetName: out.Id}, 5*time.Minute); err != nil {
		return "", err
	}
	return aws.ToString(out.Id), nil
}

func prepareFleetRelease(options FleetReleaseOptions, previous *FleetRelease) (FleetRelease, error) {
	var games []control.Game
	if options.Catalog != "" {
		var err error
		games, err = control.LoadCatalog(options.Catalog)
		if err != nil {
			return FleetRelease{}, err
		}
	} else if previous != nil {
		games = append([]control.Game(nil), previous.Catalog...)
	} else {
		return FleetRelease{}, fmt.Errorf("initial fleet release requires --catalog")
	}
	for i := range games {
		ports := make(map[string]json.RawMessage, len(games[i].Ports))
		for key, value := range games[i].Ports {
			ports[key] = append(json.RawMessage(nil), value...)
		}
		games[i].Ports = ports
		games[i].ConfigEditor = append(json.RawMessage(nil), games[i].ConfigEditor...)
	}
	assets := map[string]string{}
	if previous != nil {
		for _, game := range games {
			if key := previous.Assets[game.ID]; key != "" {
				assets[game.ID] = key
			}
		}
	}
	for _, id := range []string{"fteqw", "q2repro", "ioquake3", "ut99"} {
		for _, g := range games {
			if g.ID == id {
				if assets[id] != "" {
					continue
				}
				if options.Assets == "" {
					return FleetRelease{}, fmt.Errorf("game %s needs a durable asset ZIP", id)
				}
				if info, err := os.Stat(filepath.Join(options.Assets, id+".zip")); err != nil || !info.Mode().IsRegular() {
					return FleetRelease{}, fmt.Errorf("game %s requires %s.zip in --assets", id, id)
				}
			}
		}
	}
	configured := map[string]fleet.Game{}
	known := map[string]bool{}
	for _, g := range games {
		known[g.ID] = true
	}
	for _, g := range options.Config.Games {
		if !known[g.ID] {
			return FleetRelease{}, fmt.Errorf("resource profile references unknown game %s", g.ID)
		}
		if _, duplicate := configured[g.ID]; duplicate {
			return FleetRelease{}, fmt.Errorf("duplicate resource profile %s", g.ID)
		}
		configured[g.ID] = g
	}
	options.Config.Games = nil
	for _, g := range games {
		entry, ok := configured[g.ID]
		if !ok {
			r, exists := defaultRequirements[g.ID]
			if !exists {
				return FleetRelease{}, fmt.Errorf("game %s needs resource requirements", g.ID)
			}
			entry = fleet.Game{ID: g.ID, Name: g.DisplayName, Resources: r}
		}
		options.Config.Games = append(options.Config.Games, entry)
	}
	options.Config.LaunchTemplate = "set-by-deployment"
	options.Config.LaunchVersion = "1"
	options.Config.Release = ""
	options.Config.Workload = ""
	if err := options.Config.Validate(); err != nil {
		return FleetRelease{}, err
	}
	return FleetRelease{Config: options.Config, Catalog: games, Assets: assets}, nil
}

func FleetDefaults(ctx context.Context, sdk aws.Config, name string) (fleet.Config, error) {
	stack, err := DescribeStack(ctx, cloudformation.NewFromConfig(sdk), name)
	if err != nil {
		return fleet.Config{}, err
	}
	previous, err := deployedFleetRelease(ctx, sdk, stack)
	if err != nil {
		return fleet.Config{}, err
	}
	return fleetUpdateDefaults(stack, previous)
}

func fleetUpdateDefaults(stack *cf.Stack, previous *FleetRelease) (fleet.Config, error) {
	limits, err := fleetDefaults(stack)
	if err != nil || previous == nil {
		return limits, err
	}
	config := previous.Config
	config.Games = append([]fleet.Game(nil), previous.Config.Games...)
	config.MaxHosts = min(config.MaxHosts, limits.MaxHosts)
	if config.Capacity.CPU > limits.Capacity.CPU || config.Capacity.MemoryMiB > limits.Capacity.MemoryMiB || config.Capacity.DiskMiB > limits.Capacity.DiskMiB {
		return fleet.Config{}, fmt.Errorf("deployed fleet capacity exceeds current hardware; review its configuration")
	}
	if err := config.Validate(); err != nil {
		return fleet.Config{}, fmt.Errorf("deployed fleet configuration: %w", err)
	}
	return config, nil
}

func deployedFleetRelease(ctx context.Context, sdk aws.Config, stack *cf.Stack) (*FleetRelease, error) {
	if Outputs(stack)["Fleet"] != "true" {
		return nil, fmt.Errorf("not a fleet stack")
	}
	release := ""
	for _, parameter := range stack.Parameters {
		if aws.ToString(parameter.ParameterKey) == "Release" {
			release = aws.ToString(parameter.ParameterValue)
		}
	}
	if release == "" {
		return nil, nil
	}
	object, err := s3.NewFromConfig(sdk).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(Outputs(stack)["FleetBucket"]), Key: aws.String(release + "/config.json")})
	if err != nil {
		return nil, fmt.Errorf("read deployed fleet release: %w", err)
	}
	defer object.Body.Close()
	data, err := io.ReadAll(io.LimitReader(object.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("deployed fleet release exceeds 1 MiB")
	}
	var result FleetRelease
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode deployed fleet release: %w", err)
	}
	return &result, nil
}
func fleetDefaults(stack *cf.Stack) (fleet.Config, error) {
	values := map[string]string{}
	for _, p := range stack.Parameters {
		values[aws.ToString(p.ParameterKey)] = aws.ToString(p.ParameterValue)
	}
	disk, err := strconv.Atoi(values["DiskGiB"])
	if err != nil {
		return fleet.Config{}, err
	}
	hosts, err := strconv.Atoi(values["MaxHosts"])
	if err != nil {
		return fleet.Config{}, err
	}
	cfg := DefaultFleetConfig(hosts, disk)
	_, size, _ := strings.Cut(values["InstanceType"], ".")
	switch size {
	case "micro":
		cfg.Capacity.MemoryMiB = 640
		cfg.Capacity.CPU = 150
	case "small":
	case "medium":
		cfg.Capacity.MemoryMiB = 3200
		cfg.Capacity.CPU = 350
	default:
		return cfg, fmt.Errorf("fleet supports micro, small, or medium hosts; larger tiers need a reviewed profile")
	}
	return cfg, nil
}

// Bump when host behavior requires upgrading the controller or agent.
const hostProtocolVersion = 3

func workloadFingerprint(release FleetRelease) string {
	catalog := append([]control.Game(nil), release.Catalog...)
	for i := range catalog {
		catalog[i].DisplayName = ""
		catalog[i].ConfigEditor = nil
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].ID < catalog[j].ID })
	payload := struct {
		Protocol                int
		Catalog                 []control.Game
		Assets                  map[string]string
		IdleMinutes, MaxMinutes int
	}{hostProtocolVersion, catalog, release.Assets, release.Config.IdleMinutes, release.Config.MaxMinutes}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
