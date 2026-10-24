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
	rows, err := w.Table.List(ctx)
	if err != nil {
		return fmt.Errorf("list lobby: %w", err)
	}
	for _, id := range managedIDs(rows, w.IDs) {
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
	if errors.Is(err, control.ErrNotFound) && row.TemplateID != "" && row.Pending(now) && row.Action == "start" {
		template, e := w.Controller.Store.Get(row.TemplateID)
		if e != nil && !errors.Is(e, control.ErrNotFound) {
			return e
		}
		if errors.Is(e, control.ErrNotFound) || template.Desired == "deleted" {
			return w.Table.Mark(ctx, id, row.RequestID, "completedID", "Game preset is no longer available")
		}
		config := template.Config
		instance, err = w.Controller.CreateRequested(control.CreateRequest{ID: id, Name: row.Snapshot.Name, Game: template.Game.ID, Provider: template.Provider, Config: &config, IdleMinutes: template.IdleMinutes, MaxMinutes: template.MaxMinutes}, row.RequestID)
	}
	if errors.Is(err, control.ErrNotFound) {
		if row.Pending(now) {
			reason := "Saved game no longer exists"
			if row.Action == "remove" {
				reason = ""
			}
			if err := w.Table.Mark(ctx, id, row.RequestID, "completedID", reason); err != nil {
				return err
			}
		}
		snapshot := Snapshot{ID: id, Name: id, Phase: "unavailable", Error: "Saved game no longer exists", ObservedAt: now}
		if row.TemplateID != "" {
			snapshot = row.Snapshot
			snapshot.Phase = "stopped"
			snapshot.Desired = "stopped"
			snapshot.Address = ""
			snapshot.Players = nil
			snapshot.ObservedAt = now
			if row.Action == "remove" {
				snapshot.Desired = "deleted"
				snapshot.Phase = "deleted"
			}
		}
		return w.Table.Publish(ctx, snapshot)
	}
	if errors.Is(err, control.ErrShuttingDown) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.RequestID != "" && row.RequestID != row.CompletedID {
		switch {
		case !row.Pending(now):
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", "Request expired; try again")
		case row.AcceptedID != row.RequestID:
			desired := map[string]string{"": "running", "start": "running", "stop": "stopped", "remove": "deleted"}[row.Action]
			err = w.Controller.RequestDesired(id, row.RequestID, desired)
			if errors.Is(err, control.ErrShuttingDown) {
				return nil
			}
			if err == nil {
				err = w.Table.Mark(ctx, id, row.RequestID, "acceptedID", "")
			}
		case row.Action == "stop" && instance.Phase == "stopped", row.Action == "remove" && instance.Phase == "deleted":
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", "")
		case (row.Action == "start" || row.Action == "") && instance.Phase == "running" && instance.Desired == "running" && instance.Players != nil && !instance.UpdatedAt.Before(w.StartedAt):
			err = w.Table.Mark(ctx, id, row.RequestID, "completedID", "")
		case (instance.Phase == "failed" && !instance.UpdatedAt.Before(w.StartedAt)) || ((row.Action == "" || row.Action == "start") && instance.Desired != "running"):
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
	if instance.Phase != "deleted" && instance.UpdatedAt.Before(w.StartedAt) {
		snapshot.Phase = "starting"
		snapshot.Address = ""
		snapshot.Players = nil
	}
	return w.Table.Publish(ctx, snapshot)
}

// Pending makes idle shutdown fail closed if request storage cannot be reached.
func (w *Worker) Pending(ctx context.Context, now time.Time) (bool, error) {
	rows, err := w.Table.List(ctx)
	if err != nil {
		return false, err
	}
	for _, id := range managedIDs(rows, w.IDs) {
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
