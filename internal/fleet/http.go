package fleet

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/launchui"
)

var serverID = regexp.MustCompile(`^server-[a-f0-9]{32}$`)

type Page struct {
	Scheduler *Scheduler
	TokenHash string
}

func (p *Page) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", launchui.Handler())
	mux.HandleFunc("GET /api/games", p.list)
	mux.HandleFunc("POST /api/games", p.create)
	mux.HandleFunc("POST /api/hosts/{host}/games/{id}/{action}", p.action)
	mux.HandleFunc("POST /api/games/{id}/{action}", p.action)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			got := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
			expected, e := hex.DecodeString(p.TokenHash)
			if e != nil || len(expected) != 32 || subtle.ConstantTimeCompare(expected, got[:]) != 1 {
				reply(w, 401, map[string]string{"error": "Invalid key"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (p *Page) fail(w http.ResponseWriter, err error) {
	log.Printf("fleet: %v", err)
	reply(w, 503, map[string]string{"error": "Couldn't load the fleet. Try again shortly."})
}
func (p *Page) create(w http.ResponseWriter, r *http.Request) {
	var input struct{ ID, Game, Name string }
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF || !serverID.MatchString(input.ID) {
		reply(w, 400, map[string]string{"error": "Invalid server request"})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		for _, g := range p.Scheduler.Config.Games {
			if g.ID == input.Game {
				input.Name = g.Name
			}
		}
	}
	if input.Name == "" || len(input.Name) > 80 || strings.ContainsAny(input.Name, "\x00\n\r") {
		reply(w, 400, map[string]string{"error": "Choose a game and server name"})
		return
	}
	server, err := p.Scheduler.Create(r.Context(), input.ID, input.Game, input.Name)
	if err != nil {
		status := 409
		if errors.Is(err, ErrCapacity) {
			status = 429
		}
		reply(w, status, map[string]string{"error": err.Error()})
		return
	}
	if err = p.Scheduler.Reconcile(r.Context()); err != nil {
		log.Printf("fleet request queued: %v", err)
	}
	reply(w, 202, map[string]string{"id": server.ID, "message": "Starting server."})
}
func (p *Page) action(w http.ResponseWriter, r *http.Request) {
	if host := r.PathValue("host"); host != "" {
		state, err := p.Scheduler.Store.Load(r.Context())
		if err != nil {
			p.fail(w, err)
			return
		}
		found := false
		for _, v := range state.Servers {
			if v.ID == r.PathValue("id") && v.Host == host {
				found = true
			}
		}
		if !found {
			reply(w, 404, map[string]string{"error": "Server not found"})
			return
		}
	}
	if err := p.Scheduler.Action(r.Context(), r.PathValue("id"), r.PathValue("action")); err != nil {
		reply(w, 409, map[string]string{"error": err.Error()})
		return
	}
	if err := p.Scheduler.Reconcile(r.Context()); err != nil {
		log.Printf("fleet action queued: %v", err)
	}
	reply(w, 202, map[string]string{"message": "Queued."})
}
func (p *Page) list(w http.ResponseWriter, r *http.Request) {
	state, err := p.Scheduler.Store.Load(r.Context())
	if err != nil {
		p.fail(w, err)
		return
	}
	reports, err := p.Scheduler.Store.Reports(r.Context())
	if err != nil {
		p.fail(w, err)
		return
	}
	byID := map[string]Report{}
	for _, v := range reports {
		byID[v.ID] = v
	}
	hosts := []map[string]any{}
	hostStates := map[string]string{}
	for _, h := range state.Hosts {
		m, e := p.Scheduler.Machines.Inspect(r.Context(), h)
		hs := m.State
		message := h.Error
		if e != nil {
			hs = "unknown"
			message = "Status unavailable"
		}
		if h.Retiring {
			hs = "stopping"
		}
		hostStates[h.ID] = hs
		hosts = append(hosts, map[string]any{"id": h.ID, "name": h.ID, "state": hs, "error": message, "capacity": h.Capacity, "reserved": used(state, h.ID)})
	}
	choices := []map[string]string{}
	names := map[string]string{}
	for _, g := range p.Scheduler.Config.Games {
		names[g.ID] = g.Name
		choices = append(choices, map[string]string{"id": g.ID, "name": g.Name})
	}
	games := []map[string]any{}
	for _, v := range state.Servers {
		report := byID[v.ID]
		phase := "queued"
		address := ""
		var players *int
		failure := ""
		pending := true
		if report.Host == v.Host && report.RequestID == v.RequestID {
			phase = report.Phase
			address = report.Address
			players = report.Players
			failure = report.Error
			pending = !requestComplete(v, report) && phase != "failed"
		}
		if hostStates[v.Host] == "stopped" && v.Desired == "stopped" {
			phase = "stopped"
			pending = false
			address = ""
			players = nil
		}
		if pending && time.Now().Unix()-v.RequestedAt > 1200 {
			failure = "Startup is taking too long. Stop the server and check the host logs."
		}
		if hostStates[v.Host] != "running" || report.ObservedAt.Before(time.Now().Add(-45*time.Second)) || report.Desired != "running" || failure != "" {
			address = ""
			players = nil
			if phase == "running" {
				phase = "unknown"
			}
		}
		if v.Error != "" {
			failure = v.Error
			phase = "failed"
			pending = false
			address = ""
			players = nil
		}
		action := map[string]string{"running": "start", "stopped": "stop", "deleted": "remove"}[v.Desired]
		games = append(games, map[string]any{"id": v.ID, "name": v.Name, "game": names[v.Game], "hostId": v.Host, "phase": phase, "desired": v.Desired, "pending": pending, "action": action, "address": address, "players": players, "error": failure})
	}
	reply(w, 200, map[string]any{"hosts": hosts, "games": games, "choices": choices, "maxHosts": p.Scheduler.Config.MaxHosts, "expiresAt": p.Scheduler.Config.Deadline})
}
