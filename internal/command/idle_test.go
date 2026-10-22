package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

func TestIdleWindowRequiresConfirmedStopsAndResetsOnActivity(t *testing.T) {
	start := time.Unix(1, 0)
	if got := idleSince(nil, time.Time{}, start); got != start {
		t.Fatal("empty startup did not begin a full idle window")
	}
	stopped := control.Instance{Desired: "stopped", Phase: "stopped"}
	deleted := control.Instance{Desired: "deleted", Phase: "deleted"}
	if got := idleSince([]control.Instance{stopped, deleted}, start, start.Add(time.Minute)); got != start {
		t.Fatal("confirmed idle state restarted the window")
	}
	for _, busy := range []control.Instance{
		{Desired: "running", Phase: "running"},
		{Desired: "running", Phase: "stopped"},
		{Desired: "stopped", Phase: "running"},
		{Desired: "stopped", Phase: "starting"},
		{Desired: "stopped", Phase: "failed"},
		{Desired: "deleted", Phase: "deleting"},
		{Desired: "deleted", Phase: "stopped"},
		{Desired: "stopped", Phase: "stopped", Error: "provider unreachable"},
	} {
		if got := idleSince([]control.Instance{stopped, busy}, start, start.Add(time.Minute)); !got.IsZero() {
			t.Fatalf("unsafe idle state: %+v", busy)
		}
	}
	resumed := start.Add(2 * time.Minute)
	if got := idleSince([]control.Instance{stopped}, time.Time{}, resumed); got != resumed {
		t.Fatal("idle window included earlier active time")
	}
}

func TestPoweroffWaitsFullStartupGrace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	const grace = 20 * time.Millisecond
	start := time.Now()
	calls := 0
	err := poweroffWhenIdle(ctx, grace, func() ([]control.Instance, error) { return nil, nil }, func(context.Context) (bool, error) {
		calls++
		if time.Since(start) < grace {
			t.Fatal("powered off before the full grace period")
		}
		return true, nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("poweroff calls=%d error=%v", calls, err)
	}
}

func TestPoweroffDisabledCanceledOrUnobservableDoesNothing(t *testing.T) {
	for _, mode := range []string{"disabled", "canceled", "cancel during observation", "observation failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			duration := time.Millisecond
			if mode == "disabled" {
				duration = 0
			}
			if mode == "canceled" {
				cancel()
			}
			observations := 0
			err := poweroffWhenIdle(ctx, duration, func() ([]control.Instance, error) {
				observations++
				if mode == "disabled" || mode == "canceled" {
					t.Fatal("observed state while disabled/canceled")
				}
				if mode == "cancel during observation" {
					if observations > 1 {
						cancel()
					}
					return nil, nil
				}
				return nil, errors.New("database unavailable")
			}, func(context.Context) (bool, error) {
				t.Fatal("unsafe poweroff callback")
				return true, nil
			})
			if mode != "disabled" && err == nil {
				t.Fatal("hid cancellation or observation failure")
			}
		})
	}
}

func TestIdleShutdownRaceRestartsGrace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	const grace = 10 * time.Millisecond
	calls := 0
	var rejected time.Time
	err := poweroffWhenIdle(ctx, grace, func() ([]control.Instance, error) { return nil, nil }, func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			rejected = time.Now()
			return false, nil
		}
		if time.Since(rejected) < grace {
			t.Fatal("retry skipped the new idle grace")
		}
		return true, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("poweroff attempts=%d error=%v", calls, err)
	}
}
