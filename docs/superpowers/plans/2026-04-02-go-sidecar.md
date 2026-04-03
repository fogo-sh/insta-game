# Go Sidecar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the two duplicated Python sidecar services with a single statically-compiled Go binary that is configured entirely by env vars and downloaded into each game image at build time.

**Architecture:** A single Go module at `sidecar/` with four files — `main.go` (HTTP server, process lifecycle, auto-shutdown), `download.go` (DATA_URL fetch/extract), and two protocol implementations under `protocol/` (xonotic UDP getinfo, quake1 NetQuake binary query). The binary is built for `linux/arm64` in CI and published as a GitHub Release asset; both Dockerfiles fetch it with curl and delete their Python/uv setup.

**Tech Stack:** Go 1.22, AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`), Go stdlib `net/http` (no external HTTP framework)

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `sidecar/go.mod` | Create | Go module definition |
| `sidecar/main.go` | Create | Config, HTTP routes, process lifecycle, auto-shutdown loop, signal handling |
| `sidecar/download.go` | Create | DATA_URL fetch + zip/raw extract with retry |
| `sidecar/protocol/xonotic.go` | Create | DarkPlaces UDP `getinfo` query |
| `sidecar/protocol/quake1.go` | Create | NetQuake `CCREQ_SERVER_INFO` binary query |
| `sidecar/Makefile` | Create | `build`, `test`, `vet` targets |
| `.github/workflows/publish-sidecar.yml` | Create | Build ARM64 binary, publish to `sidecar-latest` release |
| `docker-containers/xonotic/Dockerfile` | Modify | Remove uv/Python, add sidecar curl download |
| `docker-containers/qssm/Dockerfile` | Modify | Remove uv/Python, add sidecar curl download |
| `docker-containers/xonotic/Makefile` | Modify | Remove `sidecar-service` from `RUFF_TARGETS` |
| `docker-containers/qssm/Makefile` | Modify | Remove `sidecar-service` from `RUFF_TARGETS` |
| `pulumi/game_service.py` | Modify | Add `protocol`, `game_cmd`, `game_args`, `game_quit_cmd`, `game_quit_timeout` params |
| `pulumi/__main__.py` | Modify | Pass new params to both `GameService` instantiations |
| `docker-containers/xonotic/sidecar-service/` | Delete | Replaced by Go binary |
| `docker-containers/qssm/sidecar-service/` | Delete | Replaced by Go binary |

---

### Task 1: Scaffold the Go module and protocol package

**Files:**
- Create: `sidecar/go.mod`
- Create: `sidecar/protocol/xonotic.go`
- Create: `sidecar/protocol/quake1.go`

- [ ] **Step 1: Create the Go module**

```bash
mkdir -p sidecar/protocol
```

Write `sidecar/go.mod`:

```go
module github.com/fogo-sh/insta-game/sidecar

go 1.22

require (
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/config v1.27.11
	github.com/aws/aws-sdk-go-v2/service/ecs v1.41.4
)
```

- [ ] **Step 2: Write the Xonotic protocol implementation**

Write `sidecar/protocol/xonotic.go`:

```go
package protocol

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// QueryXonotic sends a DarkPlaces UDP getinfo query to 127.0.0.1:port and
// returns server info. Returns nil if the server does not respond.
func QueryXonotic(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte("\xff\xff\xff\xffgetinfo"))
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	// Response: \xff\xff\xff\xffinfoResponse\n\key\val\key\val\...
	payload := string(buf[17:n])
	parts := strings.Split(payload, "\\")
	kv := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		kv[parts[i]] = parts[i+1]
	}

	players := 0
	if v, ok := kv["clients"]; ok {
		fmt.Sscanf(v, "%d", &players)
	}

	return &ServerInfo{Players: players}, nil
}
```

- [ ] **Step 3: Write the Quake 1 protocol implementation**

Write `sidecar/protocol/quake1.go`:

```go
package protocol

import (
	"fmt"
	"net"
	"time"
)

