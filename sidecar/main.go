package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/fogo-sh/insta-game/sidecar/protocol"
)

// ---- Config ----------------------------------------------------------------

type cfg struct {
	Protocol        string
	GameCmd         string
	GameArgs        []string
	GameQuitCmd     string
	GameQuitTimeout time.Duration
	GamePort        int
	DataURL         string
	DefaultConfig   string
	ConfigPath      string
	UserAgent       string
	Token           string
	IdleTimeout     time.Duration
	ECSCluster      string
	ECSService      string
	AWSRegion       string
	Host            string
	Port            int
}

func loadConfig() cfg {
	gameArgs := []string{}
	if v := envOr("GAME_ARGS", ""); v != "" {
		gameArgs = strings.Fields(v)
	}
	quitTimeout, _ := strconv.Atoi(envOr("GAME_QUIT_TIMEOUT", "15"))
	gamePort, _ := strconv.Atoi(envOr("GAME_PORT", "26000"))
	idleTimeout, _ := strconv.Atoi(envOr("IDLE_TIMEOUT_SECONDS", "600"))
	port, _ := strconv.Atoi(envOr("PORT", "5001"))

	return cfg{
		Protocol:        envOr("PROTOCOL", "xonotic"),
		GameCmd:         envOr("GAME_CMD", ""),
		GameArgs:        gameArgs,
		GameQuitCmd:     envOr("GAME_QUIT_CMD", "quit"),
		GameQuitTimeout: time.Duration(quitTimeout) * time.Second,
		GamePort:        gamePort,
		DataURL:         envOr("DATA_URL", ""),
		DefaultConfig:   envOr("DEFAULT_CONFIG_PATH", "/opt/default-server.cfg"),
		ConfigPath:      envOr("CONFIG_PATH", "/opt/data/server.cfg"),
		UserAgent:       envOr("DOWNLOAD_USER_AGENT", ""),
		Token:           envOr("TOKEN", "abc123"),
		IdleTimeout:     time.Duration(idleTimeout) * time.Second,
		ECSCluster:      envOr("ECS_CLUSTER", ""),
		ECSService:      envOr("ECS_SERVICE", ""),
		AWSRegion:       envOr("AWS_REGION", "ca-central-1"),
		Host:            envOr("HOST", "0.0.0.0"),
		Port:            port,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---- Process ----------------------------------------------------------------

var (
	proc   *exec.Cmd
	procIn io.WriteCloser
	procMu sync.Mutex
)

func startGame(c cfg) error {
	procMu.Lock()
	defer procMu.Unlock()

	if proc != nil && proc.ProcessState == nil {
		exitGameLocked(c)
	}

	cmd := exec.Command(c.GameCmd, c.GameArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("start game: %w", err)
	}
	proc = cmd
	procIn = stdin
	log.Printf("SIDECAR: game started (pid %d)", proc.Process.Pid)
	return nil
}

// exitGame acquires the lock and calls exitGameLocked.
func exitGame(c cfg) {
	procMu.Lock()
	defer procMu.Unlock()
	exitGameLocked(c)
}

// exitGameLocked shuts down the game process. Caller must hold procMu.
func exitGameLocked(c cfg) {
	if proc == nil {
		return
	}
	if proc.ProcessState != nil {
		log.Printf("SIDECAR: game process already exited")
		proc = nil
		procIn = nil
		return
	}

	if procIn != nil {
		log.Printf("SIDECAR: sending quit command: %s", c.GameQuitCmd)
		fmt.Fprintf(procIn, "%s\n", c.GameQuitCmd)
		procIn.Close()
		procIn = nil
	}

	done := make(chan error, 1)
	go func() { done <- proc.Wait() }()

	select {
	case <-done:
		log.Printf("SIDECAR: game exited cleanly")
	case <-time.After(c.GameQuitTimeout):
		log.Printf("SIDECAR: game did not exit cleanly, killing")
		proc.Process.Kill()
		<-done
	}
	proc = nil
}

// ---- Protocol ---------------------------------------------------------------

func queryServer(c cfg) *protocol.ServerInfo {
	var info *protocol.ServerInfo
	var err error
	switch c.Protocol {
	case "quake1":
		info, err = protocol.QueryQuake1(c.GamePort)
	case "quake2":
		info, err = protocol.QueryQuake2(c.GamePort)
	default:
		info, err = protocol.QueryXonotic(c.GamePort)
	}
	if err != nil {
		return nil
	}
	return info
}

// ---- Auto-shutdown ----------------------------------------------------------

func autoShutdownLoop(c cfg) {
	if c.IdleTimeout == 0 || c.ECSService == "" {
		log.Printf("SIDECAR: auto-shutdown disabled")
		return
	}
	log.Printf("SIDECAR: auto-shutdown enabled (timeout %s)", c.IdleTimeout)

	lastActive := time.Now()
	for {
		time.Sleep(60 * time.Second)
		info := queryServer(c)
		if info != nil && info.Players > 0 {
			lastActive = time.Now()
			log.Printf("SIDECAR: auto-shutdown: %d player(s) active", info.Players)
			continue
		}
		idle := time.Since(lastActive)
		log.Printf("SIDECAR: auto-shutdown: idle for %ds (timeout %ds)", int(idle.Seconds()), int(c.IdleTimeout.Seconds()))
		if idle >= c.IdleTimeout {
			log.Printf("SIDECAR: idle timeout reached, shutting down ECS service")
			shutdownECS(c)
			return
		}
	}
}

func shutdownECS(c cfg) {
	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.AWSRegion))
	if err != nil {
		log.Printf("SIDECAR: failed to load AWS config: %v", err)
		return
	}
	client := ecs.NewFromConfig(awsCfg)
	desired := int32(0)
	_, err = client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &c.ECSCluster,
		Service:      &c.ECSService,
		DesiredCount: &desired,
	})
	if err != nil {
		log.Printf("SIDECAR: failed to update ECS service: %v", err)
		return
	}
	log.Printf("SIDECAR: ECS service scaled to 0")
}

