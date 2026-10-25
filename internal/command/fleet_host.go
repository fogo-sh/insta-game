package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/fleet"
	"github.com/fogo-sh/insta-game/internal/gamefiles"
)

func loadFleetRelease(ctx context.Context, sdk aws.Config) (cloud.FleetRelease, error) {
	out, err := s3.NewFromConfig(sdk).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(os.Getenv("INSTA_GAME_FLEET_BUCKET")), Key: aws.String(os.Getenv("INSTA_GAME_FLEET_RELEASE") + "/config.json")})
	if err != nil {
		return cloud.FleetRelease{}, err
	}
	defer out.Body.Close()
	var release cloud.FleetRelease
	if err = json.NewDecoder(io.LimitReader(out.Body, 1024*1024)).Decode(&release); err != nil {
		return release, err
	}
	return release, nil
}
func fleetHost(ctx context.Context) error {
	sdk, err := cloud.LoadSDK(ctx, os.Getenv("AWS_REGION"))
	if err != nil {
		return err
	}
	release, err := loadFleetRelease(ctx, sdk)
	if err != nil {
		return err
	}
	metadata := imds.NewFromConfig(sdk)
	identity, err := metadata.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return err
	}
	table := &fleet.Table{Client: dynamodb.NewFromConfig(sdk), Name: os.Getenv("INSTA_GAME_FLEET_TABLE")}
	state, err := table.Load(ctx)
	if err != nil {
		return err
	}
	var host fleet.Host

	compute := ec2.NewFromConfig(sdk)
	own, err := compute.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{identity.InstanceID}})
	if err != nil {
		return err
	}
	if len(own.Reservations) != 1 || len(own.Reservations[0].Instances) != 1 {
		return fmt.Errorf("expected own EC2 identity")
	}
	instance := own.Reservations[0].Instances[0]
	tags := map[string]string{}
	for _, t := range instance.Tags {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	if tags["insta-game:fleet"] != os.Getenv("INSTA_GAME_FLEET_STACK") {
		return fmt.Errorf("machine belongs to another fleet")
	}
	for _, candidate := range state.Hosts {
		if candidate.ID == tags["insta-game:host"] && candidate.LaunchToken == aws.ToString(instance.ClientToken) && (candidate.InstanceID == "" || candidate.InstanceID == identity.InstanceID) {
			host = candidate
			break
		}
	}
	if host.ID == "" || host.Retiring || state.Deleting {
		return fmt.Errorf("machine is not an active fleet member")
	}
	address, err := metadata.GetMetadata(ctx, &imds.GetMetadataInput{Path: "public-ipv4"})
	if err != nil {
		return err
	}
	ip, err := io.ReadAll(address.Content)
	address.Content.Close()
	if err != nil {
		return err
	}
	root := "/var/lib/insta-game"
	store, err := control.OpenStore(filepath.Join(root, "instances.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	docker := control.NewDocker("/var/run/docker.sock", strings.TrimSpace(string(ip)), "0.0.0.0")
	defer docker.Close()
	docker.Binary = "/usr/local/bin/insta-game"
	docker.AssetVersion = os.Getenv("INSTA_GAME_FLEET_RELEASE")
	docker.PrepareAssets = func(ctx context.Context, g control.Game) (string, error) {
		key := release.Assets[g.ID]
		if key == "" {
			return "", nil
		}
		destination := filepath.Join(root, "assets", strings.TrimSuffix(filepath.Base(key), ".zip"))
		if _, err := os.Stat(filepath.Join(destination, ".ready")); err == nil {
			return destination, nil
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return "", err
		}
		temporary, err := os.MkdirTemp(filepath.Dir(destination), ".download-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(temporary)
		archive, err := os.CreateTemp(root, ".asset-*.zip")
		if err != nil {
			return "", err
		}
		defer os.Remove(archive.Name())
		defer archive.Close()
		object, err := s3.NewFromConfig(sdk).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(os.Getenv("INSTA_GAME_FLEET_BUCKET")), Key: aws.String(key)})
		if err != nil {
			return "", err
		}
		_, err = io.Copy(archive, object.Body)
		object.Body.Close()
		if err != nil {
			return "", err
		}
		if err = gamefiles.ExtractZip(ctx, archive.Name(), temporary); err != nil {
			return "", err
		}
		if err = os.WriteFile(filepath.Join(temporary, ".ready"), nil, 0644); err != nil {
			return "", err
		}
		if err = os.Chmod(filepath.Join(temporary, ".ready"), 0644); err != nil {
			return "", err
		}
		if err = os.Chmod(temporary, 0755); err != nil {
			return "", err
		}
		if err = os.Rename(temporary, destination); err != nil {
			return "", err
		}
		return destination, nil
	}
	controller := control.NewController(store, release.Catalog, map[string]control.Provider{"docker": docker}, "docker")
	run, cancel := context.WithCancel(ctx)
	defer cancel()
	worker := &fleet.Worker{Store: table, Host: host.ID, Config: release.Config, Controller: controller, StartedAt: time.Now().UTC()}
	var workers sync.WaitGroup
	workers.Go(func() { controller.Run(run) })
	workers.Go(func() { worker.Run(run) })
	defer func() { cancel(); workers.Wait() }()

	idle := make(chan error, 1)
	workers.Go(func() {
		idle <- poweroffWhenIdle(run, 2*time.Minute, store.List, func(ctx context.Context) (bool, error) {
			pending, err := worker.Pending(ctx)
			if err != nil || pending {
				return false, err
			}
			ok, err := controller.BeginIdleShutdown()
			if !ok || err != nil {
				return false, err
			}
			return true, exec.CommandContext(ctx, "systemctl", "poweroff").Run()
		})
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-idle:
		return err
	}
}