// QueryQuake1 sends a NetQuake CCREQ_SERVER_INFO control packet to
// 127.0.0.1:port and returns server info. Returns nil if the server does not
// respond or the response is malformed.
func QueryQuake1(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte("\x80\x00\x00\x0c\x02QUAKE\x00\x03"))
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	if n < 7 || buf[0] != 0x80 || buf[1] != 0x00 || buf[4] != 0x83 {
		return nil, fmt.Errorf("unexpected response header")
	}

	payload := buf[5:n]
	parts := splitNull(payload)
	if len(parts) < 3 {
		return nil, fmt.Errorf("response too short")
	}

	hostname := string(parts[1])
	mapName := string(parts[2])
	players := 0
	if len(payload) >= 3 {
		players = int(payload[len(payload)-3])
	}

	return &ServerInfo{
		Players:  players,
		Hostname: hostname,
		Map:      mapName,
	}, nil
}

func splitNull(b []byte) [][]byte {
	var parts [][]byte
	start := 0
	for i, c := range b {
		if c == 0x00 {
			parts = append(parts, b[start:i])
			start = i + 1
		}
	}
	parts = append(parts, b[start:])
	return parts
}
```

- [ ] **Step 4: Write the shared ServerInfo type**

Create `sidecar/protocol/protocol.go`:

```go
package protocol

// ServerInfo holds the result of a game server status query.
// Fields not provided by a protocol are left as zero values.
type ServerInfo struct {
	Players  int
	Hostname string
	Map      string
}
```

- [ ] **Step 5: Verify the package compiles**

```bash
cd sidecar
go build ./protocol/...
```

Expected: no output, no errors.

- [ ] **Step 6: Commit**

```bash
git add sidecar/
git commit -m "feat(sidecar): scaffold Go module and protocol implementations"
```

---

### Task 2: Write download.go

**Files:**
- Create: `sidecar/download.go`

- [ ] **Step 1: Write download.go**

Write `sidecar/download.go`:

```go
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadData(dataURL, defaultConfigPath, userAgent string) error {
	entries := parseDataURL(dataURL)

	if len(entries) == 0 {
		log.Printf("SIDECAR: No DATA_URL set, using bundled default config: %s", defaultConfigPath)
		return copyFile(defaultConfigPath, "/opt/data/server.cfg")
	}

	for _, e := range entries {
		log.Printf("SIDECAR: Downloading data from: %s", e.url)
		content, err := fetchWithRetry(e.url, userAgent)
		if err != nil {
			return fmt.Errorf("download %s: %w", e.url, err)
		}

		if isZip(content) {
			dest := e.path
			if dest == "" {
				dest = "/opt/"
			}
			log.Printf("SIDECAR: Extracting zip to %s", dest)
			if err := extractZip(content, dest); err != nil {
				return fmt.Errorf("extract %s: %w", e.url, err)
			}
		} else {
			log.Printf("SIDECAR: Saving file to %s", e.path)
			if err := os.WriteFile(e.path, content, 0644); err != nil {
				return fmt.Errorf("write %s: %w", e.path, err)
			}
		}
	}
	return nil
}

type dataEntry struct {
	url  string
	path string
}

func parseDataURL(raw string) []dataEntry {
	var out []dataEntry
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		url, path, _ := strings.Cut(part, "=")
		out = append(out, dataEntry{url: strings.TrimSpace(url), path: strings.TrimSpace(path)})
	}
	return out
}

