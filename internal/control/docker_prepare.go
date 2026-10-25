package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type imagePreparation struct {
	done    chan struct{}
	started time.Time
	assets  string
	err     error
}
type dockerPreparation struct {
	mu      sync.Mutex
	jobs    map[string]*imagePreparation
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
	slot    chan struct{}
}

// Close cancels downloads before the controller releases its local database.
func (d *Docker) Close() { d.preparation.cancel(); d.preparation.workers.Wait() }
func (d *Docker) Prepare(ctx context.Context, i Instance) (bool, error) {
	p := d.preparation
	p.mu.Lock()
	defer p.mu.Unlock()
	job := p.jobs[d.preparationKey(i.Game)]
	if job != nil {
		select {
		case <-job.done:
			if job.err == nil {
				return true, nil
			}
			if !i.StartedAt.After(job.started) {
				return false, job.err
			}
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			return false, nil
		}
	}
	// Avoid background work when this game has no external assets and its image is cached.
	if job == nil && d.PrepareAssets == nil {
		err := d.request(ctx, http.MethodGet, "/images/"+url.PathEscape(i.Game.Image)+"/json", nil, nil)
		if err == nil {
			return true, nil
		}
		if e, ok := err.(*dockerError); !ok || e.status != 404 {
			return false, err
		}
	}
	job = &imagePreparation{done: make(chan struct{}), started: time.Now()}
	p.jobs[d.preparationKey(i.Game)] = job
	p.workers.Add(1)
	go func() {
		defer p.workers.Done()
		defer close(job.done)
		ctx, cancel := context.WithTimeout(p.ctx, 20*time.Minute)
		defer cancel()
		select {
		case p.slot <- struct{}{}:
			defer func() { <-p.slot }()
		case <-ctx.Done():
			job.err = ctx.Err()
			return
		}
		if err := d.request(ctx, http.MethodGet, "/images/"+url.PathEscape(i.Game.Image)+"/json", nil, nil); err != nil {
			if e, ok := err.(*dockerError); !ok || e.status != 404 {
				job.err = err
				return
			}
			if job.err = d.pullImage(ctx, i.Game.Image); job.err != nil {
				return
			}
		}
		if d.PrepareAssets != nil {
			job.assets, job.err = d.PrepareAssets(ctx, i.Game)
		}
	}()
	return false, nil
}
func (d *Docker) pullImage(ctx context.Context, image string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return err
	}
	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("image pull failed (HTTP %d)", res.StatusCode)
	}
	decoder := json.NewDecoder(res.Body)
	for {
		var event struct {
			Error string `json:"error"`
		}
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if event.Error != "" {
			return fmt.Errorf("image pull: %s", event.Error)
		}
	}
	return ctx.Err()
}
func (d *Docker) preparationKey(game Game) string {
	return game.ID + "/" + game.Image + "/" + d.AssetVersion
}
func (d *Docker) assetDirectory(game Game) string {
	p := d.preparation
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if j := p.jobs[d.preparationKey(game)]; j != nil {
		select {
		case <-j.done:
			return j.assets
		default:
		}
	}
	return ""
}
