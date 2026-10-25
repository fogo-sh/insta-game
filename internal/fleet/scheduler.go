package fleet

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

var ErrCapacity = errors.New("Host limit reached. Remove a saved server to free space, or ask the administrator to raise the limit.")
var ErrUnavailable = errors.New("This host is unavailable. Its saved servers have not been moved.")

const StartupTimeout = 20 * time.Minute
const StartupTimeoutMessage = "Server startup timed out. Start again to retry, or remove the saved server."
const RequestTimeoutMessage = "The host did not finish this request. Retry the action; saved data is retained."

type Machine struct{ ID, State string }
type Machines interface {
	Inspect(context.Context, Host) (Machine, error)
	Wake(context.Context, Host) (string, error)
	Stop(context.Context, Host) error
	Delete(context.Context, Host) error
}

type Scheduler struct {
	Store    Store
	Machines Machines
	Config   Config
}

func fits(capacity, used, required Resources) bool {
	return used.CPU+required.CPU <= capacity.CPU && used.MemoryMiB+required.MemoryMiB <= capacity.MemoryMiB && used.DiskMiB+required.DiskMiB <= capacity.DiskMiB
}
func used(s State, host string) Resources {
	var r Resources
	for _, server := range s.Servers {
		if server.Host == host {
			r.CPU += server.Resources.CPU
			r.MemoryMiB += server.Resources.MemoryMiB
			r.DiskMiB += server.Resources.DiskMiB
		}
	}
	return r
}
func (s *Scheduler) Create(ctx context.Context, id, game, name string) (Server, error) {
	var choice Game
	for _, g := range s.Config.Games {
		if g.ID == game {
			choice = g
			break
		}
	}
	if choice.ID == "" {
		return Server{}, fmt.Errorf("Unknown game")
	}
	for attempt := 0; attempt < 8; attempt++ {
		state, err := s.Store.Load(ctx)
		if err != nil {
			return Server{}, err
		}
		if state.Deleting || s.expired(time.Now()) {
			return Server{}, fmt.Errorf("This deployment has ended")
		}
		for _, v := range state.Servers {
			if v.ID == id {
				if v.Game != game || v.Name != name || v.Desired == "deleted" {
					return Server{}, fmt.Errorf("Server ID is already in use")
				}
				return v, nil
			}
		}
		removed, err := s.Store.Tombstoned(ctx, id)
		if err != nil {
			return Server{}, err
		}
		if removed {
			return Server{}, fmt.Errorf("This server was removed")
		}
		if len(state.Servers) >= s.Config.MaxServers {
			return Server{}, ErrCapacity
		}
		host, err := s.choose(ctx, state, choice.Resources)
		if err != nil {
			return Server{}, err
		}
		if host.ID == "" {
			if len(state.Hosts) >= s.Config.MaxHosts {
				return Server{}, ErrCapacity
			}
			host = Host{ID: "host-" + control.NewToken()[:16], Release: s.Config.Release, Workload: s.Config.Workload, LaunchToken: control.NewToken(), LaunchTemplate: s.Config.LaunchTemplate, LaunchVersion: s.Config.LaunchVersion, Capacity: s.Config.Capacity, CreatedAt: time.Now().Unix()}
			state.Hosts = append(state.Hosts, host)
		}
		server := Server{ID: id, Game: game, Name: name, Host: host.ID, Desired: "running", RequestID: control.NewToken(), RequestedAt: time.Now().Unix(), Resources: choice.Resources}
		state.Servers = append(state.Servers, server)
		if err = s.Store.Save(ctx, state, state.Revision); errors.Is(err, ErrConflict) {
			continue
		} else if err != nil {
			return Server{}, err
		}
		return server, nil
	}
	return Server{}, ErrConflict
}
func (s *Scheduler) choose(ctx context.Context, state State, required Resources) (Host, error) {
	type candidate struct {
		host            Host
		rank, remaining int
	}
	var candidates []candidate
	for _, h := range state.Hosts {
		load := used(state, h.ID)
		if h.Retiring || (h.Release != s.Config.Release && (h.Workload == "" || h.Workload != s.Config.Workload)) || !fits(h.Capacity, load, required) {
			continue
		}
		m, err := s.Machines.Inspect(ctx, h)
		if err != nil {
			continue
		}
		rank := 0
		switch m.State {
		case "running":
		case "pending", "new":
			rank = 1
		case "stopped", "stopping":
			rank = 2
		default:
			continue
		}
		candidates = append(candidates, candidate{h, rank, h.Capacity.MemoryMiB - load.MemoryMiB - required.MemoryMiB})
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.remaining != b.remaining {
			return a.remaining < b.remaining
		}
		return a.host.ID < b.host.ID
	})
	if len(candidates) > 0 {
		return candidates[0].host, nil
	}
	return Host{}, nil
}
func (s *Scheduler) Action(ctx context.Context, id, action string) error {
	desired := map[string]string{"start": "running", "stop": "stopped", "remove": "deleted"}[action]
	if desired == "" {
		return fmt.Errorf("Unknown action")
	}
	for attempt := 0; attempt < 8; attempt++ {
		state, err := s.Store.Load(ctx)
		if err != nil {
			return err
		}
		if state.Deleting || s.expired(time.Now()) {
			return fmt.Errorf("This deployment has ended")
		}
		found := false
		for i, v := range state.Servers {
			if v.ID != id {
				continue
			}
			found = true
			if v.Desired == "deleted" && desired != "deleted" {
				return fmt.Errorf("This server is being removed")
			}
			if v.Desired == desired {
				// An idle stop is acknowledged locally before the scheduled reconciler
				// copies it into the registry. A start during that interval is new intent.
				reports, err := s.Store.Reports(ctx)
				if err != nil {
					return err
				}
				restart := requestExpired(v, time.Now()) && (v.Desired == "running" || v.Error != "")
				for _, r := range reports {
					if r.ID == v.ID && r.Host == v.Host && r.RequestID == v.RequestID {
						restart = restart || desired == "running" && (r.Desired == "stopped" || r.Phase == "failed")
					}
				}
				if !restart {
					return nil
				}
			}
			state.Servers[i].Desired = desired
			state.Servers[i].RequestID = control.NewToken()
			state.Servers[i].RequestedAt = time.Now().Unix()
			state.Servers[i].ReadyAt = 0
			state.Servers[i].Error = ""
		}
		if !found {
			return fmt.Errorf("Server not found")
		}
		if err = s.Store.Save(ctx, state, state.Revision); errors.Is(err, ErrConflict) {
			continue
		} else {
			return err
		}
	}
	return ErrConflict
}
func (s *Scheduler) expired(now time.Time) bool {
	return s.Config.Deadline > 0 && now.Unix() >= s.Config.Deadline
}

