package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testProvider struct {
	mu        sync.Mutex
	resources map[string]Observation
	failure   error
	agent     string
}

func (p *testProvider) Ensure(_ context.Context, i Instance) (Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := Observation{Exists: true, AgentURL: p.agent}
	p.resources[i.ID] = o
	return o, p.failure
}
func (p *testProvider) Inspect(_ context.Context, i Instance) (Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.resources[i.ID], p.failure
}
func (p *testProvider) Start(_ context.Context, i Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.resources[i.ID]
	o.Running = true
	o.Started = true
	p.resources[i.ID] = o
	return p.failure
}
func (p *testProvider) Stop(_ context.Context, i Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.resources[i.ID]
	o.Running = false
	p.resources[i.ID] = o
	return p.failure
}
func (p *testProvider) Delete(_ context.Context, i Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.resources, i.ID)
	return p.failure
}
func setup(t *testing.T) (*Controller, *testProvider, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "instances.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	p := &testProvider{resources: map[string]Observation{}}
	return NewController(store, []Game{{ID: "xonotic", DisplayName: "Xonotic", DefaultConfig: "hostname test", Image: "test"}}, map[string]Provider{"docker": p}, "docker"), p, path
}
func create(t *testing.T, c *Controller, id string) Instance {
	t.Helper()
	i, err := c.Create(CreateRequest{ID: id, Game: "xonotic", Name: id, MaxMinutes: 120, IdleMinutes: 10})
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func TestInstancesRecoverIndependentlyAndCreationIsIdempotent(t *testing.T) {
	c, p, path := setup(t)
	first := create(t, c, "first")
	second := create(t, c, "second")
	retry := create(t, c, "first")
	if retry.AgentToken != first.AgentToken {
		t.Fatal("retry replaced credentials")
	}
	if _, err := c.Create(CreateRequest{ID: "first", Game: "xonotic", Name: "different"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if first.AgentToken == second.AgentToken {
		t.Fatal("instances share credentials")
	}
	c.Reconcile(context.Background())
	if !p.resources["first"].Running || !p.resources["second"].Running {
		t.Fatal("did not start both")
	}
	if err := c.SetDesired("first", "stopped"); err != nil {
		t.Fatal(err)
	}
	c.Store.Close()
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Store = store
	t.Cleanup(func() { store.Close() })
	c.Reconcile(context.Background())
	c.Reconcile(context.Background())
	a, _ := store.Get("first")
	b, _ := store.Get("second")
	if a.Phase != "stopped" || !p.resources[b.ID].Running || b.AgentToken != second.AgentToken {
		t.Fatalf("recovery: %s %#v", a.Phase, b)
	}
	if err := c.SetDesired("first", "deleted"); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background())
	if _, ok := p.resources["first"]; ok {
		t.Fatal("delete retained infrastructure")
	}
	if _, ok := p.resources["second"]; !ok {
		t.Fatal("delete touched another instance")
	}
}
func TestUnknownPlayersDoNotTriggerIdleShutdown(t *testing.T) {
	c, p, _ := setup(t)
	status := AgentStatus{Running: true, Ready: false}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(status) }))
	defer server.Close()
	p.agent = server.URL
	i := create(t, c, "idle")
	c.Reconcile(context.Background())
	c.Store.Update(i.ID, func(i *Instance) error { i.EmptySince = time.Now().Add(-time.Hour); return nil })
	c.Reconcile(context.Background())
	i, _ = c.Store.Get(i.ID)
	if i.Desired != "running" || !i.EmptySince.IsZero() || i.Players != nil {
		t.Fatalf("unknown counted as empty: %#v", i)
	}
	status.Ready = true
	c.Store.Update(i.ID, func(i *Instance) error { i.EmptySince = time.Now().Add(-time.Hour); return nil })
	c.Reconcile(context.Background())
	i, _ = c.Store.Get(i.ID)
	if i.Desired != "stopped" || i.StopReason == "" {
		t.Fatalf("confirmed empty did not stop: %#v", i)
	}
}
func TestAPIRequiresTokenAndNeverReturnsInstanceCredentials(t *testing.T) {
	c, _, _ := setup(t)
	i := create(t, c, "private")
	handler := c.Handler("controller-secret")
	request := httptest.NewRequest("GET", "/api/instances", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatalf("anonymous list: %d", response.Code)
	}
	request.Header.Set("Authorization", "Bearer controller-secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || strings.Contains(response.Body.String(), i.AgentToken) || strings.Contains(response.Body.String(), i.RconPassword) {
		t.Fatalf("credentials leaked: %d", response.Code)
	}
	request = httptest.NewRequest("GET", "/api/instances/private/stop", nil)
	request.Header.Set("Authorization", "Bearer controller-secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code < 400 {
		t.Fatal("GET mutation accepted")
	}
}
func TestProviderFailurePreservesObservationAndMaximumLifetime(t *testing.T) {
	c, p, _ := setup(t)
	i := create(t, c, "outage")
	c.Reconcile(context.Background())
	p.failure = errors.New("provider unreachable")
	c.Reconcile(context.Background())
	i, _ = c.Store.Get(i.ID)
	if i.Error == "" || i.Phase == "stopped" {
		t.Fatalf("outage hidden: %#v", i)
	}
	c.Store.Update(i.ID, func(i *Instance) error { i.StartedAt = time.Now().Add(-3 * time.Hour); return nil })
	c.Reconcile(context.Background())
	i, _ = c.Store.Get(i.ID)
	if i.Desired != "stopped" {
		t.Fatal("maximum lifetime ignored during provider outage")
	}
}

