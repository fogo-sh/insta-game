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

type wakeTarget struct {
	id, protocol string
	port         int
}

type controllerHTTPError int

func (e controllerHTTPError) Error() string { return fmt.Sprintf("controller HTTP %d", e) }

// wakeGame starts a saved game through its controller, optionally waking EC2 first.
func wakeGame(ctx context.Context, host *cloud.Host, controllerURL, token string, target wakeTarget) (string, error) {
	id := target.id
	var observedAt time.Time
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
			return controllerHTTPError(res.StatusCode)
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
		if err == nil {
			// Wait for reconciliation without comparing clocks on different hosts.
			fresh := !observedAt.IsZero() && !instance.UpdatedAt.Equal(observedAt)
			if observedAt.IsZero() {
				observedAt = instance.UpdatedAt
			}
			if instance.Game.Protocol != target.protocol {
				return "", fmt.Errorf("route protocol %s does not match game protocol %s", target.protocol, instance.Game.Protocol)
			}
			if fresh && (instance.Phase == "deleted" || instance.Phase == "failed") {
				return "", fmt.Errorf("game %s is %s: %s", id, instance.Phase, instance.Error)
			}
			if instance.Desired == "stopped" && !started {
				err = request(http.MethodPost, base+"/start", nil)
				if err == nil {
					started = true
				}
			}
			if fresh && err == nil && instance.Phase == "running" && instance.Desired == "running" && instance.Error == "" && instance.Players != nil && instance.Connect() != "" {
				if target.port == 0 {
					return instance.Connect(), nil
				}
				for _, endpoint := range instance.Endpoints {
					if endpoint.Port == target.port {
						return endpoint.Address, nil
					}
				}
				return "", fmt.Errorf("game %s has no endpoint for catalog port %d", id, target.port)
			}
		}
		var status controllerHTTPError
		if errors.Is(err, os.ErrNotExist) || (errors.As(err, &status) && status >= 400 && status < 500 && status != 408 && status != 409 && status != 429) {
			return "", err
		}
		if err != nil && host != nil && time.Since(lastWake) >= 10*time.Second {
			ip, wakeErr := host.Wake(ctx)
			if wakeErr != nil {
				return "", wakeErr
			}
			base = "https://" + net.JoinHostPort(ip, "3000") + "/api/instances/" + url.PathEscape(id)
			observedAt, lastWake = time.Time{}, time.Now()
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
	targets := map[string]wakeTarget{}
	flags.Func("route", "[PROTOCOL://]listen-address=instance-id[/catalog-port] (repeatable; private/tailnet only)", func(value string) error {
		listen, id, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(id) == "" {
			return fmt.Errorf("route must be listen-address=instance-id")
		}
		protocol := "quake1"
		if scheme, address, ok := strings.Cut(listen, "://"); ok {
			protocol, listen = scheme, address
		}
		target := wakeTarget{id: id, protocol: protocol}
		if base, port, hasPort := strings.Cut(id, "/"); hasPort {
			if strings.TrimSpace(base) == "" {
				return fmt.Errorf("route requires a game ID")
			}
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("invalid catalog port")
			}
			target.id, target.port = base, n
		}
		targets[id] = target
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
		return wakeGame(ctx, host, *controllerURL, token, targets[id])
	})
	if err != nil {
		return err
	}
	for _, mapping := range mappings {
		fmt.Fprintf(out, "Wake gateway %s://%s -> %s\n", mapping.Protocol, mapping.Listen, mapping.InstanceID)
	}
	return gateway.Serve(ctx)
}
