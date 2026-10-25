package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const registryKey = "fleet/state"
const maxStateBytes = 300 * 1024

type Table struct {
	Client *dynamodb.Client
	Name   string
}

func text(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"id": text(id)}
}
func stringAttr(item map[string]types.AttributeValue, name string) string {
	if v, ok := item[name].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func (t *Table) Load(ctx context.Context) (State, error) {
	result, err := t.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(t.Name), Key: key(registryKey), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return State{}, err
	}
	if len(result.Item) == 0 {
		return State{}, nil
	}
	var state State
	if err := json.Unmarshal([]byte(stringAttr(result.Item, "state")), &state); err != nil {
		return State{}, fmt.Errorf("decode fleet state: %w", err)
	}
	revision, ok := result.Item["revision"].(*types.AttributeValueMemberN)
	if !ok || revision.Value != strconv.FormatInt(state.Revision, 10) || state.Revision < 1 {
		return State{}, fmt.Errorf("fleet state revision is inconsistent")
	}
	return state, nil
}

func (t *Table) statePut(state State, previous int64) (*dynamodb.PutItemInput, error) {
	if previous < 0 || previous == math.MaxInt64 || len(state.Hosts) > 16 || len(state.Servers) > 128 {
		return nil, fmt.Errorf("fleet state exceeds revision or allocation limits")
	}
	state.Revision = previous + 1
	data, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if len(data) > maxStateBytes {
		return nil, fmt.Errorf("fleet state exceeds %d bytes", maxStateBytes)
	}
	input := &dynamodb.PutItemInput{
		TableName:                 aws.String(t.Name),
		Item:                      map[string]types.AttributeValue{"id": text(registryKey), "revision": &types.AttributeValueMemberN{Value: strconv.FormatInt(state.Revision, 10)}, "state": text(string(data))},
		ConditionExpression:       aws.String("revision = :previous"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":previous": &types.AttributeValueMemberN{Value: strconv.FormatInt(previous, 10)}},
	}
	if previous == 0 {
		input.ConditionExpression = aws.String("attribute_not_exists(id)")
		input.ExpressionAttributeValues = nil
	}
	return input, nil
}

// Save commits one allocation decision. The caller must reload after a conflict.
func (t *Table) Save(ctx context.Context, state State, previous int64) error {
	input, err := t.statePut(state, previous)
	if err != nil {
		return err
	}
	_, err = t.Client.PutItem(ctx, input)
	var conflict *types.ConditionalCheckFailedException
	if errors.As(err, &conflict) {
		return ErrConflict
	}
	return err
}

func (t *Table) Tombstoned(ctx context.Context, id string) (bool, error) {
	result, err := t.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(t.Name), Key: key("tombstone/" + id), ConsistentRead: aws.Bool(true), ProjectionExpression: aws.String("id")})
	if err != nil {
		return false, err
	}
	return len(result.Item) != 0, nil
}

// Forget releases capacity only when the permanent tombstone is committed too.
func (t *Table) Forget(ctx context.Context, state State, previous int64, id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("invalid forgotten server ID")
	}
	for _, server := range state.Servers {
		if server.ID == id {
			return fmt.Errorf("forgotten server remains in fleet state")
		}
	}
	input, err := t.statePut(state, previous)
	if err != nil {
		return err
	}
	_, err = t.Client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{Put: &types.Put{TableName: input.TableName, Item: input.Item, ConditionExpression: input.ConditionExpression, ExpressionAttributeValues: input.ExpressionAttributeValues}},
		{Put: &types.Put{TableName: aws.String(t.Name), Item: key("tombstone/" + id)}},
		{Delete: &types.Delete{TableName: aws.String(t.Name), Key: key("report/" + id)}},
	}})
	if transactionCondition(err, 0) {
		return ErrConflict
	}
	return err
}

func (t *Table) Reports(ctx context.Context) ([]Report, error) {
	state, err := t.Load(ctx)
	if err != nil {
		return nil, err
	}
	var reports []Report
	for start := 0; start < len(state.Servers); start += 100 {
		var keys []map[string]types.AttributeValue
		for _, server := range state.Servers[start:min(start+100, len(state.Servers))] {
			keys = append(keys, key("report/"+server.ID))
		}
		pending := map[string]types.KeysAndAttributes{t.Name: {Keys: keys, ConsistentRead: aws.Bool(true)}}
		for attempt := 0; len(pending) > 0; attempt++ {
			if attempt == 3 {
				return nil, fmt.Errorf("fleet report read remained incomplete after three attempts")
			}
			if attempt > 0 {
				timer := time.NewTimer(time.Duration(attempt) * 100 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
			result, err := t.Client.BatchGetItem(ctx, &dynamodb.BatchGetItemInput{RequestItems: pending})
			if err != nil {
				return nil, err
			}
			for _, item := range result.Responses[t.Name] {
				var report Report
				if err := json.Unmarshal([]byte(stringAttr(item, "report")), &report); err != nil {
					return nil, fmt.Errorf("decode fleet report: %w", err)
				}
				reports = append(reports, report)
			}
			pending = result.UnprocessedKeys
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].ID < reports[j].ID })
	return reports, nil
}

func (t *Table) Publish(ctx context.Context, report Report) error {
	if !validID.MatchString(report.ID) || !validID.MatchString(report.Host) || report.RequestID == "" || report.ObservedAt.IsZero() {
		return fmt.Errorf("report requires server, host, request, and observation time")
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if len(data) > 16*1024 {
		return fmt.Errorf("fleet report exceeds 16 KiB")
	}
	_, err = t.Client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		{ConditionCheck: &types.ConditionCheck{TableName: aws.String(t.Name), Key: key("tombstone/" + report.ID), ConditionExpression: aws.String("attribute_not_exists(id)")}},
		{Put: &types.Put{TableName: aws.String(t.Name), Item: map[string]types.AttributeValue{"id": text("report/" + report.ID), "host": text(report.Host), "report": text(string(data))}, ConditionExpression: aws.String("attribute_not_exists(#host) OR #host = :host"), ExpressionAttributeNames: map[string]string{"#host": "host"}, ExpressionAttributeValues: map[string]types.AttributeValue{":host": text(report.Host)}}},
	}})
	if transactionCondition(err, 0) {
		return nil // A late worker must not resurrect a removed server's report.
	}
	if transactionCondition(err, 1) {
		return ErrReportOwner
	}
	return err
}

func transactionCondition(err error, index int) bool {
	var canceled *types.TransactionCanceledException
	return errors.As(err, &canceled) && len(canceled.CancellationReasons) > index && aws.ToString(canceled.CancellationReasons[index].Code) == "ConditionalCheckFailed"
}
