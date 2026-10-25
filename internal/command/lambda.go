package command

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdaurl"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/lobby"
)

func lambdaCommand(ctx context.Context) error {
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
	service := &lobby.Service{IDs: ids, TokenHash: hash, Table: &lobby.Table{Client: dynamodb.NewFromConfig(sdk), Name: os.Getenv("INSTA_GAME_LOBBY_TABLE")}, Host: &lobby.Host{Client: ec2.NewFromConfig(sdk), InstanceID: os.Getenv("INSTA_GAME_HOST_ID"), StackID: os.Getenv("INSTA_GAME_STACK_ID")}}
	var page http.Handler = service.Handler()
	reconcile := service.Reconcile
	if value := os.Getenv("INSTA_GAME_LOBBY_HOSTS"); value != "" {
		bindings, err := lobby.ParseBindings(value)
		if err != nil {
			return fmt.Errorf("lobby hosts: %w", err)
		}
		fleet := &lobby.Fleet{TokenHash: hash}
		for _, b := range bindings {
			fleet.Members = append(fleet.Members, lobby.Member{ID: b.ID, Name: b.Name, Service: &lobby.Service{IDs: b.IDs, TokenHash: hash, Table: &lobby.Table{Client: dynamodb.NewFromConfig(sdk), Name: b.Table}, Host: &lobby.Host{Client: ec2.NewFromConfig(sdk), InstanceID: b.InstanceID, StackID: b.StackID}}})
		}
		page, reconcile = fleet.Handler(), fleet.Reconcile
	}
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
