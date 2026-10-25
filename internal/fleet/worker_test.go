package fleet

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

func testWorker(t *testing.T) (*Worker, *Scheduler, *memoryStore, Server) {
	t.Helper()
	scheduler, store, _ := testScheduler()
	server, err := scheduler.Create(context.Background(), "server-one", "quake", "Friday")
	if err != nil {
		t.Fatal(err)
	}
	local, err := control.OpenStore(filepath.Join(t.TempDir(), "controller.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = local.Close() })
	controller := control.NewController(local, []control.Game{{ID: "quake", DisplayName: "Quake"}}, map[string]control.Provider{"docker": &control.Docker{}}, "docker")
	worker := &Worker{Store: store, Host: server.Host, Config: scheduler.Config, Controller: controller, StartedAt: time.Now()}
	return worker, scheduler, store, server
}

func TestWorkerLostAcknowledgementCannotUndoLaterStop(t *testing.T) {
	worker, scheduler, store, server := testWorker(t)
	ctx := context.Background()
	store.publishErr = errors.New("temporary table failure")
	if err := worker.Sync(ctx); err == nil {
		t.Fatal("publication error hidden")
	}
	instance, err := worker.Controller.Store.Get(server.ID)
	if err != nil || instance.StartRequestID != server.RequestID {
		t.Fatalf("intent was not committed with its request: %v", err)
	}
	if err := worker.Controller.SetDesired(server.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	if err := worker.Controller.Store.Update(server.ID, func(i *control.Instance) error {
		i.Phase = "stopped"
		i.UpdatedAt = time.Now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	store.publishErr = nil
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	instance, _ = worker.Controller.Store.Get(server.ID)
	if instance.Desired != "stopped" || store.reports[server.ID].Desired != "stopped" {
		t.Fatal("retried cloud intent restarted a locally stopped server")
	}
	// Start arrives before the scheduled reconciler mirrors the local idle stop.
	if err := scheduler.Action(ctx, server.ID, "start"); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	instance, _ = worker.Controller.Store.Get(server.ID)
	if instance.Desired != "running" || instance.StartRequestID == server.RequestID {
		t.Fatal("explicit new start did not replace accepted stop")
	}
}

func TestWorkerSuppressesPrebootReadinessAndReportsSetupFailure(t *testing.T) {
	worker, _, store, server := testWorker(t)
	ctx := context.Background()
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Controller.Store.Update(server.ID, func(i *control.Instance) error {
		players := 2
		i.Phase = "running"
		i.Players = &players
		i.UpdatedAt = worker.StartedAt.Add(-time.Minute)
		i.Endpoints = []control.Endpoint{{Address: "old-address:26000"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	report := store.reports[server.ID]
	if report.Phase != "starting" || report.Address != "" || report.Players != nil {
		t.Fatal("historical readiness advertised after host restart")
	}
	store.state.Servers = append(store.state.Servers, Server{ID: "server-unknown", Host: server.Host, Game: "unknown", Name: "Unknown", Desired: "running", RequestID: "new", RequestedAt: time.Now().Unix()})
	if err := worker.Sync(ctx); err == nil {
		t.Fatal("unknown catalog game silently succeeded")
	}
	report = store.reports["server-unknown"]
	if report.Phase != "failed" || report.Error == "" || report.RequestID != "new" {
		t.Fatal("setup failure was not published for the current request")
	}
}

func TestWorkerDoesNotStartExpiredRequestAfterLateBootstrap(t *testing.T) {
	worker, _, store, server := testWorker(t)
	store.state.Servers[0].RequestedAt = time.Now().Add(-StartupTimeout - time.Minute).Unix()
	if err := worker.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Controller.Store.Get(server.ID); !errors.Is(err, control.ErrNotFound) {
		t.Fatal("late host created an expired server")
	}
	if r := store.reports[server.ID]; r.Error != StartupTimeoutMessage || r.Phase != "failed" {
		t.Fatal("late bootstrap timeout was not visible")
	}
}

func TestDelayedCentralAcknowledgementDoesNotStopReadyGame(t *testing.T) {
	worker, scheduler, store, server := testWorker(t)
	ctx := context.Background()
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	store.state.Servers[0].RequestedAt = time.Now().Add(-time.Hour).Unix()
	if err := worker.Controller.Store.Update(server.ID, func(i *control.Instance) error {
		players := 1
		i.Phase = "running"
		i.Players = &players
		i.UpdatedAt = time.Now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	instance, _ := worker.Controller.Store.Get(server.ID)
	if instance.Desired != "running" || store.state.Servers[0].ReadyAt == 0 || store.state.Servers[0].Error != "" {
		t.Fatal("healthy game timed out because scheduled reconciliation was delayed")
	}
}

func TestWorkerWaitsForInitialAgentButReportsLossAfterReadiness(t *testing.T) {
	worker, _, store, server := testWorker(t)
	ctx := context.Background()
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.Controller.Store.Update(server.ID, func(i *control.Instance) error {
		i.Phase = "starting"
		i.Error = "Agent unavailable: connection refused"
		i.UpdatedAt = time.Now()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if r := store.reports[server.ID]; r.Error != "" || r.Phase != "starting" || r.Address != "" {
		t.Fatal("initial agent startup reported as a terminal failure", r)
	}
	store.state.Servers[0].ReadyAt = time.Now().Unix()
	if err := worker.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if store.reports[server.ID].Error == "" {
		t.Fatal("lost agent after readiness was hidden")
	}
}
