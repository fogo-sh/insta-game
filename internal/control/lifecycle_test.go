package control

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryDiscardsIdleObservation(t *testing.T) {
	i := Instance{Desired: "running", Phase: "failed", Generation: 3, EmptySince: time.Now().Add(-time.Hour), Error: "exited", Players: new(int), IdleMinutes: 1}
	if err := setDesired(&i, "running"); err != nil {
		t.Fatal(err)
	}
	if i.Generation != 4 || i.Phase != "queued" || !i.EmptySince.IsZero() || i.Players != nil || i.Error != "" {
		t.Fatalf("retry retained old observation: %+v", i)
	}
	applyAgentStatus(&i, AgentStatus{Running: true, Ready: true}, nil)
	if i.Desired != "running" {
		t.Fatal("old idle interval stopped a fresh start")
	}
}

func TestTimeoutInvalidatesInFlightObservation(t *testing.T) {
	c, _, _ := setup(t)
	i := create(t, c, "timeout")
	if err := c.Store.Update(i.ID, func(current *Instance) error {
		current.StartedAt = time.Now().Add(-3 * time.Hour)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	i, _ = c.Store.Get(i.ID)
	c.reconcileInstance(context.Background(), i)
	stopped, _ := c.Store.Get(i.ID)
	if stopped.Desired != "stopped" || stopped.Generation <= i.Generation || stopped.StopReason != "Maximum runtime reached" {
		t.Fatalf("timeout failed: %+v", stopped)
	}
	if err := c.SetDesired(i.ID, "running"); err != nil {
		t.Fatal(err)
	}
	// Completing an older reconcile must not restore its phase, endpoints or error.
	c.reconcileInstance(context.Background(), i)
	restarted, _ := c.Store.Get(i.ID)
	if restarted.Desired != "running" || restarted.Phase != "queued" || restarted.Generation != stopped.Generation+1 || restarted.StopReason != "" {
		t.Fatalf("stale timeout overwrote restart: %+v", restarted)
	}
}

func TestDeletedIntentIsTerminalAndInvalidIntentIsRejected(t *testing.T) {
	i := Instance{Desired: "stopped", Phase: "stopped"}
	if err := setDesired(&i, "typo"); err == nil || i.Desired != "stopped" {
		t.Fatal("accepted invalid intent")
	}
	if err := setDesired(&i, "deleted"); err != nil {
		t.Fatal(err)
	}
	generation := i.Generation
	if err := setDesired(&i, "running"); err == nil || i.Generation != generation {
		t.Fatal("resurrected deleted instance")
	}
}

func TestProcessExitClearsIdleEvidence(t *testing.T) {
	i := Instance{Desired: "running", Phase: "running", EmptySince: time.Now().Add(-time.Hour), Players: new(int)}
	applyAgentStatus(&i, AgentStatus{Error: "crashed"}, nil)
	if i.Phase != "failed" || i.Error != "crashed" || i.Players != nil || !i.EmptySince.IsZero() {
		t.Fatalf("exit retained readiness: %+v", i)
	}
}

type delayedInspection struct {
	Provider
	entered chan struct{}
	release chan struct{}
}

func (p delayedInspection) Inspect(ctx context.Context, _ Instance) (Observation, error) {
	close(p.entered)
	select {
	case <-p.release:
		return Observation{}, errors.New("old provider failure")
	case <-ctx.Done():
		return Observation{}, ctx.Err()
	}
}

func TestStopWinsOverLateProviderFailure(t *testing.T) {
	c, provider, _ := setup(t)
	i := create(t, c, "late")
	delayed := delayedInspection{Provider: provider, entered: make(chan struct{}), release: make(chan struct{})}
	c.Providers["docker"] = delayed
	done := make(chan struct{})
	go func() { defer close(done); c.reconcileInstance(context.Background(), i) }()
	select {
	case <-delayed.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("inspection did not start")
	}
	if err := c.SetDesired(i.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	close(delayed.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reconcile did not finish")
	}
	current, err := c.Store.Get(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Desired != "stopped" || current.Error != "" || current.Generation != i.Generation+1 {
		t.Fatalf("late failure overwrote stop: %+v", current)
	}
	c.Providers["docker"] = provider
	c.Reconcile(context.Background())
	current, _ = c.Store.Get(i.ID)
	if current.Phase != "stopped" {
		t.Fatalf("stop did not settle: %+v", current)
	}
}
