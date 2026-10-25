package command

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdaurl"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/fleet"
	"github.com/fogo-sh/insta-game/internal/lobby"
)

func lambdaCommand(ctx context.Context) error {
	if os.Getenv("INSTA_GAME_FLEET_TABLE") != "" {
		sdk, err := cloud.LoadSDK(ctx, os.Getenv("AWS_REGION"))
		if err != nil {
			return err
		}
		release, err := loadFleetRelease(ctx, sdk)
		if err != nil {
			return err
		}
		limit, err := strconv.Atoi(os.Getenv("INSTA_GAME_FLEET_MAX_HOSTS"))
		if err != nil || limit < 1 || limit > 16 {
			return fmt.Errorf("invalid fleet host limit")
		}
		release.Config.MaxHosts = min(release.Config.MaxHosts, limit)
		release.Config.Release = os.Getenv("INSTA_GAME_FLEET_RELEASE")
		release.Config.LaunchTemplate = os.Getenv("INSTA_GAME_FLEET_TEMPLATE")
		release.Config.LaunchVersion = os.Getenv("INSTA_GAME_FLEET_VERSION")
		if err = release.Config.Validate(); err != nil {
			return err
		}
		hash := os.Getenv("INSTA_GAME_LOBBY_TOKEN_HASH")
		if err = lobby.ValidateHash(hash); err != nil {
			return err
		}
		scheduler := &fleet.Scheduler{Store: &fleet.Table{Client: dynamodb.NewFromConfig(sdk), Name: os.Getenv("INSTA_GAME_FLEET_TABLE")}, Machines: &fleet.EC2{Client: ec2.NewFromConfig(sdk), Stack: os.Getenv("INSTA_GAME_FLEET_STACK")}, Config: release.Config}
		return startLambda(ctx, (&fleet.Page{Scheduler: scheduler, TokenHash: hash}).Handler(), scheduler.Reconcile)
	}
	ids, err := lobby.ParseIDs(os.Getenv("INSTA_GAME_LOBBY_INSTANCES"))
	if err != nil {
		return err
	}
	hash := os.Getenv("INSTA_GAME_LOBBY_TOKEN_HASH")
	if err := lobby.ValidateHash(hash); err != nil {
		return err
	}
	sdk, err := cloud.LoadSDK(ctx, os.Getenv("AWS_REGION"))
	if err != nil {
		return err
	}
	table := &lobby.Table{Client: dynamodb.NewFromConfig(sdk), Name: os.Getenv("INSTA_GAME_LOBBY_TABLE")}
	service := &lobby.Service{IDs: ids, TokenHash: hash, Table: table, Host: &lobby.Host{Client: ec2.NewFromConfig(sdk), InstanceID: os.Getenv("INSTA_GAME_HOST_ID"), StackID: os.Getenv("INSTA_GAME_STACK_ID")}}
	var page http.Handler = service.Handler()
	reconcile := service.Reconcile
	return startLambda(ctx, page, reconcile)
}
func startLambda(ctx context.Context, page http.Handler, reconcile func(context.Context) error) error {
	handler := lambdaurl.Wrap(page)
	lambda.StartWithOptions(func(ctx context.Context, raw json.RawMessage) (any, error) {
		var source struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal(raw, &source); err != nil {
			return nil, err
		}
		if source.Source == "aws.events" {
			return nil, reconcile(ctx)
		}
		var event events.LambdaFunctionURLRequest
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, err
		}
		return handler(ctx, &event)
	}, lambda.WithContext(ctx))
	return nil
}