// ---- HTTP -------------------------------------------------------------------

func authorize(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(auth, "Bearer ")
		if provided != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func statusHandler(c cfg) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		procMu.Lock()
		running := proc != nil && proc.ProcessState == nil
		pid := 0
		if proc != nil && proc.Process != nil {
			pid = proc.Process.Pid
		}
		procMu.Unlock()

		info := queryServer(c)
		resp := map[string]any{
			"running":   running,
			"ready":     info != nil,
			"players":   0,
			"hostname":  "",
			"map":       "",
			"timestamp": time.Now().Format(time.RFC3339),
			"pid":       pid,
		}
		if info != nil {
			resp["players"] = info.Players
			resp["hostname"] = info.Hostname
			resp["map"] = info.Map
		}
		jsonResponse(w, resp)
	}
}

func restartHandler(c cfg) http.HandlerFunc {
	return authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		exitGame(c)
		if err := startGame(c); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		procMu.Lock()
		pid := proc.Process.Pid
		procMu.Unlock()
		jsonResponse(w, map[string]any{"pid": pid, "status": "restarted"})
	})
}

func stopHandler(c cfg) http.HandlerFunc {
	return authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		exitGame(c)
		jsonResponse(w, map[string]any{"status": "stopped"})
	})
}

// ---- Main -------------------------------------------------------------------

func main() {
	c := loadConfig()

	if err := downloadData(c.DataURL, c.DefaultConfig, c.ConfigPath, c.UserAgent); err != nil {
		log.Fatalf("SIDECAR: data download failed: %v", err)
	}

	if err := startGame(c); err != nil {
		log.Fatalf("SIDECAR: failed to start game: %v", err)
	}

	go autoShutdownLoop(c)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Printf("SIDECAR: signal received, shutting down")
		exitGame(c)
		os.Exit(0)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", statusHandler(c))
	mux.HandleFunc("/restart", restartHandler(c))
	mux.HandleFunc("/stop", stopHandler(c))

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	log.Printf("SIDECAR: listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("SIDECAR: server error: %v", err)
	}
}