func startupExpired(server Server, now time.Time) bool {
	return server.Desired == "running" && requestExpired(server, now)
}

func requestExpired(server Server, now time.Time) bool {
	return server.ReadyAt == 0 && now.Unix()-server.RequestedAt >= int64(StartupTimeout/time.Second)
}

func requestComplete(server Server, report Report) bool {
	return report.Host == server.Host && report.RequestID == server.RequestID && report.Error == "" && report.Desired == server.Desired && report.Phase == server.Desired
}

// Reconcile persists launch identity before touching EC2. Browser requests only record intent.
func (s *Scheduler) Reconcile(ctx context.Context) error {
	for attempt := 0; attempt < 8; attempt++ {
		err := s.reconcile(ctx)
		if !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return ErrConflict
}

func (s *Scheduler) reconcile(ctx context.Context) error {
	state, err := s.Store.Load(ctx)
	if err != nil {
		return err
	}
	reports, err := s.Store.Reports(ctx)
	if err != nil {
		return err
	}
	byID := map[string]Report{}
	for _, r := range reports {
		byID[r.ID] = r
	}
	changed := false
	if s.expired(time.Now()) && !state.Deleting {
		state.Deleting = true
		changed = true
	}
	for i, v := range state.Servers {
		r := byID[v.ID]
		if v.ReadyAt == 0 && v.Desired == "running" && r.Host == v.Host && r.RequestID == v.RequestID && r.Error == "" && r.Desired == "running" && r.Phase == "running" && r.Players != nil && time.Since(r.ObservedAt) <= 45*time.Second {
			state.Servers[i].ReadyAt = r.ObservedAt.Unix()
			v.ReadyAt = r.ObservedAt.Unix()
			changed = true
		}
		failure := ""
		if startupExpired(v, time.Now()) {
			failure = StartupTimeoutMessage
		} else if requestExpired(v, time.Now()) && !requestComplete(v, r) {
			failure = RequestTimeoutMessage
		}
		if !state.Deleting && v.Error != failure {
			state.Servers[i].Error = failure
			changed = true
		}
		if r.Host != v.Host || r.RequestID != v.RequestID || r.Error != "" {
			continue
		}
		if !state.Deleting && v.Desired == "running" && r.Desired == "stopped" && r.Phase == "stopped" {
			state.Servers[i].Desired = "stopped"
			changed = true
		}
		if !state.Deleting && v.Desired == "deleted" && r.Desired == "deleted" && r.Phase == "deleted" {
			state.Servers = append(state.Servers[:i], state.Servers[i+1:]...)
			return s.Store.Forget(ctx, state, state.Revision, v.ID)
		}
	}
	// Retirement closes admission in the same CAS that checked deletion intent.
	// A concurrent create must win that CAS or observe Retiring and choose elsewhere.
	for i, h := range state.Hosts {
		retained := false
		for _, v := range state.Servers {
			retained = retained || v.Host == h.ID && v.Desired != "deleted"
		}
		if !h.Retiring && (state.Deleting || !retained) {
			state.Hosts[i].Retiring = true
			changed = true
		}
	}
	if changed {
		if err := s.Store.Save(ctx, state, state.Revision); err != nil {
			return err
		}
		state.Revision++
	}
	var failures []error
	for _, h := range state.Hosts {
		needed := false
		hasExpired := false
		for _, v := range state.Servers {
			if v.Host == h.ID {
				r := byID[v.ID]
				complete := requestComplete(v, r)
				if startupExpired(v, time.Now()) || requestExpired(v, time.Now()) && !complete {
					hasExpired = true
					continue
				}
				if v.Desired == "running" || !complete {
					needed = true
				}
			}
		}
		if h.Retiring {
			if !state.Deleting {
				for _, v := range state.Servers {
					if v.Host == h.ID && v.Desired != "deleted" {
						return fmt.Errorf("refusing to retire host %s with saved servers", h.ID)
					}
				}
			}
			if err = s.Machines.Delete(ctx, h); err != nil {
				failures = append(failures, err)
				continue
			}
			// The provider returns only after termination is observed, before releasing this slot.
			if e := s.forgetHost(ctx, h.ID); e != nil {
				return e
			}
			continue
		}
		if needed {
			id, e := s.Machines.Wake(ctx, h)
			if e != nil {
				failures = append(failures, fmt.Errorf("%s: %w", h.ID, e))
				if h.Error != "Host startup failed; retrying" {
					if saveErr := s.updateHost(ctx, h.ID, func(h *Host) { h.Error = "Host startup failed; retrying" }); saveErr != nil {
						failures = append(failures, saveErr)
					}
				}
				continue
			}
			if id != h.InstanceID || h.Error != "" {
				if e = s.updateHost(ctx, h.ID, func(h *Host) { h.InstanceID = id; h.Error = "" }); e != nil {
					return e
				}
			}
		} else if hasExpired {
			latest, e := s.Store.Load(ctx)
			if e != nil {
				return e
			}
			if latest.Revision != state.Revision {
				return ErrConflict
			}
			if e := s.Machines.Stop(ctx, h); e != nil {
				failures = append(failures, fmt.Errorf("stop expired startup host %s: %w", h.ID, e))
			}
		}
	}
	return errors.Join(failures...)
}

// Called only after the provider observed termination. Each removed server must
// have explicit deletion intent, unless the whole deployment is being destroyed.
func (s *Scheduler) forgetHost(ctx context.Context, id string) error {
	for {
		state, err := s.Store.Load(ctx)
		if err != nil {
			return err
		}
		hostIndex := -1
		for i, h := range state.Hosts {
			if h.ID == id {
				if !h.Retiring {
					return fmt.Errorf("host %s is not retiring", id)
				}
				hostIndex = i
			}
		}
		if hostIndex < 0 {
			return nil
		}
		serverIndex := -1
		for i, v := range state.Servers {
			if v.Host == id {
				serverIndex = i
				break
			}
		}
		if serverIndex >= 0 {
			if !state.Deleting && state.Servers[serverIndex].Desired != "deleted" {
				return fmt.Errorf("host %s still has saved servers", id)
			}
			serverID := state.Servers[serverIndex].ID
			state.Servers = append(state.Servers[:serverIndex], state.Servers[serverIndex+1:]...)
			if err := s.Store.Forget(ctx, state, state.Revision, serverID); err != nil {
				return err
			}
			continue
		}
		state.Hosts = append(state.Hosts[:hostIndex], state.Hosts[hostIndex+1:]...)
		return s.Store.Save(ctx, state, state.Revision)
	}
}
func (s *Scheduler) updateHost(ctx context.Context, id string, fn func(*Host)) error {
	for attempt := 0; attempt < 8; attempt++ {
		state, err := s.Store.Load(ctx)
		if err != nil {
			return err
		}
		for i := range state.Hosts {
			if state.Hosts[i].ID == id {
				fn(&state.Hosts[i])
				if err = s.Store.Save(ctx, state, state.Revision); errors.Is(err, ErrConflict) {
					break
				}
				return err
			}
		}
		if !errors.Is(err, ErrConflict) {
			return nil
		}
	}
	return ErrConflict
}
