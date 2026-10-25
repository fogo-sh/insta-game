package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/fleet"
)

func fleetStatus(ctx context.Context, sdk aws.Config, c cloud.Config, out io.Writer) error {
	table := &fleet.Table{Client: dynamodb.NewFromConfig(sdk), Name: c.LobbyTable}
	state, err := table.Load(ctx)
	if err != nil {
		return err
	}
	machines := &fleet.EC2{Client: ec2.NewFromConfig(sdk), Stack: c.Stack}
	hosts := []map[string]any{}
	for _, h := range state.Hosts {
		m, err := machines.Inspect(ctx, h)
		if err != nil {
			return err
		}
		hosts = append(hosts, map[string]any{"host": h.ID, "instance": m.ID, "state": m.State, "capacity": h.Capacity, "error": h.Error})
	}
	servers := []map[string]string{}
	for _, s := range state.Servers {
		servers = append(servers, map[string]string{"id": s.ID, "game": s.Game, "name": s.Name, "host": s.Host, "desired": s.Desired})
	}
	return json.NewEncoder(out).Encode(map[string]any{"url": c.LobbyURL, "hosts": hosts, "servers": servers, "deleting": state.Deleting})
}
func fleetStop(ctx context.Context, sdk aws.Config, c cloud.Config) error {
	table := &fleet.Table{Client: dynamodb.NewFromConfig(sdk), Name: c.LobbyTable}
	state, err := table.Load(ctx)
	if err != nil {
		return err
	}
	scheduler := &fleet.Scheduler{Store: table, Machines: &fleet.EC2{Client: ec2.NewFromConfig(sdk), Stack: c.Stack}}
	for _, s := range state.Servers {
		if s.Desired == "deleted" {
			continue
		}
		if err = scheduler.Action(ctx, s.ID, "stop"); err != nil {
			return fmt.Errorf("stop %s: %w", s.ID, err)
		}
	}
	return scheduler.Reconcile(ctx)
}
