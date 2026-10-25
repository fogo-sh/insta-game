package lobby

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/launchui"
)

type HostControl interface {
	State(context.Context) (string, error)
	Wake(context.Context) error
}
type Service struct {
	Table     Storage
	Host      HostControl
	IDs       []string
	TokenHash string
}

type View struct {
	HostID string `json:"hostId,omitempty"`
	Snapshot
	Action       string `json:"action,omitempty"`
	Pending      bool   `json:"pending"`
	RequestError string `json:"requestError,omitempty"`
}

func (s *Service) Reconcile(ctx context.Context) error {
	rows, err := s.Table.List(ctx)
	if err != nil {
		return err
	}
	for _, id := range managedIDs(rows, s.IDs) {
		row, err := s.Table.Read(ctx, id)
		if err != nil {
			return err
		}
		if row.RequestID != "" && row.RequestID != row.CompletedID && !row.Pending(time.Now()) {
			if err := s.Table.Mark(ctx, id, row.RequestID, "completedID", "Request expired; try again"); err != nil {
				return err
			}
		}
		if row.Pending(time.Now()) {
			return s.Host.Wake(ctx)
		}
	}
	return nil
}
func (s *Service) Handler() http.Handler { return protect(s.routes(), s.TokenHash) }

func (s *Service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", launchui.Handler())
	mux.HandleFunc("GET /api/games", func(w http.ResponseWriter, r *http.Request) {
		result, err := s.view(r.Context())
		if err != nil {
			s.fail(w, err)
			return
		}
		respond(w, 200, result)
	})
	mux.HandleFunc("POST /api/games/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		action := r.PathValue("action")
		if action != "start" && action != "stop" && action != "remove" {
			respond(w, 404, map[string]string{"error": "Unknown action"})
			return
		}
		row, err := s.Table.Read(r.Context(), id)
		if err != nil {
			s.fail(w, err)
			return
		}
		allowed := slices.Contains(s.IDs, id) || slices.Contains(s.IDs, row.TemplateID)
		if !allowed {
			respond(w, 404, map[string]string{"error": "Game not found"})
			return
		}
		if slices.Contains(s.IDs, id) && action == "remove" {
			respond(w, 409, map[string]string{"error": "Game presets can only be removed by the administrator"})
			return
		}
		if row.Snapshot.ID == "" || row.Snapshot.Phase == "unavailable" || (row.Snapshot.Desired == "deleted" && action != "remove") {
			respond(w, 409, map[string]string{"error": "This server is unavailable."})
			return
		}
		err = s.Table.Queue(r.Context(), id, action, time.Now().UTC())
		if errors.Is(err, ErrBusy) {
			respond(w, 409, map[string]string{"error": err.Error()})
			return
		}
		if err != nil {
			s.fail(w, err)
			return
		}
		if err := s.Host.Wake(r.Context()); err != nil {
			log.Printf("wake queued; initial attempt failed: %v", err)
			respond(w, 202, map[string]string{"message": "Queued. Host startup will retry."})
			return
		}
		respond(w, 202, map[string]string{"message": "Queued."})
	})
	mux.HandleFunc("POST /api/games", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ID       string `json:"id"`
			Template string `json:"template"`
			Name     string `json:"name"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			respond(w, 400, map[string]string{"error": "Invalid server request"})
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			respond(w, 400, map[string]string{"error": "Invalid server request"})
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if !instanceID.MatchString(input.ID) || !strings.HasPrefix(input.ID, "server-") || len(input.ID) != 39 || len(input.Name) > 80 {
			respond(w, 400, map[string]string{"error": "Invalid server ID or name"})
			return
		}
		if !slices.Contains(s.IDs, input.Template) {
			respond(w, 404, map[string]string{"error": "Game not found"})
			return
		}
		preset, err := s.Table.Read(r.Context(), input.Template)
		if err != nil {
			s.fail(w, err)
			return
		}
		if preset.Snapshot.ID == "" || preset.Snapshot.Phase == "unavailable" {
			respond(w, 409, map[string]string{"error": "This game is unavailable."})
			return
		}
		if input.Name == "" {
			input.Name = preset.Snapshot.Name
			if input.Name == "" {
				input.Name = preset.Snapshot.Game
			}
		}
		now := time.Now().UTC()
		snapshot := Snapshot{ID: input.ID, Name: input.Name, Game: preset.Snapshot.Game, Phase: "queued", Desired: "running", ObservedAt: now}
		err = s.Table.Create(r.Context(), input.Template, snapshot, now)
		if errors.Is(err, ErrBusy) {
			existing, e := s.Table.Read(r.Context(), input.ID)
			if e != nil {
				s.fail(w, e)
				return
			}
			if existing.TemplateID != input.Template || existing.Snapshot.Name != input.Name {
				respond(w, 409, map[string]string{"error": "Server ID already exists"})
				return
			}
		} else if err != nil {
			s.fail(w, err)
			return
		}
		if err := s.Host.Wake(r.Context()); err != nil {
			log.Printf("server saved; wake will retry: %v", err)
		}
		respond(w, 202, map[string]string{"id": input.ID, "message": "Starting server."})
	})
	return mux
}

type listing struct {
	Host    string              `json:"host"`
	Games   []View              `json:"games"`
	Presets []map[string]string `json:"presets"`
}

func (s *Service) view(ctx context.Context) (listing, error) {
	state, err := s.Host.State(ctx)
	if err != nil {
		return listing{}, err
	}
	rows, err := s.Table.List(ctx)
	if err != nil {
		return listing{}, err
	}
	byID := map[string]Row{}
	for _, row := range rows {
		byID[row.Snapshot.ID] = row
	}
	presets := []map[string]string{}
	for _, id := range s.IDs {
		row := byID[id]
		if row.Snapshot.ID != "" && row.Snapshot.Phase != "unavailable" {
			presets = append(presets, map[string]string{"id": id, "name": row.Snapshot.Name, "game": row.Snapshot.Game})
		}
	}
	views := []View{}
	for _, id := range managedIDs(rows, s.IDs) {
		row := byID[id]
		snapshot := row.Snapshot
		if snapshot.ID == "" {
			snapshot = Snapshot{ID: id, Name: id, Phase: "unavailable", Error: "Waiting for initial host setup"}
		}
		pending := row.Pending(time.Now())
		failure := row.Failure
		if row.RequestID != "" && row.RequestID != row.CompletedID && !pending {
			failure = "Request expired; try again"
		}
		// A historical running snapshot never proves that a sleeping or unreachable game is ready.
		if snapshot.Phase != "unavailable" && snapshot.Phase != "deleted" && (state != "running" || time.Since(snapshot.ObservedAt) > 45*time.Second) {
			snapshot.Address = ""
			snapshot.Players = nil
			snapshot.Phase = "unknown"
			if state == "stopped" {
				snapshot.Phase = "sleeping"
			}
		}
		views = append(views, View{Snapshot: snapshot, Action: row.Action, Pending: pending, RequestError: failure})
	}
	return listing{Host: state, Games: views, Presets: presets}, nil

}

func protect(next http.Handler, hash string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			supplied := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
			expected, err := hex.DecodeString(hash)
			if err != nil || len(expected) != 32 || subtle.ConstantTimeCompare(supplied[:], expected) != 1 {
				respond(w, 401, map[string]string{"error": "Invitation key required"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) fail(w http.ResponseWriter, err error) {
	log.Printf("lobby request: %v", err)
	respond(w, 503, map[string]string{"error": "Status unavailable. Try again shortly."})
}
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode response: %v", err)
	}
}
func ValidateHash(value string) error {
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("lobby token hash must be a SHA-256 hex digest")
	}
	return nil
}
