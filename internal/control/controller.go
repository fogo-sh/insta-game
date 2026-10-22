package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/fogo-sh/insta-game/internal/agent"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Controller struct {
	Store           *Store
	Games           []Game
	Providers       map[string]Provider
	DefaultProvider string
	HTTP            *http.Client
}

func NewController(store *Store, games []Game, providers map[string]Provider, defaultProvider string) *Controller {
	return &Controller{Store: store, Games: games, Providers: providers, DefaultProvider: defaultProvider, HTTP: &http.Client{Timeout: 5 * time.Second}}
}
func NewToken() string {
	var data [24]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}

func (c *Controller) Create(req CreateRequest) (Instance, error) {
	if req.ID == "" {
		req.ID = NewToken()[:20]
	}
	if !validID.MatchString(req.ID) {
		return Instance{}, fmt.Errorf("id must contain 1–64 lowercase letters, numbers or hyphens")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 80 {
		return Instance{}, fmt.Errorf("name must contain 1–80 characters")
	}
	if req.IdleMinutes < 0 || req.IdleMinutes > 10080 || req.MaxMinutes < 0 || req.MaxMinutes > 10080 {
		return Instance{}, fmt.Errorf("timeouts must be between 0 and 10080 minutes")
	}
	var game Game
	for _, g := range c.Games {
		if g.ID == req.Game {
			game = g
			break
		}
	}
	if game.ID == "" {
		return Instance{}, fmt.Errorf("unknown game %q", req.Game)
	}
	if req.Provider == "" {
		req.Provider = c.DefaultProvider
	}
	if _, ok := c.Providers[req.Provider]; !ok {
		return Instance{}, fmt.Errorf("provider %q is not configured", req.Provider)
	}
	config := game.DefaultConfig
	if req.Config != nil {
		config = *req.Config
	}
	if len(config) > 60000 || strings.ContainsRune(config, 0) {
		return Instance{}, fmt.Errorf("config must be at most 60000 bytes without NUL characters")
	}

	now := time.Now().UTC()
	instance := Instance{Generation: 1, ID: req.ID, Name: req.Name, Game: game, Provider: req.Provider, Desired: "running", Phase: "queued", Config: config, IdleMinutes: req.IdleMinutes, MaxMinutes: req.MaxMinutes, CreatedAt: now, StartedAt: now, UpdatedAt: now, AgentToken: NewToken(), RconPassword: NewToken()}
	return c.Store.Create(instance)
}

func (c *Controller) SetDesired(id, desired string) error {
	if desired != "running" && desired != "stopped" && desired != "deleted" {
		return fmt.Errorf("invalid desired state")
	}
	return c.Store.Update(id, func(i *Instance) error {
		if i.Desired == "deleted" {
			return fmt.Errorf("instance has been deleted")
		}
		if i.Desired == desired {
			return nil
		}
		i.Generation++
		i.Desired = desired
		i.Error = ""
		i.StopReason = ""
		i.EmptySince = time.Time{}
		if desired == "running" {
			i.StartedAt = time.Now().UTC()
			i.Phase = "queued"
		}
		return nil
	})
}
func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		c.Reconcile(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (c *Controller) Reconcile(ctx context.Context) {
	instances, err := c.Store.List()
	if err != nil {
		log.Printf("list instances: %v", err)
		return
	}
	var workers sync.WaitGroup
	slots := make(chan struct{}, 4)
	for _, instance := range instances {
		if instance.Phase == "deleted" {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			workers.Wait()
			return
		}
		workers.Add(1)
		go func() { defer workers.Done(); defer func() { <-slots }(); c.reconcileInstance(ctx, instance) }()
	}
	workers.Wait()
}
func (c *Controller) reconcileInstance(ctx context.Context, instance Instance) {
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	observation, phase, opErr := c.reconcile(opCtx, instance)
	cancel()
	now := time.Now().UTC()
	if saveErr := c.Store.Update(instance.ID, func(current *Instance) error {
		// A user may have changed intent while an infrastructure request was in flight.
		if current.Generation != instance.Generation {
			return nil
		}
		current.UpdatedAt = now
		if opErr != nil {
			current.Error = opErr.Error()
			current.Players = nil
			current.EmptySince = time.Time{}
			return nil
		}
		current.Phase = phase
		current.Endpoints = observation.Endpoints
		current.Error = observation.Failure
		current.Players = nil
		if phase != "running" {
			current.EmptySince = time.Time{}
		}
		return nil
	}); saveErr != nil {
		log.Printf("save observation for %s: %v", instance.ID, saveErr)
	}
	if opErr == nil && phase == "running" {
		c.observeAgent(ctx, instance, observation)
	}
}

func (c *Controller) reconcile(ctx context.Context, i Instance) (Observation, string, error) {
	provider, err := c.provider(i)
	if err != nil {
		return Observation{}, i.Phase, err
	}
	if i.Desired == "deleted" {
		err := provider.Delete(ctx, i)
		return Observation{}, "deleted", err
	}
	if i.Desired == "running" && i.MaxMinutes > 0 && time.Since(i.StartedAt) >= time.Duration(i.MaxMinutes)*time.Minute {
		err := c.Store.Update(i.ID, func(current *Instance) error {
			if current.Desired == "running" && current.Generation == i.Generation {
				current.Desired = "stopped"
				current.StopReason = "Maximum runtime reached"
			}
			return nil
		})
		return Observation{}, "stopping", err
	}
	observation, err := provider.Inspect(ctx, i)
	if err != nil {
		return observation, i.Phase, err
	}
	if i.Desired == "stopped" {
		if observation.Running {
			err = provider.Stop(ctx, i)
			return observation, "stopping", err
		}
		return observation, "stopped", nil
	}

	if !observation.Exists {
		observation, err = provider.Ensure(ctx, i)
		if err != nil {
			return observation, "queued", err
		}
	}
	if !observation.Running {
		// A crashed game needs an explicit stop/start, not an endless restart loop.
		if i.Phase != "queued" && observation.Started {
			observation.Failure = "Game exited; stop and start to retry"
			return observation, "failed", nil
		}
		if err = provider.Start(ctx, i); err != nil {
			return observation, "starting", err
		}
		return observation, "starting", nil
	}
	return observation, "running", nil
}

type AgentStatus struct {
	Running bool   `json:"running"`
	Ready   bool   `json:"ready"`
	Players int    `json:"players"`
	Map     string `json:"map"`
	Error   string `json:"error"`
}

func (c *Controller) observeAgent(ctx context.Context, i Instance, o Observation) {
	var status AgentStatus
	err := c.agentRequest(ctx, i, o.AgentURL, "GET", "/status", nil, &status)
	c.Store.Update(i.ID, func(current *Instance) error {
		if current.Desired != "running" || current.Generation != i.Generation {
			return nil
		}
		if err != nil {
			current.Error = "Agent unavailable: " + err.Error()
			current.Players = nil
			current.EmptySince = time.Time{}
			current.Phase = "starting"
			return nil
		}
		if !status.Running {
			current.Phase = "failed"
			current.Error = "Game process exited"
			if status.Error != "" {
				current.Error = status.Error
			}
			return nil
		}
		if !status.Ready {
			current.Phase = "starting"
			current.EmptySince = time.Time{}
			return nil
		}
		current.Phase = "running"
		current.Players = &status.Players
		current.Map = status.Map
		current.Error = ""
		now := time.Now().UTC()
		if status.Players > 0 {
			current.EmptySince = time.Time{}
			return nil
		}
		if current.EmptySince.IsZero() {
			current.EmptySince = now
		}
		if current.IdleMinutes > 0 && now.Sub(current.EmptySince) >= time.Duration(current.IdleMinutes)*time.Minute {
			current.Desired = "stopped"
			current.StopReason = "Server empty for idle timeout"
		}
		return nil
	})
}
func (c *Controller) agentRequest(ctx context.Context, i Instance, base, method, path string, body io.Reader, result any) error {
	if base == "" {
		return fmt.Errorf("agent address is unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+i.AgentToken)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if strings.HasPrefix(base, "https://") {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = agent.ClientTLS(i.AgentToken)
		defer transport.CloseIdleConnections()
		secure := *c.HTTP
		secure.Transport = transport
		client = &secure
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("agent HTTP %d: %s", res.StatusCode, data)
	}
	if result == nil {
		_, err = io.Copy(io.Discard, res.Body)
		return err
	}
	return json.NewDecoder(res.Body).Decode(result)
}

func (c *Controller) provider(i Instance) (Provider, error) {
	p, ok := c.Providers[i.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %q is not configured", i.Provider)
	}
	return p, nil
}
