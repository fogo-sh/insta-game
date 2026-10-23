package control

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/securehttp"
)

//go:embed web/*
var web embed.FS

func (c *Controller) Handler(token string) http.Handler {
	mux := http.NewServeMux()
	if len(c.DiscordKey) == 32 && c.DiscordGuild != "" {
		mux.HandleFunc("POST /discord", c.discord)
	}
	mux.HandleFunc("GET /api/providers", func(w http.ResponseWriter, r *http.Request) {
		names := []string{}
		for name := range c.Providers {
			names = append(names, name)
		}
		sort.Strings(names)
		writeJSON(w, 200, map[string]any{"providers": names, "default": c.DefaultProvider})
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, c.Games) })
	mux.HandleFunc("GET /api/instances", func(w http.ResponseWriter, r *http.Request) {
		list, err := c.Store.List()
		if err != nil {
			writeError(w, err)
			return
		}
		visible := []Instance{}
		for _, i := range list {
			if i.Phase != "deleted" {
				visible = append(visible, i)
			}
		}
		writeJSON(w, 200, visible)
	})
	mux.HandleFunc("POST /api/instances", func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := readJSON(w, r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		instance, err := c.Create(req)
		if err != nil {
			status := 400
			if errors.Is(err, ErrConflict) {
				status = 409
			}
			if errors.Is(err, ErrShuttingDown) {
				status = 503
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 202, instance)
	})
	mux.HandleFunc("GET /api/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		i, err := c.Store.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, i)
	})
	mux.HandleFunc("POST /api/instances/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		desired := map[string]string{"start": "running", "stop": "stopped"}[action]
		if desired == "" {
			http.NotFound(w, r)
			return
		}
		if err := c.SetDesired(r.PathValue("id"), desired); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 202, map[string]string{"desired": desired})
	})
	mux.HandleFunc("DELETE /api/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := c.SetDesired(r.PathValue("id"), "deleted"); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 202, map[string]string{"desired": "deleted"})
	})
	mux.HandleFunc("GET /api/instances/{id}/logs", c.logs)
	mux.HandleFunc("POST /api/instances/{id}/console", func(w http.ResponseWriter, r *http.Request) {
		i, err := c.Store.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		var req struct {
			Command string `json:"command"`
		}
		if err := readJSON(w, r, &req); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		provider, err := c.provider(i)
		if err != nil {
			writeError(w, err)
			return
		}
		observation, err := provider.Inspect(r.Context(), i)
		if err != nil {
			writeError(w, err)
			return
		}
		body, _ := json.Marshal(req)
		if err := c.agentRequest(r.Context(), i, observation.AgentURL, "POST", "/console", strings.NewReader(string(body)), nil); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "sent"})
	})
	assets, _ := fs.Sub(web, "web")
	static := http.FileServer(http.FS(assets))
	mux.Handle("GET /", static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeJSON(w, 401, map[string]string{"error": "Controller token required"})
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (c *Controller) logs(w http.ResponseWriter, r *http.Request) {
	i, err := c.Store.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	provider, err := c.provider(i)
	if err != nil {
		writeError(w, err)
		return
	}
	observation, err := provider.Inspect(r.Context(), i)
	if err != nil {
		writeError(w, err)
		return
	}
	if observation.AgentURL == "" {
		writeJSON(w, 409, map[string]string{"error": "Server is not running"})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", observation.AgentURL+"/logs", nil)
	if err != nil {
		writeError(w, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+i.AgentToken)
	// The streaming request has no total deadline; cancellation follows the caller.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 5 * time.Second
	transport.TLSClientConfig = securehttp.ClientTLS(i.AgentToken)
	defer transport.CloseIdleConnections()
	res, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		writeError(w, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		writeJSON(w, 502, map[string]string{"error": "Agent logs unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	buffer := make([]byte, 4096)
	for {
		n, err := res.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return
			}
			http.NewResponseController(w).Flush()
		}
		if err != nil {
			return
		}
	}
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := 500
	if errors.Is(err, ErrShuttingDown) {
		status = 503
	}
	if errors.Is(err, ErrNotFound) {
		status = 404
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
