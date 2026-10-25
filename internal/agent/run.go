package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fogo-sh/insta-game/internal/securehttp"
)

var gameLogs = NewRingLog(200)

func Run(ctx context.Context, c Config) error {
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	if c.GameCmd == "" {
		return fmt.Errorf("GAME_CMD is required")
	}
	if c.Token == "" {
		return fmt.Errorf("TOKEN is required")
	}
	if c.GameQuitTimeout <= 0 {
		c.GameQuitTimeout = 15 * time.Second
	}
	if c.Protocol == "" {
		return fmt.Errorf("PROTOCOL is required")
	}
	root := c.DataRoot
	if root == "" {
		root = filepath.Dir(c.ConfigPath)
	}
	if err := seedData(ctx, c.SeedPath, root); err != nil {
		return err
	}
	if err := seedFiles(ctx, c.AssetSeed, root, ".insta-game-assets-seeded"); err != nil {
		return err
	}
	if err := downloadData(c.DataURL, c.DefaultConfig, c.ConfigPath, c.UserAgent); err != nil {
		return err
	}
	if c.ConfigText != nil {
		if err := os.WriteFile(c.ConfigPath, []byte(*c.ConfigText), 0600); err != nil {
			return err
		}
	}
	if err := configureRcon(c.Protocol, c.ConfigPath, c.RconPassword); err != nil {
		return err
	}
	var process Process
	if err := process.Start(c); err != nil {
		return err
	}
	defer process.Stop(c)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", c.Host, c.Port), Handler: handler(c, &process, cancelRun), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	if c.TLS {
		config, err := securehttp.ServerTLS(c.Token)
		if err != nil {
			return err
		}
		server.TLSConfig = config
		go func() { done <- server.ListenAndServeTLS("", "") }()
	} else {
		go func() { done <- server.ListenAndServe() }()
	}
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func handler(c Config, p *Process, shutdown func()) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shutdown", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]string{"status": "shutting down"})
		shutdown()
	}))
	mux.HandleFunc("GET /status", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		running, failure := p.State()
		response := map[string]any{"running": running, "ready": false, "players": 0, "hostname": "", "map": "", "error": failure}
		if running {
			if info := queryServer(c); info != nil {
				response["ready"] = true
				response["players"] = info.Players
				response["hostname"] = info.Hostname
				response["map"] = info.Map
			}
		}
		jsonResponse(w, response)
	}))
	mux.HandleFunc("POST /restart", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		var req restartRequest
		if r.ContentLength != 0 {
			if err := decodeRequest(w, r, &req); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
		}
		p.Stop(c)
		if err := applyRestartConfig(c, req); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := p.Start(c); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "restarted"})
	}))
	mux.HandleFunc("POST /stop", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		p.Stop(c)
		jsonResponse(w, map[string]string{"status": "stopped"})
	}))
	mux.HandleFunc("POST /console", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Command string `json:"command"`
		}
		if err := decodeRequest(w, r, &req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := p.Command(req.Command); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		jsonResponse(w, map[string]string{"status": "sent"})
	}))
	mux.HandleFunc("GET /config", authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(c.ConfigPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	}))
	mux.HandleFunc("GET /logs", logsHandler(c))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "insta-game agent\n/status\n/logs")
	})
	return mux
}

func decodeRequest(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}