func TestStartupCrashRequiresExplicitRetry(t *testing.T) {
	c, p, _ := setup(t)
	i := create(t, c, "broken")
	c.Reconcile(context.Background())
	p.mu.Lock()
	observation := p.resources[i.ID]
	observation.Running = false
	observation.Failure = "bad startup configuration"
	p.resources[i.ID] = observation
	p.mu.Unlock()
	c.Reconcile(context.Background())
	c.Reconcile(context.Background())
	current, _ := c.Store.Get(i.ID)
	if current.Phase != "failed" || p.resources[i.ID].Running {
		t.Fatalf("startup crash restarted: %#v", current)
	}
	c.SetDesired(i.ID, "stopped")
	c.Reconcile(context.Background())
	c.SetDesired(i.ID, "running")
	c.Reconcile(context.Background())
	if !p.resources[i.ID].Running {
		t.Fatal("explicit stop/start did not retry")
	}
}

func TestConcurrentCreateRetriesReturnOneInstance(t *testing.T) {
	c, _, _ := setup(t)
	results := make(chan Instance, 8)
	failures := make(chan error, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			i, err := c.Create(CreateRequest{ID: "same-request", Game: "xonotic", Name: "Friday"})
			if err != nil {
				failures <- err
				return
			}
			results <- i
		})
	}
	workers.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	token := ""
	for i := range results {
		if token == "" {
			token = i.AgentToken
		}
		if i.AgentToken != token {
			t.Fatal("concurrent retry replaced credentials")
		}
	}
	list, err := c.Store.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("created duplicate instances: %d, %v", len(list), err)
	}
}

func TestProviderSelectionIsDurableAndPartOfCreationIdentity(t *testing.T) {
	c, docker, _ := setup(t)
	local := &testProvider{resources: map[string]Observation{}}
	c.Providers["local"] = local
	c.DefaultProvider = "local"
	request := CreateRequest{ID: "selected", Game: "xonotic", Name: "Selected"}
	instance, err := c.Create(request)
	if err != nil {
		t.Fatal(err)
	}
	if instance.Provider != "local" {
		t.Fatal("default was not saved")
	}
	c.DefaultProvider = "docker"
	request.Provider = "local"
	if _, err := c.Create(request); err != nil {
		t.Fatal("explicit retry failed", err)
	}
	request.Provider = "docker"
	if _, err := c.Create(request); !errors.Is(err, ErrConflict) {
		t.Fatalf("provider change = %v", err)
	}
	request.ID = "missing"
	request.Provider = "aws"
	if _, err := c.Create(request); err == nil {
		t.Fatal("accepted an unconfigured provider")
	}
	c.Reconcile(context.Background())
	if len(local.resources) != 1 || len(docker.resources) != 0 {
		t.Fatal("reconciled through the wrong provider")
	}
}
