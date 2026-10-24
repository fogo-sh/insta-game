// Package lobby connects an always-accessible launch page to a sleeping controller.
package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/fogo-sh/insta-game/internal/control"
)

const RequestLifetime = 10 * time.Minute

var ErrBusy = errors.New("another request is pending, or this action was requested less than 30 seconds ago")
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
	TemplateID, Action                          string
	ExpiresAt, RequestedAt                      int64
}

func (r Row) Pending(now time.Time) bool {
	return r.RequestID != "" && r.RequestID != r.CompletedID && r.ExpiresAt > now.Unix()
}

// Only snapshots are public. Request fields coordinate retries without copying the controller database.
type Storage interface {
	Read(context.Context, string) (Row, error)
	List(context.Context) ([]Row, error)
	Create(context.Context, string, Snapshot, time.Time) error
	Queue(context.Context, string, string, time.Time) error
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
	return decodeRow(result.Item)
}
func decodeRow(item map[string]types.AttributeValue) (Row, error) {
	r := Row{TemplateID: textAttr(item, "templateID"), Action: textAttr(item, "action"), RequestID: textAttr(item, "requestID"), AcceptedID: textAttr(item, "acceptedID"), CompletedID: textAttr(item, "completedID"), Failure: textAttr(item, "failure"), ExpiresAt: numberAttr(item, "expiresAt"), RequestedAt: numberAttr(item, "requestedAt")}
	if data := textAttr(item, "snapshot"); data != "" {
		if err := json.Unmarshal([]byte(data), &r.Snapshot); err != nil {
			return Row{}, fmt.Errorf("decode lobby snapshot: %w", err)
		}
	}
	return r, nil
}
func (t *Table) Request(ctx context.Context, id string, now time.Time) error {
	return t.Queue(ctx, id, "start", now)
}
func (t *Table) Queue(ctx context.Context, id, action string, now time.Time) error {
	_, err := t.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(t.Name), Key: key(id),
		UpdateExpression:          aws.String("SET #action = :action, requestID = :request, expiresAt = :expires, requestedAt = :now REMOVE failure"),
		ConditionExpression:       aws.String("attribute_exists(#snapshot) AND (attribute_not_exists(expiresAt) OR expiresAt <= :now OR requestID = completedID) AND (attribute_not_exists(requestedAt) OR requestedAt <= :cooldown OR #action <> :action)"),
		ExpressionAttributeNames:  map[string]string{"#snapshot": "snapshot", "#action": "action"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":action": textValue(action), ":request": textValue(control.NewToken()), ":expires": numberValue(now.Add(RequestLifetime).Unix()), ":now": numberValue(now.Unix()), ":cooldown": numberValue(now.Add(-30 * time.Second).Unix())},
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

// List reads the dedicated lobby table, including every page of saved status.
func (t *Table) List(ctx context.Context) ([]Row, error) {
	rows := []Row{}
	pager := dynamodb.NewScanPaginator(t.Client, &dynamodb.ScanInput{TableName: aws.String(t.Name), ConsistentRead: aws.Bool(true)})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			row, err := decodeRow(item)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Snapshot.ID < rows[j].Snapshot.ID })
	return rows, nil
}
func (t *Table) Create(ctx context.Context, template string, snapshot Snapshot, now time.Time) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = t.Client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(t.Name), ConditionExpression: aws.String("attribute_not_exists(id)"), Item: map[string]types.AttributeValue{
		"id": textValue(snapshot.ID), "templateID": textValue(template), "snapshot": textValue(string(data)), "action": textValue("start"), "requestID": textValue(snapshot.ID), "requestedAt": numberValue(now.Unix()), "expiresAt": numberValue(now.Add(RequestLifetime).Unix()),
	}})
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return ErrBusy
	}
	return err
}
func managedIDs(rows []Row, templates []string) []string {
	ids := append([]string{}, templates...)
	for _, row := range rows {
		for _, id := range templates {
			if row.TemplateID == id {
				ids = append(ids, row.Snapshot.ID)
				break
			}
		}
	}
	return ids
}
