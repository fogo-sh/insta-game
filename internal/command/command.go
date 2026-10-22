package command

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fogo-sh/insta-game/internal/agent"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/securehttp"
)

const help = `insta-game — on-demand dedicated servers

  discord  Print or register the Go controller's Discord commands
	aws      Manage an AWS game host
  wake     Answer Quake connection attempts and wake an AWS host
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

Run any command with -h for its flags. Client commands accept --url and --token-file.
INSTA_GAME_TOKEN overrides the token file. Defaults use .insta-game/token.
`

func Run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprint(out, help)
		return nil
	}
	switch args[0] {
	case "discord":
		return discordCommand(ctx, args[1:], out)
	case "aws":
		return awsCommand(ctx, args[1:], out)
	case "wake":
		return wakeCommand(ctx, args[1:], out)
	case "serve":
		return serve(ctx, args[1:], out)
	case "agent":
		if len(args) == 3 && args[1] == "--config" {
			return agent.RunFile(ctx, args[2])
		}
		if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
			fmt.Fprintln(out, "agent: set GAME_CMD, GAME_ARGS, GAME_PORT, CONFIG_PATH, CONFIG_TEXT and TOKEN; DATA_URL and RCON_PASSWORD are optional.")
			return nil
		}
		if len(args) > 1 {
			return fmt.Errorf("agent accepts no flags; configure GAME_CMD, TOKEN and GAME_* variables")
		}
		return agent.Run(ctx, agent.FromEnv())
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
	tlsEnabled := flags.Bool("tls", false, "Serve HTTPS using the controller token as its pinned identity")
	poweroffIdle := flags.Duration("poweroff-idle", 0, "Power off the host after all instances are stopped for this duration (0 disables)")
	dataDir := flags.String("data", ".insta-game", "Private controller data directory")
	discordKey := flags.String("discord-key", os.Getenv("DISCORD_PUBLIC_KEY"), "Discord application public key (hex)")
	discordGuild := flags.String("discord-guild", os.Getenv("DISCORD_GUILD_ID"), "Allowed Discord guild ID")
	nativeConfig := flags.String("native-config", "", "Native engine definitions (JSON file)")
	defaultProvider := flags.String("default-provider", "docker", "Default execution provider")
	catalog := flags.String("catalog", "docker-containers", "Directory containing game folders")
	socket := flags.String("docker-socket", "/var/run/docker.sock", "Local Docker socket")
	publicHost := flags.String("public-host", "localhost", "Hostname/IP advertised to game clients")
	bindIP := flags.String("game-bind", "127.0.0.1", "Host IP for game ports; use 0.0.0.0 for LAN/public access")
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	if *poweroffIdle < 0 {
		return fmt.Errorf("poweroff-idle must not be negative")
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
	controller := control.NewController(store, games, map[string]control.Provider{"docker": control.NewDocker(*socket, *publicHost, *bindIP)}, *defaultProvider)
	if *nativeConfig != "" {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("native process supervision currently requires Linux")
		}
		data, err := os.ReadFile(*nativeConfig)
		if err != nil {
			return err
		}
		var engines map[string]control.NativeGame
		if err := json.Unmarshal(data, &engines); err != nil {
			return err
		}
		root, err := filepath.Abs(filepath.Join(*dataDir, "native"))
		if err != nil {
			return err
		}
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		controller.Providers["local"] = &control.Native{Root: root, Binary: binary, PublicHost: *publicHost, Games: engines}
	}
	if _, ok := controller.Providers[*defaultProvider]; !ok {
		return fmt.Errorf("default provider %q is not configured", *defaultProvider)
	}
	controller.DefaultProvider = *defaultProvider
	if *discordKey != "" {
		key, err := hex.DecodeString(*discordKey)
		if err != nil || len(key) != 32 || *discordGuild == "" {
			return fmt.Errorf("Discord requires a 32-byte public key and an allowed guild ID")
		}
		controller.DiscordKey = key
		controller.DiscordGuild = *discordGuild
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	workers.Go(func() { controller.Run(runCtx) })
	defer func() { cancel(); workers.Wait() }()
	server := &http.Server{Addr: *listen, Handler: controller.Handler(token), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	scheme := "http"
	if *tlsEnabled {
		server.TLSConfig, err = securehttp.ServerTLS(token)
		if err != nil {
			return err
		}
		scheme = "https"
	}
	fmt.Fprintf(out, "insta-game listening on %s://%s\nController token: %s (or INSTA_GAME_TOKEN)\n", scheme, *listen, filepath.Join(*dataDir, "token"))
	serverDone := make(chan error, 1)
	go func() {
		if *tlsEnabled {
			serverDone <- server.ListenAndServeTLS("", "")
		} else {
			serverDone <- server.ListenAndServe()
		}
	}()
	var idleDone chan error
	if *poweroffIdle > 0 {
		idleDone = make(chan error, 1)
		workers.Go(func() {
			idleDone <- poweroffWhenIdle(runCtx, *poweroffIdle, store.List, func(ctx context.Context) (bool, error) {
				committed, err := controller.BeginIdleShutdown()
				if err != nil || !committed {
					return false, err
				}
				return true, exec.CommandContext(ctx, "systemctl", "poweroff").Run()
			})
		})
	}
	var stopErr error
	select {
	case err := <-serverDone:
		return err
	case stopErr = <-idleDone:
	case <-ctx.Done():
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	return errors.Join(stopErr, server.Shutdown(shutdown))
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
		flags.StringVar(&req.Provider, "provider", "", "Execution provider (controller default if omitted)")
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
	transport := &http.Transport{ResponseHeaderTimeout: 10 * time.Second, TLSClientConfig: securehttp.ClientTLS(token)}
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
