package lobby

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var web embed.FS

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
	Snapshot
	Pending      bool   `json:"pending"`
	RequestError string `json:"requestError,omitempty"`
}

func (s *Service) Reconcile(ctx context.Context) error {
	for _, id := range s.IDs {
		row, err := s.Table.Read(ctx, id)
		if err != nil {
			return err
		}
		if row.RequestID != "" && row.RequestID != row.CompletedID && !row.Pending(time.Now()) {
			if err := s.Table.Mark(ctx, id, row.RequestID, "completedID", "Start request expired; try again"); err != nil {
				return err
			}
		}
		if row.Pending(time.Now()) {
			return s.Host.Wake(ctx)
		}
	}
	return nil
}
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	files, _ := fs.Sub(web, "web")
	mux.Handle("GET /", http.FileServer(http.FS(files)))
	mux.HandleFunc("GET /api/games", func(w http.ResponseWriter, r *http.Request) {
		state, err := s.Host.State(r.Context())
		if err != nil {
			s.fail(w, err)
			return
		}
		views := []View{}
		for _, id := range s.IDs {
			row, err := s.Table.Read(r.Context(), id)
			if err != nil {
				s.fail(w, err)
				return
			}
			snapshot := row.Snapshot
			if snapshot.ID == "" {
				snapshot = Snapshot{ID: id, Name: id, Phase: "unavailable", Error: "Waiting for initial host setup"}
			}
			pending := row.Pending(time.Now())
			failure := row.Failure
			if row.RequestID != "" && row.RequestID != row.CompletedID && !pending {
				failure = "Start request expired; try again"
			}
			// A historical running snapshot never proves that a sleeping or unreachable game is ready.
			if snapshot.Phase != "unavailable" && (state != "running" || time.Since(snapshot.ObservedAt) > 45*time.Second) {
				snapshot.Address = ""
				snapshot.Players = nil
				snapshot.Phase = "unknown"
				if state == "stopped" {
					snapshot.Phase = "sleeping"
				}
			}
			views = append(views, View{Snapshot: snapshot, Pending: pending, RequestError: failure})
		}
		respond(w, 200, map[string]any{"host": state, "games": views})
	})
	mux.HandleFunc("POST /api/games/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		allowed := false
		for _, saved := range s.IDs {
			if saved == id {
				allowed = true
			}
		}
		if !allowed {
			respond(w, 404, map[string]string{"error": "Game not found"})
			return
		}
		row, err := s.Table.Read(r.Context(), id)
		if err != nil {
			s.fail(w, err)
			return
		}
		if row.Snapshot.ID == "" || row.Snapshot.Phase == "unavailable" {
			respond(w, 409, map[string]string{"error": "Saved game is not available; ask the host administrator"})
			return
		}
		err = s.Table.Request(r.Context(), id, time.Now().UTC())
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
			respond(w, 202, map[string]string{"message": "Start saved; the host wake will be retried"})
			return
		}
		respond(w, 202, map[string]string{"message": "Start requested"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			supplied := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
			expected, err := hex.DecodeString(s.TokenHash)
			if err != nil || len(expected) != 32 || subtle.ConstantTimeCompare(supplied[:], expected) != 1 {
				respond(w, 401, map[string]string{"error": "Invitation key required"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Service) fail(w http.ResponseWriter, err error) {
	log.Printf("lobby request: %v", err)
	respond(w, 503, map[string]string{"error": "Host status is unavailable; try again shortly"})
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
