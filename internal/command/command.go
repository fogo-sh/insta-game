package command

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/agent"
	"github.com/fogo-sh/insta-game/internal/control"
)

const help = `insta-game — on-demand dedicated servers

  serve    Run the controller and embedded web UI
  agent    Supervise one game (configured through GAME_* environment variables)
  games    List supported games
  list     List server instances
  create   Create a server (--game xonotic --name "Friday games")
  show     Show one server
  start    Start a stopped server
  stop     Stop a server and keep its data
  delete   Permanently remove a server and its data
  logs     Stream game logs
  console  Send a command to the game's standard input
  files    Serve a directory of game downloads, read-only

Run any command with -h for its flags. Client commands accept --url and --token-file.
INSTA_GAME_TOKEN overrides the token file. Defaults use .insta-game/token.
`

func Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprint(out, help)
		return nil
	}
	switch args[0] {
	case "serve":
		return serve(ctx, args[1:], out)
	case "agent":
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Fprintln(out, "agent: set GAME_CMD, GAME_ARGS, GAME_PORT, CONFIG_PATH, CONFIG_TEXT and TOKEN; DATA_URL and RCON_PASSWORD are optional.")
			return nil
		}
		if len(args) > 1 {
			return fmt.Errorf("agent accepts no flags; configure GAME_CMD, TOKEN and GAME_* variables")
		}
		return agent.Run(ctx, agent.FromEnv())
	case "files":
		return files(ctx, args[1:], out)
	default:
		return client(ctx, args[0], args[1:], out)
	}
}
func tokenAt(path string, create bool) (string, error) {
	if value := os.Getenv("INSTA_GAME_TOKEN"); value != "" {
		return value, nil
	}
	data, err := os.ReadFile(path)
	if err == nil {
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("empty token file %s", path)
		}
		return value, nil
	}
	if !os.IsNotExist(err) || !create {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	value := control.NewToken()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := f.WriteString(value + "\n")
	closeErr := f.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return value, nil
}
func serve(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(out)
	listen := flags.String("listen", "127.0.0.1:3000", "HTTP listen address")
	dataDir := flags.String("data", ".insta-game", "Private controller data directory")
	catalog := flags.String("catalog", "docker-containers", "Directory containing game folders")
	socket := flags.String("docker-socket", "/var/run/docker.sock", "Local Docker socket")
	publicHost := flags.String("public-host", "localhost", "Hostname/IP advertised to game clients")
	bindIP := flags.String("game-bind", "127.0.0.1", "Host IP for game ports; use 0.0.0.0 for LAN/public access")
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	games, err := control.LoadCatalog(*catalog)
	if err != nil {
		return err
	}
	store, err := control.OpenStore(filepath.Join(*dataDir, "instances.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	token, err := tokenAt(filepath.Join(*dataDir, "token"), true)
	if err != nil {
		return err
	}
	controller := control.NewController(store, games, control.NewDocker(*socket, *publicHost, *bindIP))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); controller.Run(runCtx) }()
	defer func() { cancel(); <-done }()
	fmt.Fprintf(out, "insta-game listening on http://%s\nController token: %s (or INSTA_GAME_TOKEN)\n", *listen, filepath.Join(*dataDir, "token"))
	return httpServe(ctx, *listen, controller.Handler(token))
}
func httpServe(ctx context.Context, address string, handler http.Handler) error {
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
func flagError(err error) error {
	if err == flag.ErrHelp {
		return nil
	}
	return err
}

func client(ctx context.Context, command string, args []string, out io.Writer) error {
	paths := map[string]string{"games": "/catalog", "list": "/instances", "create": "/instances", "show": "", "start": "", "stop": "", "delete": "", "logs": "", "console": ""}
	path, ok := paths[command]
	if !ok {
		return fmt.Errorf("unknown command %q; run insta-game help", command)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(out)
	base := flags.String("url", "http://127.0.0.1:3000", "Controller URL")
	tokenFile := flags.String("token-file", ".insta-game/token", "Controller token file")
	var req control.CreateRequest
	var configPath string
	if command == "create" {
		flags.StringVar(&req.ID, "id", "", "Stable request ID for idempotent creation (optional)")
		flags.StringVar(&req.Game, "game", "", "Game ID")
		flags.StringVar(&req.Name, "name", "", "Server name")
		flags.StringVar(&configPath, "config", "", "Configuration file (otherwise game defaults)")
		flags.IntVar(&req.IdleMinutes, "idle", 10, "Idle timeout in minutes (0 disables)")
		flags.IntVar(&req.MaxMinutes, "max-runtime", 120, "Maximum runtime in minutes (0 disables)")
	}
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	method := "GET"
	var body io.Reader
	if command == "create" {
		if configPath != "" {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}
			value := string(data)
			req.Config = &value
		}
		data, err := json.Marshal(req)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
		method = "POST"
	} else if path == "" {
		if flags.NArg() < 1 {
			return fmt.Errorf("%s requires a server ID", command)
		}
		id := flags.Arg(0)
		if strings.ContainsAny(id, "/?#") {
			return fmt.Errorf("invalid server ID")
		}
		path = "/instances/" + id
		switch command {
		case "start", "stop":
			method = "POST"
			path += "/" + command
		case "delete":
			method = "DELETE"
		case "logs":
			path += "/logs"
		case "console":
			if flags.NArg() < 2 {
				return fmt.Errorf("console requires a server ID and command")
			}
			data, _ := json.Marshal(map[string]string{"command": strings.Join(flags.Args()[1:], " ")})
			body = bytes.NewReader(data)
			method = "POST"
			path += "/console"
		}
	}
	token, err := tokenAt(*tokenFile, false)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(*base, "/")+"/api"+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	transport := &http.Transport{ResponseHeaderTimeout: 10 * time.Second}
	defer transport.CloseIdleConnections()
	res, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, data)
	}
	if command == "logs" {
		_, err = io.Copy(out, res.Body)
		return err
	}
	var data bytes.Buffer
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if err := json.Indent(&data, raw, "", "  "); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, data.String())
	return err
}
