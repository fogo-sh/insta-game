package fleet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPageCatalogExistsWithoutHostsAndProtectsMutations(t *testing.T) {
	scheduler, store, _ := testScheduler()
	sum := sha256.Sum256([]byte("invitation"))
	handler := (&Page{Scheduler: scheduler, TokenHash: hex.EncodeToString(sum[:])}).Handler()
	call := func(path, method, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if r := call("/api/games", "GET", "", "wrong"); r.Code != 401 {
		t.Fatal(r.Code)
	}
	response := call("/api/games", "GET", "", "invitation")
	var listing struct {
		Choices      []Game
		Games, Hosts []json.RawMessage
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Choices) != 1 || len(listing.Hosts) != 0 || len(listing.Games) != 0 {
		t.Fatal(response.Body.String())
	}
	const id = "server-0123456789abcdef0123456789abcdef"
	response = call("/api/games", "POST", `{"id":"`+id+`","game":"quake","name":"Friday"}`, "invitation")
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	state, _ := store.Load(context.Background())
	if len(state.Hosts) != 1 || len(state.Servers) != 1 {
		t.Fatal("request failed to reserve independent server and host")
	}
	response = call("/api/hosts/another-host/games/"+id+"/stop", "POST", "", "invitation")
	if response.Code != 404 {
		t.Fatal("foreign host route mutated server")
	}
	state, _ = store.Load(context.Background())
	if state.Servers[0].Desired != "running" {
		t.Fatal("foreign route changed intent")
	}
}
func TestPageDoesNotAdvertiseStaleOrSleepingAddresses(t *testing.T) {
	for _, test := range []struct {
		name, hostState string
		age             time.Duration
		wantAddress     bool
	}{{"ready", "running", time.Second, true}, {"asleep", "stopped", time.Second, false}, {"stale", "running", time.Minute, false}} {
		t.Run(test.name, func(t *testing.T) {
			scheduler, store, machines := testScheduler()
			host := Host{ID: "host-a"}
			server := Server{ID: "server-a", Host: host.ID, Game: "quake", Desired: "running", RequestID: "request"}
			store.state = State{Hosts: []Host{host}, Servers: []Server{server}}
			machines.states[host.ID] = test.hostState
			players := 0
			store.reports[server.ID] = Report{ID: server.ID, Host: host.ID, RequestID: server.RequestID, Phase: "running", Desired: "running", Address: "1.2.3.4:1234", Players: &players, ObservedAt: time.Now().Add(-test.age)}
			page := &Page{Scheduler: scheduler}
			w := httptest.NewRecorder()
			page.list(w, httptest.NewRequest(http.MethodGet, "/api/games", nil))
			var body struct{ Games []struct{ Address string } }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if (body.Games[0].Address != "") != test.wantAddress {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestPageKeepsAcceptedStopAndRemovePendingUntilBackendFinishes(t *testing.T) {
	for _, desired := range []string{"stopped", "deleted"} {
		t.Run(desired, func(t *testing.T) {
			scheduler, store, machines := testScheduler()
			host := Host{ID: "host-a"}
			server := Server{ID: "server-a", Host: host.ID, Game: "quake", Desired: desired, RequestID: "request"}
			store.state = State{Hosts: []Host{host}, Servers: []Server{server}}
			machines.states[host.ID] = "running"
			store.reports[server.ID] = Report{ID: server.ID, Host: host.ID, RequestID: server.RequestID, Phase: "running", Desired: desired, ObservedAt: time.Now()}
			w := httptest.NewRecorder()
			(&Page{Scheduler: scheduler}).list(w, httptest.NewRequest(http.MethodGet, "/api/games", nil))
			var body struct {
				Games []struct {
					Pending bool
					Address string
				}
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !body.Games[0].Pending || body.Games[0].Address != "" {
				t.Fatal("accepted intent was advertised as finished", w.Body.String())
			}
		})
	}
}
