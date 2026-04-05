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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	terminal "github.com/buildkite/terminal-to-html/v3"
	"github.com/fogo-sh/insta-game/sidecar/protocol"
)

// ---- Config ----------------------------------------------------------------

type cfg struct {
	Protocol        string
	MetadataPath    string
	ProtocolFile    string
	GameCmd         string
	GameArgs        []string
	GameQuitCmd     string
	GameQuitTimeout time.Duration
	GamePort        int
	DataURL         string
	DefaultConfig   string
	ConfigPath      string
	RconPassword    string
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
		MetadataPath:    envOr("GAME_METADATA_PATH", "/opt/game.json"),
		ProtocolFile:    envOr("PROTOCOL_FILE", "/opt/protocol.txt"),
		GameCmd:         envOr("GAME_CMD", ""),
		GameArgs:        gameArgs,
		GameQuitCmd:     envOr("GAME_QUIT_CMD", "quit"),
		GameQuitTimeout: time.Duration(quitTimeout) * time.Second,
		GamePort:        gamePort,
		DataURL:         envOr("DATA_URL", ""),
		DefaultConfig:   envOr("DEFAULT_CONFIG_PATH", "/opt/default-server.cfg"),
		ConfigPath:      envOr("CONFIG_PATH", "/opt/data/server.cfg"),
		RconPassword:    envOr("RCON_PASSWORD", ""),
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

func resolveProtocol(c cfg) string {
	if protocol := strings.TrimSpace(os.Getenv("PROTOCOL")); protocol != "" {
		return protocol
	}

	if protocol, err := readMetadataProtocol(c.MetadataPath); err == nil {
		return protocol
	}

	if protocol, err := readProtocolFile(c.ProtocolFile); err == nil {
		return protocol
	}

	return inferProtocolFromConfig(c)
}

func readProtocolFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	protocol := strings.TrimSpace(string(content))
	if protocol == "" {
		return "", fmt.Errorf("protocol file %q is empty", path)
	}
	return protocol, nil
}

func readMetadataProtocol(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var metadata struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return "", err
	}
	if strings.TrimSpace(metadata.Protocol) == "" {
		return "", fmt.Errorf("metadata file %q does not define a protocol", path)
	}
	return strings.TrimSpace(metadata.Protocol), nil
}

func inferProtocolFromConfig(c cfg) string {
	switch strings.ToLower(filepath.Base(c.GameCmd)) {
	case "bzfs":
		return "bzflag"
	case "fteqw.sv":
		return "quake1"
	case "q2proded", "q2reproded", "q2pro":
		return "quake2"
	case "ioq3ded":
		return "quake3"
	case "start-ut99.sh", "ucc-bin-arm64":
		return "ut99"
	case "xonotic-linux-arm64-dedicated":
		return "xonotic"
	}

	switch strings.ToLower(filepath.Base(c.ConfigPath)) {
	case "unrealtournament.ini":
		return "ut99"
	}

	return "xonotic"
}

// ---- Process ----------------------------------------------------------------

var (
	proc     *exec.Cmd
	procIn   io.WriteCloser
	procMu   sync.Mutex
	gameLogs = NewRingLog(200)
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
	cmd.Stdout = io.MultiWriter(os.Stdout, gameLogs)
	cmd.Stderr = io.MultiWriter(os.Stderr, gameLogs)
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
	case "bzflag":
		info, err = protocol.QueryBZFlag(c.GamePort)
	case "quake1":
		info, err = protocol.QueryQuake1(c.GamePort)
	case "quake2":
		info, err = protocol.QueryQuake2(c.GamePort)
	case "quake3":
		info, err = protocol.QueryQuake3(c.GamePort)
	case "ut99":
		info, err = protocol.QueryUT99(c.GamePort)
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

func indexHandler(c cfg) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		procMu.Lock()
		running := proc != nil && proc.ProcessState == nil
		procMu.Unlock()

		info := queryServer(c)

		ready := info != nil
		players := 0
		hostname := ""
		mapName := ""
		if info != nil {
			players = info.Players
			hostname = info.Hostname
			mapName = info.Map
		}

		statusStr := "offline"
		if running && ready {
			statusStr = "running"
		} else if running {
			statusStr = "starting"
		}

		extra := ""
		if hostname != "" {
			extra += fmt.Sprintf("\nhostname: %s", hostname)
		}
		if mapName != "" {
			extra += fmt.Sprintf("\nmap:      %s", mapName)
		}

		body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta http-equiv="refresh" content="5">
  <title>insta-game — %s</title>
  <style>
    body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
    a { color: #7af; }
    pre { line-height: 1.6; }
  </style>
</head>
<body>
<pre>
insta-game — %s

status:   %s
ready:    %v
players:  %d%s
</pre>
<a href="/status">/status (json)</a>
</body>
</html>`,
			c.Protocol,
			c.Protocol,
			statusStr,
			ready,
			players,
			extra,
		)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}
}

// toHTML converts a single log line from ANSI escape codes to HTML.
func toHTML(line string) string {
	return strings.TrimRight(string(terminal.Render([]byte(line))), "\n")
}

func logsHandler(c cfg) http.HandlerFunc {
	return authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")

		// Flush existing buffered lines first.
		for _, line := range gameLogs.Lines() {
			fmt.Fprintf(w, "data: %s\n\n", toHTML(line))
		}
		flusher.Flush()

		// Subscribe to new lines.
		ch, cancel := gameLogs.Subscribe()
		defer cancel()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", toHTML(line))
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}

// ---- Main -------------------------------------------------------------------

func main() {
	c := loadConfig()
	c.Protocol = resolveProtocol(c)

	if err := downloadData(c.DataURL, c.DefaultConfig, c.ConfigPath, c.UserAgent); err != nil {
		log.Fatalf("SIDECAR: data download failed: %v", err)
	}

	if err := configureRcon(c.Protocol, c.ConfigPath, c.RconPassword); err != nil {
		log.Fatalf("SIDECAR: rcon setup failed: %v", err)
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
	mux.HandleFunc("/", indexHandler(c))
	mux.HandleFunc("/status", statusHandler(c))
	mux.HandleFunc("/restart", restartHandler(c))
	mux.HandleFunc("/stop", stopHandler(c))
	mux.HandleFunc("/logs", logsHandler(c))

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	log.Printf("SIDECAR: listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("SIDECAR: server error: %v", err)
	}
}
