package control

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDockerMutationsVerifyOwnershipAndUseImmutableID(t *testing.T) {
	instance := Instance{ID: "collision", AgentToken: "instance-secret"}
	owned := false
	mutations := 0
	docker := &Docker{client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			labels := map[string]string{}
			if owned {
				labels = map[string]string{"gg.insta-game.instance": instance.ID, "gg.insta-game.owner": owner(instance)}
			}
			data, _ := json.Marshal(map[string]any{"Id": "immutable-container-id", "Config": map[string]any{"Labels": labels}})
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
		}
		mutations++
		if r.URL.Path != "/containers/immutable-container-id" {
			t.Errorf("mutation targeted a replaceable name: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}}
	if err := docker.Delete(context.Background(), instance); err == nil {
		t.Fatal("deleted an unmanaged container")
	}
	if mutations != 0 {
		t.Fatal("mutated an unmanaged container")
	}
	owned = true
	if err := docker.Delete(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	if mutations != 1 {
		t.Fatal("did not remove owned resource")
	}
}

func TestDockerRebootRecoveryKeepsSameBootFailures(t *testing.T) {
	instance := Instance{ID: "reboot", AgentToken: "secret"}
	boot := time.Unix(200, 0)
	for _, started := range []time.Time{boot.Add(-time.Minute), boot.Add(time.Minute)} {
		t.Run(started.String(), func(t *testing.T) {
			docker := &Docker{bootedAt: boot, client: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				value := map[string]any{"Id": "owned", "Config": map[string]any{"Labels": map[string]string{"gg.insta-game.instance": instance.ID, "gg.insta-game.owner": owner(instance)}}, "State": map[string]any{"Status": "exited", "Running": false, "ExitCode": 137, "StartedAt": started}}
				data, _ := json.Marshal(value)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}}
			got, err := docker.Inspect(context.Background(), instance)
			if err != nil {
				t.Fatal(err)
			}
			if started.Before(boot) {
				if got.Started || got.Failure != "" {
					t.Fatalf("host reboot must permit restart: %+v", got)
				}
			} else if !got.Started || got.Failure == "" {
				t.Fatalf("same-boot crash was hidden: %+v", got)
			}
		})
	}
}
