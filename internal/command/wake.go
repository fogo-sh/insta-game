package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/securehttp"
	"github.com/fogo-sh/insta-game/internal/wake"
)

// wakeGame waits for the host's authoritative controller before starting a saved game.
func wakeGame(ctx context.Context, host *cloud.Host, token, id string) (string, error) {
	requestedAt := time.Now()
	ip, err := host.Wake(ctx)
	if err != nil {
		return "", err
	}
	transport := &http.Transport{TLSClientConfig: securehttp.ClientTLS(token)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	base := "https://" + net.JoinHostPort(ip, "3000") + "/api/instances/" + url.PathEscape(id)
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
		err = request(http.MethodGet, base, &instance)
		if err == nil && instance.UpdatedAt.After(requestedAt) {
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
				return instance.Connect(), nil
			}
		}
		if err != nil && time.Since(lastWake) >= 10*time.Second {
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
	configPath := flags.String("config", "aws.json", "Shared AWS host configuration")
	var mappings []wake.Mapping
	flags.Func("route", "Quake UDP listen address=existing game ID (repeatable; use private/tailnet addresses)", func(value string) error {
		listen, id, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(id) == "" {
			return fmt.Errorf("route must be listen-address=game-id")
		}
		mappings = append(mappings, wake.Mapping{Listen: listen, GameID: id})
		return nil
	})
	if err := flags.Parse(args); err != nil {
		return flagError(err)
	}
	if len(mappings) == 0 {
		return fmt.Errorf("at least one --route is required")
	}
	config, err := cloud.Load(*configPath)
	if err != nil {
		return err
	}
	token, err := tokenAt(config.TokenPath, false)
	if err != nil {
		return err
	}
	host, err := cloud.NewHost(ctx, config)
	if err != nil {
		return err
	}
	gateway, err := wake.New(mappings, func(ctx context.Context, id string) (string, error) { return wakeGame(ctx, host, token, id) })
	if err != nil {
		return err
	}
	for _, mapping := range mappings {
		fmt.Fprintf(out, "Quake wake gateway %s -> %s (retry for direct connection address)\n", mapping.Listen, mapping.GameID)
	}
	return gateway.Serve(ctx)
}
