package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func testTable(t *testing.T, handler http.HandlerFunc) *Table {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := dynamodb.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), RetryMaxAttempts: 1}, func(o *dynamodb.Options) { o.BaseEndpoint = aws.String(server.URL) })
	return &Table{Client: client, Name: "test-fleet"}
}

func TestStateCASDoesNotOverwriteConcurrentChanges(t *testing.T) {
	var requests []map[string]json.RawMessage
	table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
		var input map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		requests = append(requests, input)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if len(requests) == 2 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"changed"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})
	ctx := context.Background()
	state := State{}
	if err := table.Save(ctx, state, 0); err != nil {
		t.Fatal(err)
	}
	if err := table.Save(ctx, state, 7); !errors.Is(err, ErrConflict) {
		t.Fatalf("concurrent save: %v", err)
	}
	if state.Revision != 0 {
		t.Fatal("Save mutated its caller's state")
	}
	if string(requests[0]["ConditionExpression"]) != `"attribute_not_exists(id)"` || string(requests[1]["ConditionExpression"]) != `"revision = :previous"` {
		t.Fatal("save did not condition initial and subsequent writes")
	}
	var item map[string]map[string]string
	if err := json.Unmarshal(requests[1]["Item"], &item); err != nil {
		t.Fatal(err)
	}
	if item["revision"]["N"] != "8" || !strings.Contains(item["state"]["S"], `"revision":8`) {
		t.Fatal("serialized state and condition revision disagree")
	}
}

func TestForgetAtomicallyReleasesAndTombstones(t *testing.T) {
	var called bool
	table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if !strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".TransactWriteItems") {
			t.Error("delete must be a transaction")
		}
		var input struct{ TransactItems []map[string]json.RawMessage }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if len(input.TransactItems) != 3 {
			t.Fatalf("transaction has %d writes", len(input.TransactItems))
		}
		if !strings.Contains(string(input.TransactItems[0]["Put"]), `revision = :previous`) || !strings.Contains(string(input.TransactItems[1]["Put"]), "tombstone/server-one") || !strings.Contains(string(input.TransactItems[2]["Delete"]), "report/server-one") {
			t.Error("incomplete atomic deletion")
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#TransactionCanceledException","CancellationReasons":[{"Code":"ConditionalCheckFailed"},{"Code":"None"},{"Code":"None"}]}`))
	})
	if err := table.Forget(context.Background(), State{Servers: []Server{{ID: "server-one"}}}, 3, "server-one"); err == nil || called {
		t.Fatal("deletion accepted a server still in registry")
	}
	if err := table.Forget(context.Background(), State{}, 3, "server-one"); !errors.Is(err, ErrConflict) {
		t.Fatalf("delete conflict: %v", err)
	}
}

func TestPublishRejectsForeignHostAndDoesNotResurrectDeletedServer(t *testing.T) {
	for _, tc := range []struct {
		name, reasons string
		want          error
	}{
		{"deleted", `[{"Code":"ConditionalCheckFailed"},{"Code":"None"}]`, nil},
		{"foreign host", `[{"Code":"None"},{"Code":"ConditionalCheckFailed"}]`, ErrReportOwner},
		{"transaction interrupted", `[{"Code":"None"},{"Code":"TransactionConflict"}]`, errors.New("transaction conflict")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
				var input struct{ TransactItems []map[string]json.RawMessage }
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				if len(input.TransactItems) != 2 || !strings.Contains(string(input.TransactItems[0]["ConditionCheck"]), "tombstone/server-one") || !strings.Contains(string(input.TransactItems[1]["Put"]), "#host = :host") {
					t.Error("report publication lost ownership or deletion guard")
				}
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#TransactionCanceledException","CancellationReasons":` + tc.reasons + `}`))
			})
			err := table.Publish(context.Background(), Report{ID: "server-one", Host: "host-one", RequestID: "request-one", ObservedAt: time.Now()})
			if tc.want == nil && err != nil || tc.want != nil && err == nil || tc.want == ErrReportOwner && !errors.Is(err, ErrReportOwner) {
				t.Fatalf("Publish() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestStateBoundsAndCorruptionFailClosed(t *testing.T) {
	table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		_, _ = w.Write([]byte(`{"Item":{"id":{"S":"fleet/state"},"revision":{"N":"5"},"state":{"S":"{\"revision\":4}"}}}`))
	})
	if _, err := table.Load(context.Background()); err == nil {
		t.Fatal("inconsistent stored revision accepted")
	}
	for _, state := range []State{
		{Servers: make([]Server, 129)},
		{Hosts: make([]Host, 17)},
		{Hosts: []Host{{Error: strings.Repeat("x", maxStateBytes)}}},
	} {
		if _, err := table.statePut(state, 1); err == nil {
			t.Fatal("oversized state accepted")
		}
	}
}

func TestReportsReadsOnlyActiveIDsAndRetriesIncompleteBatches(t *testing.T) {
	state := State{Revision: 1}
	for i := range 128 {
		state.Servers = append(state.Servers, Server{ID: fmt.Sprintf("server-%03d", i)})
	}
	encoded, _ := json.Marshal(state)
	batches := 0
	table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".GetItem") {
			_ = json.NewEncoder(w).Encode(map[string]any{"Item": map[string]any{"revision": map[string]string{"N": "1"}, "state": map[string]string{"S": string(encoded)}}})
			return
		}
		if !strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".BatchGetItem") {
			t.Error("reports must not scan permanent deletion history")
		}
		batches++
		var request struct {
			RequestItems map[string]struct {
				Keys           []map[string]map[string]string
				ConsistentRead bool
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		batch := request.RequestItems["test-fleet"]
		if len(batch.Keys) > 100 || !batch.ConsistentRead {
			t.Error("invalid report read batch")
		}
		var items []map[string]any
		response := map[string]any{}
		for i, k := range batch.Keys {
			if batches == 1 && i == 0 {
				response["UnprocessedKeys"] = map[string]any{"test-fleet": map[string]any{"Keys": batch.Keys[:1], "ConsistentRead": true}}
				continue
			}
			report, _ := json.Marshal(Report{ID: strings.TrimPrefix(k["id"]["S"], "report/")})
			items = append(items, map[string]any{"report": map[string]string{"S": string(report)}})
		}
		response["Responses"] = map[string]any{"test-fleet": items}
		_ = json.NewEncoder(w).Encode(response)
	})
	reports, err := table.Reports(context.Background())
	if err != nil || len(reports) != 128 || batches != 3 {
		t.Fatalf("reports=%d batches=%d error=%v", len(reports), batches, err)
	}
}

func TestReportsFailsVisiblyWhenBatchRemainsIncomplete(t *testing.T) {
	batches := 0
	table := testTable(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".GetItem") {
			_, _ = w.Write([]byte(`{"Item":{"revision":{"N":"1"},"state":{"S":"{\"revision\":1,\"servers\":[{\"id\":\"server-one\"}]}"}}}`))
			return
		}
		batches++
		var input map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"UnprocessedKeys": input["RequestItems"]})
	})
	reports, err := table.Reports(context.Background())
	if err == nil || len(reports) != 0 || batches != 3 {
		t.Fatalf("incomplete read looked successful: reports=%d attempts=%d err=%v", len(reports), batches, err)
	}
}
