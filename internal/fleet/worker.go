package fleet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

type Worker struct {
	Store      Store
	Host       string
	Config     Config
	Controller *control.Controller
	StartedAt  time.Time
}

func (w *Worker) Sync(ctx context.Context) error {
	state, err := w.Store.Load(ctx)
	if err != nil {
		return err
	}
	owned := false
	for _, h := range state.Hosts {
		if h.ID == w.Host && !h.Retiring {
			owned = true
		}
	}
	if !owned || state.Deleting {
		return fmt.Errorf("host is no longer accepting fleet requests")
	}
	var failures []error
	for _, v := range state.Servers {
		if v.Host != w.Host {
			continue
		}
		if err := w.syncServer(ctx, v); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", v.ID, err))
		}
	}
	return errors.Join(failures...)
}
func (w *Worker) syncServer(ctx context.Context, v Server) error {
	instance, err := w.Controller.Store.Get(v.ID)
	locallyReady := err == nil && instance.StartRequestID == v.RequestID && instance.Phase == "running" && instance.Desired == "running" && instance.Players != nil && instance.Error == "" && !instance.UpdatedAt.Before(w.StartedAt) && time.Since(instance.UpdatedAt) <= 45*time.Second
	if startupExpired(v, time.Now()) && !locallyReady {
		v.Error = StartupTimeoutMessage
		if err == nil {
			if err = w.Controller.SetDesired(v.ID, "stopped"); err != nil && !errors.Is(err, control.ErrShuttingDown) {
				return err
			}
		} else if !errors.Is(err, control.ErrNotFound) {
			return err
		}
		return w.Store.Publish(ctx, Report{ID: v.ID, Host: w.Host, RequestID: v.RequestID, Phase: "failed", Desired: "stopped", Error: v.Error, ObservedAt: time.Now().UTC()})
	}
	if errors.Is(err, control.ErrNotFound) {
		if v.Desired != "running" {
			return w.Store.Publish(ctx, Report{ID: v.ID, Host: w.Host, RequestID: v.RequestID, Phase: map[string]string{"stopped": "stopped", "deleted": "deleted"}[v.Desired], Desired: v.Desired, ObservedAt: time.Now().UTC()})
		}
		instance, err = w.Controller.CreateRequested(control.CreateRequest{ID: v.ID, Game: v.Game, Name: v.Name, Provider: "docker", IdleMinutes: w.Config.IdleMinutes, MaxMinutes: w.Config.MaxMinutes}, v.RequestID)
	}
	if errors.Is(err, control.ErrShuttingDown) {
		return nil
	}
	if err != nil {
		return w.failed(ctx, v, err)
	}
	if instance.StartRequestID != v.RequestID {
		if err = w.Controller.RequestDesired(v.ID, v.RequestID, v.Desired); err != nil {
			if errors.Is(err, control.ErrShuttingDown) {
				return nil
			}
			return w.failed(ctx, v, err)
		}
		instance, err = w.Controller.Store.Get(v.ID)
		if err != nil {
			return err
		}
	}
	r := Report{ID: v.ID, Host: w.Host, RequestID: v.RequestID, Phase: instance.Phase, Desired: instance.Desired, Address: instance.Connect(), Players: instance.Players, ObservedAt: instance.UpdatedAt}
	if instance.Error != "" && (v.ReadyAt != 0 || instance.Phase != "starting") {
		r.Error = "Game unavailable. Check the host logs."
	}
	if instance.Phase != "deleted" && instance.Desired == "running" && instance.UpdatedAt.Before(w.StartedAt) {
		r.Phase = "starting"
		r.Address = ""
		r.Players = nil
	}
	if r.Phase != "running" || r.Desired != "running" || r.Players == nil || r.Error != "" {
		r.Address = ""
	}
	return w.Store.Publish(ctx, r)
}

func (w *Worker) failed(ctx context.Context, server Server, cause error) error {
	publication := w.Store.Publish(ctx, Report{ID: server.ID, Host: w.Host, RequestID: server.RequestID, Phase: "failed", Desired: server.Desired, Error: "Server setup failed. Check the host logs.", ObservedAt: time.Now().UTC()})
	return errors.Join(cause, publication)
}
func (w *Worker) Pending(ctx context.Context) (bool, error) {
	state, err := w.Store.Load(ctx)
	if err != nil {
		return false, err
	}
	if state.Deleting {
		return false, nil
	}
	for _, v := range state.Servers {
		if v.Host != w.Host {
			continue
		}
		i, err := w.Controller.Store.Get(v.ID)
		if errors.Is(err, control.ErrNotFound) && v.Desired != "running" {
			continue
		}
		if err != nil {
			return true, err
		}
		if i.StartRequestID != v.RequestID {
			return true, nil
		}
	}
	return false, nil
}
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		op, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := w.Sync(op)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("fleet worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
