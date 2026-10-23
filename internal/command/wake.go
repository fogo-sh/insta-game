package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/securehttp"
	"github.com/fogo-sh/insta-game/internal/wake"
)

// wakeGame starts a saved game through its controller, optionally waking EC2 first.
func wakeGame(ctx context.Context, host *cloud.Host, controllerURL, token, id, protocol string) (string, error) {
	id, portText, _ := strings.Cut(id, "/")
	port := 0
	if portText != "" {
		var err error
		port, err = strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("invalid catalog port %q", portText)
		}
	}
	requestedAt := time.Now()
	if host != nil {
		ip, err := host.Wake(ctx)
		if err != nil {
			return "", err
		}
		controllerURL = "https://" + net.JoinHostPort(ip, "3000")
	}
	transport := &http.Transport{TLSClientConfig: securehttp.ClientTLS(token)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	base := strings.TrimRight(controllerURL, "/") + "/api/instances/" + url.PathEscape(id)
	request := func(method, address string, result any) error {
		req, err := http.NewRequestWithContext(ctx, method, address, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return fmt.Errorf("game %s: %w", id, os.ErrNotExist)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("controller HTTP %d", res.StatusCode)
		}
		if result != nil {
			return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(result)
		}
		_, err = io.Copy(io.Discard, res.Body)
		return err
	}
	lastWake := time.Now()
	started := false
	for {
		var instance control.Instance
		err := request(http.MethodGet, base, &instance)
		if errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && instance.UpdatedAt.After(requestedAt) {
			if instance.Game.Protocol != protocol {
				return "", fmt.Errorf("route protocol %s does not match game protocol %s", protocol, instance.Game.Protocol)
			}
			if instance.Phase == "deleted" || instance.Phase == "failed" {
				return "", fmt.Errorf("game %s is %s: %s", id, instance.Phase, instance.Error)
			}
			if instance.Desired == "stopped" && !started {
				err = request(http.MethodPost, base+"/start", nil)
				if err == nil {
					started = true
				}
			}
			if err == nil && instance.Phase == "running" && instance.Desired == "running" && instance.Error == "" && instance.Players != nil && instance.Connect() != "" {
				if port == 0 {
					return instance.Connect(), nil
				}
				for _, endpoint := range instance.Endpoints {
					if endpoint.Port == port {
						return endpoint.Address, nil
					}
				}
				return "", fmt.Errorf("game %s has no endpoint for catalog port %d", id, port)
			}
		}
		if err != nil && host != nil && time.Since(lastWake) >= 10*time.Second {
			ip, wakeErr := host.Wake(ctx)
			if wakeErr != nil {
				return "", wakeErr
			}
			base = "https://" + net.JoinHostPort(ip, "3000") + "/api/instances/" + url.PathEscape(id)
			requestedAt, lastWake = time.Now(), time.Now()
			started = false
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func wakeCommand(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("wake", flag.ContinueOnError)
	configPath := flags.String("config", "", "Optional AWS host configuration; wake EC2 before the game")
	controllerURL := flags.String("url", "http://127.0.0.1:3000", "Controller URL without AWS hosting")
	tokenPath := flags.String("token-file", ".insta-game/token", "Controller token file without AWS hosting")
	var mappings []wake.Mapping
	flags.Func("route", "[PROTOCOL://]listen-address=instance-id[/catalog-port] (repeatable; private/tailnet only)", func(value string) error {
		listen, id, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(id) == "" {
			return fmt.Errorf("route must be listen-address=instance-id")
		}
		protocol := "quake1"
		if scheme, address, ok := strings.Cut(listen, "://"); ok {
			protocol, listen = scheme, address
		}
		if base, port, hasPort := strings.Cut(id, "/"); hasPort {
			if strings.TrimSpace(base) == "" {
				return fmt.Errorf("route requires a game ID")
			}
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("invalid catalog port")
			}
		}
		mappings = append(mappings, wake.Mapping{Listen: listen, InstanceID: id, Protocol: protocol})
		return nil
	})
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	endpoint, err := url.ParseRequestURI(*controllerURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("url must be an HTTP(S) controller address")
	}
	if len(mappings) == 0 {
		return fmt.Errorf("at least one --route is required")
	}
	var host *cloud.Host
	if *configPath != "" {
		conflict := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "url" || f.Name == "token-file" {
				conflict = true
			}
		})
		if conflict {
			return fmt.Errorf("--config cannot be combined with --url or --token-file")
		}
		config, err := cloud.Load(*configPath)
		if err != nil {
			return err
		}
		*tokenPath = config.TokenPath
		host, err = cloud.NewHost(ctx, config)
		if err != nil {
			return err
		}
	}
	token, err := tokenAt(*tokenPath, false)
	if err != nil {
		return err
	}
	gateway, err := wake.New(mappings, func(ctx context.Context, id string) (string, error) {
		for _, mapping := range mappings {
			if mapping.InstanceID == id {
				return wakeGame(ctx, host, *controllerURL, token, id, mapping.Protocol)
			}
		}
		return "", fmt.Errorf("unknown route %s", id)
	})
	if err != nil {
		return err
	}
	for _, mapping := range mappings {
		fmt.Fprintf(out, "Wake gateway %s://%s -> %s\n", mapping.Protocol, mapping.Listen, mapping.InstanceID)
	}
	return gateway.Serve(ctx)
}
