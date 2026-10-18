// The legacy image entrypoint preserves AWS idle shutdown while sharing the portable agent.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/fogo-sh/insta-game/internal/agent"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	c := agent.FromEnv()
	if service := os.Getenv("ECS_SERVICE"); service != "" {
		c.OnIdle = func(ctx context.Context) error {
			cfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				return err
			}
			cluster := os.Getenv("ECS_CLUSTER")
			zero := int32(0)
			_, err = ecs.NewFromConfig(cfg).UpdateService(ctx, &ecs.UpdateServiceInput{Cluster: &cluster, Service: &service, DesiredCount: &zero})
			return err
		}
	}
	if err := agent.Run(ctx, c); err != nil {
		log.Fatal(err)
	}
}
