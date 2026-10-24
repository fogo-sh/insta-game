// Package lobby connects an always-accessible launch page to a sleeping controller.
package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fogo-sh/insta-game/internal/control"
)

const RequestLifetime = 10 * time.Minute

var ErrBusy = errors.New("a start is already pending, or was requested less than 30 seconds ago")
var instanceID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func ParseIDs(value string) ([]string, error) {
	ids := strings.Split(value, ",")
	seen := map[string]bool{}
	if len(ids) > 16 {
		return nil, fmt.Errorf("lobby supports at most 16 saved instances")
	}
	for _, id := range ids {
		if !instanceID.MatchString(id) || seen[id] {
			return nil, fmt.Errorf("lobby requires unique saved instance IDs, separated by commas")
		}
		seen[id] = true
	}
	return ids, nil
}

type Snapshot struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Game       string    `json:"game"`
	Phase      string    `json:"phase"`
	Desired    string    `json:"desired"`
	Address    string    `json:"address,omitempty"`
	Players    *int      `json:"players,omitempty"`
	Error      string    `json:"error,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

type Row struct {
	Snapshot                                    Snapshot
	RequestID, AcceptedID, CompletedID, Failure string
	ExpiresAt, RequestedAt                      int64
}

func (r Row) Pending(now time.Time) bool {
	return r.RequestID != "" && r.RequestID != r.CompletedID && r.ExpiresAt > now.Unix()
}

// Only snapshots are public. Request fields coordinate retries without copying the controller database.
type Storage interface {
	Read(context.Context, string) (Row, error)
	Request(context.Context, string, time.Time) error
	Publish(context.Context, Snapshot) error
	Mark(context.Context, string, string, string, string) error
}

type Table struct {
	Client *dynamodb.Client
	Name   string
}

func key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"id": textValue(id)}
}
func textValue(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func numberValue(v int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(v, 10)}
}
func textAttr(m map[string]types.AttributeValue, name string) string {
	if v, ok := m[name].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
func numberAttr(m map[string]types.AttributeValue, name string) int64 {
	if v, ok := m[name].(*types.AttributeValueMemberN); ok {
		n, _ := strconv.ParseInt(v.Value, 10, 64)
		return n
	}
	return 0
}
func (t *Table) Read(ctx context.Context, id string) (Row, error) {
	result, err := t.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(t.Name), Key: key(id), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return Row{}, err
	}
	r := Row{RequestID: textAttr(result.Item, "requestID"), AcceptedID: textAttr(result.Item, "acceptedID"), CompletedID: textAttr(result.Item, "completedID"), Failure: textAttr(result.Item, "failure"), ExpiresAt: numberAttr(result.Item, "expiresAt"), RequestedAt: numberAttr(result.Item, "requestedAt")}
	if s := textAttr(result.Item, "snapshot"); s != "" {
		if err := json.Unmarshal([]byte(s), &r.Snapshot); err != nil {
			return Row{}, fmt.Errorf("decode lobby snapshot: %w", err)
		}
	}
	return r, nil
}
func (t *Table) Request(ctx context.Context, id string, now time.Time) error {
	_, err := t.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(t.Name), Key: key(id),
		UpdateExpression:          aws.String("SET requestID = :request, expiresAt = :expires, requestedAt = :now REMOVE failure"),
		ConditionExpression:       aws.String("attribute_exists(#snapshot) AND (attribute_not_exists(expiresAt) OR expiresAt <= :now OR requestID = completedID) AND (attribute_not_exists(requestedAt) OR requestedAt <= :cooldown)"),
		ExpressionAttributeNames:  map[string]string{"#snapshot": "snapshot"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":request": textValue(control.NewToken()), ":expires": numberValue(now.Add(RequestLifetime).Unix()), ":now": numberValue(now.Unix()), ":cooldown": numberValue(now.Add(-30 * time.Second).Unix())},
	})
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return ErrBusy
	}
	return err
}
func (t *Table) Publish(ctx context.Context, snapshot Snapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = t.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: aws.String(t.Name), Key: key(snapshot.ID), UpdateExpression: aws.String("SET #snapshot = :snapshot"), ExpressionAttributeNames: map[string]string{"#snapshot": "snapshot"}, ExpressionAttributeValues: map[string]types.AttributeValue{":snapshot": textValue(string(data))}})
	return err
}
func (t *Table) Mark(ctx context.Context, id, request, stage, failure string) error {
	if stage != "acceptedID" && stage != "completedID" {
		return fmt.Errorf("invalid request stage")
	}
	values := map[string]types.AttributeValue{":request": textValue(request)}
	expression := "SET " + stage + " = :request"
	if stage == "completedID" {
		expression += ", failure = :failure"
		values[":failure"] = textValue(failure)
	}
	_, err := t.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{TableName: aws.String(t.Name), Key: key(id), UpdateExpression: aws.String(expression), ConditionExpression: aws.String("requestID = :request"), ExpressionAttributeValues: values})
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return nil
	}
	return err
}