func fetchWithRetry(url, userAgent string) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

	var lastErr error
	for i := 0; i <= len(backoff); i++ {
		if i > 0 {
			time.Sleep(backoff[i-1])
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	return nil, lastErr
}

func isZip(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04
}

func extractZip(data []byte, dest string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd sidecar
go build ./...
```

Expected: no output, no errors. (Will fail to link until main.go exists — that's fine if `go build ./...` fails only on missing `main` package; use `go vet ./...` instead if needed.)

- [ ] **Step 3: Commit**

```bash
git add sidecar/download.go
git commit -m "feat(sidecar): add data download with retry"
```

---

### Task 3: Write main.go

**Files:**
- Create: `sidecar/main.go`

- [ ] **Step 1: Write main.go**

Write `sidecar/main.go`:

```go
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
	Protocol        string // "xonotic" or "quake1"
	GameCmd         string
	GameArgs        []string
	GameQuitCmd     string
	GameQuitTimeout time.Duration
	GamePort        int
	DataURL         string
	DefaultConfig   string
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
	if v := env("GAME_ARGS", ""); v != "" {
		gameArgs = strings.Fields(v)
	}
	quitTimeout, _ := strconv.Atoi(env("GAME_QUIT_TIMEOUT", "15"))
	gamePort, _ := strconv.Atoi(env("GAME_PORT", "26000"))
	idleTimeout, _ := strconv.Atoi(env("IDLE_TIMEOUT_SECONDS", "600"))
	port, _ := strconv.Atoi(env("PORT", "5001"))

	return cfg{
		Protocol:        env("PROTOCOL", "xonotic"),
		GameCmd:         env("GAME_CMD", ""),
		GameArgs:        gameArgs,
		GameQuitCmd:     env("GAME_QUIT_CMD", "quit"),
		GameQuitTimeout: time.Duration(quitTimeout) * time.Second,
		GamePort:        gamePort,
		DataURL:         env("DATA_URL", ""),
		DefaultConfig:   env("DEFAULT_CONFIG_PATH", "/opt/default-server.cfg"),
		UserAgent:       env("DOWNLOAD_USER_AGENT", ""),
		Token:           env("TOKEN", "abc123"),
		IdleTimeout:     time.Duration(idleTimeout) * time.Second,
		ECSCluster:      env("ECS_CLUSTER", ""),
		ECSService:      env("ECS_SERVICE", ""),
		AWSRegion:       env("AWS_REGION", "ca-central-1"),
		Host:            env("HOST", "0.0.0.0"),
		Port:            port,
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---- Process ----------------------------------------------------------------

var (
	proc   *exec.Cmd
	procMu sync.Mutex
)

func startGame(c cfg) error {
	procMu.Lock()
	defer procMu.Unlock()

	if proc != nil && proc.ProcessState == nil {
		exitGame(c)
	}

	cmd := exec.Command(c.GameCmd, c.GameArgs...)
	cmd.Stdin, _ = io.Pipe()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start game: %w", err)
	}
	proc = cmd
	log.Printf("SIDECAR: game started (pid %d)", proc.Process.Pid)
	return nil
}

func exitGame(c cfg) {
	if proc == nil {
		return
	}
	if proc.ProcessState != nil {
		log.Printf("SIDECAR: game process already exited")
		proc = nil
		return
	}

	if stdin, ok := proc.Stdin.(io.WriteCloser); ok {
		log.Printf("SIDECAR: sending quit command: %s", c.GameQuitCmd)
		fmt.Fprintf(stdin, "%s\n", c.GameQuitCmd)
		stdin.Close()
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
		procMu.Lock()
		exitGame(c)
		procMu.Unlock()
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
		procMu.Lock()
		exitGame(c)
		procMu.Unlock()
		jsonResponse(w, map[string]any{"status": "stopped"})
	})
}

// ---- Main -------------------------------------------------------------------

func main() {
	c := loadConfig()

	if err := downloadData(c.DataURL, c.DefaultConfig, c.UserAgent); err != nil {
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
		procMu.Lock()
		exitGame(c)
		procMu.Unlock()
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

```

- [ ] **Step 2: Fetch Go dependencies**

```bash
cd sidecar
go mod tidy
```

Expected: `go.sum` created, dependencies downloaded. No errors.

- [ ] **Step 3: Build the binary for linux/arm64**

```bash
cd sidecar
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o sidecar-arm64 .
```

Expected: `sidecar-arm64` file created, no errors.

- [ ] **Step 4: Verify vet passes**

```bash
cd sidecar
go vet ./...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add sidecar/
git commit -m "feat(sidecar): add main.go with HTTP server, process lifecycle, auto-shutdown"
```

---

### Task 4: Write the Makefile for sidecar/

**Files:**
- Create: `sidecar/Makefile`

- [ ] **Step 1: Write sidecar/Makefile**

```makefile
BINARY=sidecar
GOOS=linux
GOARCH=arm64
CGO_ENABLED=0

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

build-local:
	go build -o $(BINARY)-local .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY)-local
```

- [ ] **Step 2: Verify make build works**

```bash
cd sidecar
make build
```

Expected: `sidecar` binary created. No errors.

- [ ] **Step 3: Commit**

```bash
git add sidecar/Makefile
git commit -m "chore(sidecar): add Makefile"
```

---

### Task 5: Add CI workflow to build and publish the sidecar binary

**Files:**
- Create: `.github/workflows/publish-sidecar.yml`

- [ ] **Step 1: Write publish-sidecar.yml**

Write `.github/workflows/publish-sidecar.yml`:

```yaml
name: Build and publish sidecar binary

on:
  push:
    branches: ["main"]
    paths:
      - "sidecar/**"

jobs:
  build-and-publish:
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v2

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Build sidecar binary (linux/arm64)
        run: |
          cd sidecar
          GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o sidecar .

      - name: Publish to sidecar-latest release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: sidecar-latest
          name: sidecar-latest
          files: sidecar/sidecar
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/publish-sidecar.yml
git commit -m "ci: add sidecar build and publish workflow"
```

---

### Task 6: Update the Xonotic Dockerfile

**Files:**
- Modify: `docker-containers/xonotic/Dockerfile`

- [ ] **Step 1: Replace the runtime stage of the Xonotic Dockerfile**

The current runtime stage (lines 19–45) is:

```dockerfile
FROM debian:bookworm-slim

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

RUN apt-get update && apt-get -y install --no-install-recommends \
    libcurl4 libgmp10 libjpeg62-turbo libpng16-16 zlib1g \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt

COPY --from=builder /build/xonotic/darkplaces/darkplaces-dedicated ./xonotic-linux-arm64-dedicated
COPY --from=builder /usr/local/lib/libd0_blind_id* /usr/local/lib/
RUN ldconfig

COPY Xonotic-clean/data ./data
RUN mkdir -p /opt/data && touch /opt/data/server.cfg

COPY server.cfg /opt/default-server.cfg
COPY sidecar-service/pyproject.toml .
COPY sidecar-service/sidecar-service.py .
RUN uv sync --no-install-project --python 3.12.3

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/opt/.venv/bin/python"]
CMD ["-u", "sidecar-service.py"]
```

Replace it with:

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update && apt-get -y install --no-install-recommends \
    libcurl4 libgmp10 libjpeg62-turbo libpng16-16 zlib1g \
    ca-certificates curl \
    && curl -fsSL \
       "https://github.com/fogo-sh/insta-game/releases/download/sidecar-latest/sidecar" \
       -o /usr/local/bin/sidecar \
    && chmod +x /usr/local/bin/sidecar \
    && apt-get purge -y curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt

COPY --from=builder /build/xonotic/darkplaces/darkplaces-dedicated ./xonotic-linux-arm64-dedicated
COPY --from=builder /usr/local/lib/libd0_blind_id* /usr/local/lib/
RUN ldconfig

COPY Xonotic-clean/data ./data
RUN mkdir -p /opt/data && touch /opt/data/server.cfg

COPY server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
```

- [ ] **Step 2: Commit**

```bash
git add docker-containers/xonotic/Dockerfile
git commit -m "feat(xonotic): replace Python sidecar with Go binary in Dockerfile"
```

---

### Task 7: Update the QSS-M Dockerfile

**Files:**
- Modify: `docker-containers/qssm/Dockerfile`

- [ ] **Step 1: Replace the runtime stage of the QSS-M Dockerfile**

The current runtime stage (lines 32–63) is:

```dockerfile
FROM debian:bookworm-slim

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv
COPY --from=qssm-builder /usr/local/bin/qssm /opt/qssm
COPY --from=qssm-builder /usr/local/share/quakespasm.pak /opt/quakespasm.pak
COPY --from=qssm-builder /usr/local/share/qssm.pak /opt/qssm.pak

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4 \
    libgl1 \
    libgnutls30 \
    libmad0 \
    libopusfile0 \
    libsdl2-2.0-0 \
    libvorbisfile3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY server.cfg /opt/default-server.cfg
COPY sidecar-service/pyproject.toml .
COPY sidecar-service/sidecar-service.py .

RUN uv sync --no-install-project --python 3.12.3

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/opt/.venv/bin/python"]
CMD ["-u", "sidecar-service.py"]
```

Replace it with:

```dockerfile
FROM debian:bookworm-slim

COPY --from=qssm-builder /usr/local/bin/qssm /opt/qssm
COPY --from=qssm-builder /usr/local/share/quakespasm.pak /opt/quakespasm.pak
COPY --from=qssm-builder /usr/local/share/qssm.pak /opt/qssm.pak

WORKDIR /opt

RUN apt-get update && apt-get install -y --no-install-recommends \
    libcurl4 \
    libgl1 \
    libgnutls30 \
    libmad0 \
    libopusfile0 \
    libsdl2-2.0-0 \
    libvorbisfile3 \
    zlib1g \
    ca-certificates \
    curl \
    && curl -fsSL \
       "https://github.com/fogo-sh/insta-game/releases/download/sidecar-latest/sidecar" \
       -o /usr/local/bin/sidecar \
    && chmod +x /usr/local/bin/sidecar \
    && apt-get purge -y curl \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/id1 /opt/data

COPY server.cfg /opt/default-server.cfg

EXPOSE 26000/udp
EXPOSE 5001/tcp

ENTRYPOINT ["/usr/local/bin/sidecar"]
```

- [ ] **Step 2: Commit**

```bash
git add docker-containers/qssm/Dockerfile
git commit -m "feat(qssm): replace Python sidecar with Go binary in Dockerfile"
```

---

### Task 8: Delete Python sidecar directories and update Makefiles

**Files:**
- Delete: `docker-containers/xonotic/sidecar-service/`
- Delete: `docker-containers/qssm/sidecar-service/`
- Modify: `docker-containers/xonotic/Makefile`
- Modify: `docker-containers/qssm/Makefile`

- [ ] **Step 1: Delete the Python sidecar directories**

```bash
rm -rf docker-containers/xonotic/sidecar-service/
rm -rf docker-containers/qssm/sidecar-service/
```

- [ ] **Step 2: Update the Xonotic Makefile to remove sidecar-service from RUFF_TARGETS**

In `docker-containers/xonotic/Makefile`, change line 4 from:

```makefile
RUFF_TARGETS=../../pulumi ../../lambda sidecar-service
```

to:

```makefile
RUFF_TARGETS=../../pulumi ../../lambda
```

- [ ] **Step 3: Update the QSS-M Makefile to remove sidecar-service from RUFF_TARGETS**

In `docker-containers/qssm/Makefile`, change line 4 from:

```makefile
RUFF_TARGETS=../../pulumi ../../lambda sidecar-service
```

to:

```makefile
RUFF_TARGETS=../../pulumi ../../lambda
```

- [ ] **Step 4: Verify ruff still runs cleanly**

```bash
cd docker-containers/xonotic
make ruff
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add -A docker-containers/
git commit -m "chore: delete Python sidecar dirs, update Makefile ruff targets"
```

---

### Task 9: Update Pulumi to pass new sidecar env vars

**Files:**
- Modify: `pulumi/game_service.py`
- Modify: `pulumi/__main__.py`

- [ ] **Step 1: Add new parameters to GameService in game_service.py**

In `pulumi/game_service.py`, add five new parameters to `__init__` after `idle_timeout_seconds`:

```python
    idle_timeout_seconds: int = 600,
    protocol: str = "xonotic",
    game_cmd: str = "",
    game_args: str = "",
    game_quit_cmd: str = "quit",
    game_quit_timeout: int = 15,
    cpu_architecture: str = "X86_64",
```

Then in the `environment` list inside `container_defs`, add the new vars after the existing ones:

```python
                        "environment": [
                            {"name": "AWS_REGION", "value": region},
                            {"name": "ECS_CLUSTER", "value": args[2]},
                            {"name": "ECS_SERVICE", "value": service_name},
                            {"name": "TOKEN", "value": args[1]},
                            {"name": "IDLE_TIMEOUT_SECONDS", "value": str(idle_timeout_seconds)},
                            {"name": "PROTOCOL", "value": protocol},
                            {"name": "GAME_CMD", "value": game_cmd},
                            {"name": "GAME_ARGS", "value": game_args},
                            {"name": "GAME_QUIT_CMD", "value": game_quit_cmd},
                            {"name": "GAME_QUIT_TIMEOUT", "value": str(game_quit_timeout)},
                            *([{"name": "DATA_URL", "value": data_url}] if data_url else []),
                        ],
```

- [ ] **Step 2: Pass new params in __main__.py for the xonotic GameService**

In `pulumi/__main__.py`, update the `xonotic = GameService(...)` call to add:

```python
xonotic = GameService(
    "xonotic-arm",
    game_name="xonotic-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:xonotic",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=xonotic_data_url,
    protocol="xonotic",
    game_cmd="./xonotic-linux-arm64-dedicated",
    game_args="",
    game_quit_cmd="exit",
    game_quit_timeout=30,
)
```

- [ ] **Step 3: Pass new params in __main__.py for the qssm GameService**

```python
qssm = GameService(
    "qssm-arm",
    game_name="qssm-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:qssm",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=qss_m_data_url,
    protocol="quake1",
    game_cmd="./qssm",
    game_args="-dedicated 12 -basedir /opt -game id1 -port 26000 +exec server.cfg",
    game_quit_cmd="quit",
    game_quit_timeout=15,
)
```

- [ ] **Step 4: Run ruff**

```bash
cd pulumi
uv run ruff check . && uv run ruff format --check .
```

Expected: no errors.

- [ ] **Step 5: Run pulumi preview**

```bash
cd pulumi
uv run pulumi preview
```

Expected: both ECS task definitions updated (new env vars added), no resource replacements or deletions.

- [ ] **Step 6: Commit**

```bash
git add pulumi/game_service.py pulumi/__main__.py
git commit -m "infra: pass Go sidecar env vars to ECS task definitions"
```

---

### Task 10: Update AGENTS.md and README.md

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`

- [ ] **Step 1: Update AGENTS.md project structure description**

In `AGENTS.md`, update the first paragraph of **Project Structure & Module Organization** to mention `sidecar/`:

```
`pulumi/` contains AWS infrastructure code in Python. `pulumi/__main__.py` defines the stack, and `pulumi/game_service.py` holds the reusable ECS game service component. `lambda/launcher/` contains the Lambda Function URL handler for start, stop, and status operations. `sidecar/` contains the Go sidecar binary source. `docker-containers/xonotic/` and `docker-containers/qssm/` contain the game image builds, local shell scripts, and Docker Compose setup. There is no dedicated `tests/` directory yet; validation is mostly command-based.
```

Also update the **Build, Test, and Development Commands** section to add sidecar commands. After the `docker-containers` block add:

```
From `sidecar/`:

​```sh
make build   # cross-compile for linux/arm64
make test    # go test ./...
make vet     # go vet ./...
​```
```

- [ ] **Step 2: Update README.md repository layout**

In `README.md`, add `sidecar/` to the **Repository Layout** list:

```markdown
- `pulumi/`: AWS infrastructure in Python, managed with `uv`
- `lambda/launcher/`: public Lambda handler for start, stop, and status
- `sidecar/`: Go sidecar binary — HTTP control API and process manager for game containers
- `docker-containers/xonotic/`: Xonotic server image (ARM64), built from source via the Xonotic git repo
- `docker-containers/qssm/`: QSS-M Quake server image, sidecar service, and local build scripts
```

- [ ] **Step 3: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: document Go sidecar in AGENTS.md and README"
```
