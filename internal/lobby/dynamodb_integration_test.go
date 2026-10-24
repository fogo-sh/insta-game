package lobby

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Uses an isolated deployed table; never runs as part of the ordinary test suite.
func TestDynamoDBRequestConcurrency(t *testing.T) {
	name := os.Getenv("INSTA_GAME_LOBBY_TEST_TABLE")
	if name == "" {
		t.Skip("set INSTA_GAME_LOBBY_TEST_TABLE to an isolated insta-game-lambda-check table")
	}
	if !strings.HasPrefix(name, "insta-game-lambda-check-") {
		t.Fatal("refusing a table outside the isolated check stack")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sdk, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	table := &Table{Client: dynamodb.NewFromConfig(sdk), Name: name}
	id := "test-request-concurrency"
	defer func() {
		_, err := table.Client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{TableName: aws.String(name), Key: key(id)})
		if err != nil {
			t.Error(err)
		}
	}()
	now := time.Now().UTC()
	if err := table.Publish(ctx, Snapshot{ID: id, Name: "Check", Phase: "stopped", ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() {
			err := table.Request(ctx, id, now)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("accepted %d concurrent requests", successes.Load())
	}
	first, err := table.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := table.Publish(ctx, Snapshot{ID: id, Name: "Updated", Phase: "starting", ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	row, err := table.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.RequestID != first.RequestID || !row.Pending(now) || row.Snapshot.Name != "Updated" {
		t.Fatal("status publication lost pending intent")
	}
	if err := table.Mark(ctx, id, first.RequestID, "completedID", ""); err != nil {
		t.Fatal(err)
	}
	if err := table.Request(ctx, id, now); !errors.Is(err, ErrBusy) {
		t.Fatalf("cooldown bypass: %v", err)
	}
	if err := table.Request(ctx, id, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := table.Mark(ctx, id, first.RequestID, "completedID", "stale completion"); err != nil {
		t.Fatal(err)
	}
	row, err = table.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.RequestID == first.RequestID || !row.Pending(now.Add(time.Minute)) || row.Failure != "" {
		t.Fatal("old completion overwrote a newer request")
	}
}
