package lobby

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

type Worker struct {
	Table      Storage
	IDs        []string
	Controller *control.Controller
	StartedAt  time.Time
}

func (w *Worker) Sync(ctx context.Context, now time.Time) error {
	for _, id := range w.IDs {
		if err := w.syncInstance(ctx, id, now); err != nil {
			return fmt.Errorf("lobby %s: %w", id, err)
		}
	}
	return nil
}
func (w *Worker) syncInstance(ctx context.Context, id string, now time.Time) error {
	row, err := w.Table.Read(ctx, id)
	if err != nil {
		return err
	}
	instance, err := w.Controller.Store.Get(id)
	if errors.Is(err, control.ErrNotFound) || (err == nil && instance.Desired == "deleted") {
		if row.Pending(now) {
			if err := w.Table.Mark(ctx, id, row.RequestID, "completedID", "Saved game no longer exists"); err != nil {
				return err
			}
		}
		return w.Table.Publish(ctx, Snapshot{ID: id, Name: id, Phase: "unavailable", Error: "Saved game no longer exists", ObservedAt: now})
	}
	if err != nil {
		return err
	}
	if row.RequestID != "" && row.RequestID != row.CompletedID {
		switch {
		case !row.Pending(now):
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", "Start request expired; try again")
		case row.AcceptedID != row.RequestID:
			err = w.Controller.RequestStart(id, row.RequestID)
			if errors.Is(err, control.ErrShuttingDown) {
				return nil
			}
			if err == nil {
				err = w.Table.Mark(ctx, id, row.RequestID, "acceptedID", "")
			}
		case instance.Phase == "running" && instance.Desired == "running" && instance.Players != nil && !instance.UpdatedAt.Before(w.StartedAt):
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", "")
		case (instance.Phase == "failed" && !instance.UpdatedAt.Before(w.StartedAt)) || instance.Desired != "running":
			reason := publicError(instance.Error)
			if reason == "" {
				reason = instance.StopReason
			}
			if reason == "" {
				reason = "Game stopped before becoming ready"
			}
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", reason)
		}
		if err != nil {
			return err
		}
	}
	// Re-read after accepting intent; never publish the pre-start state as current.
	instance, err = w.Controller.Store.Get(id)
	if err != nil {
		return err
	}
	snapshot := Snapshot{ID: id, Name: instance.Name, Game: instance.Game.DisplayName, Phase: instance.Phase, Desired: instance.Desired, Address: instance.Connect(), Players: instance.Players, Error: publicError(instance.Error), ObservedAt: instance.UpdatedAt}
	if instance.UpdatedAt.Before(w.StartedAt) {
		snapshot.Phase = "starting"
		snapshot.Address = ""
		snapshot.Players = nil
	}
	return w.Table.Publish(ctx, snapshot)
}

// Pending makes idle shutdown fail closed if request storage cannot be reached.
func (w *Worker) Pending(ctx context.Context, now time.Time) (bool, error) {
	for _, id := range w.IDs {
		row, err := w.Table.Read(ctx, id)
		if err != nil {
			return false, err
		}
		if row.Pending(now) {
			return true, nil
		}
	}
	return false, nil
}
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		opCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := w.Sync(opCtx, time.Now().UTC())
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func publicError(value string) string {
	if value != "" {
		return "Game is unavailable; ask the host administrator"
	}
	return ""
}
