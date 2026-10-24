package command

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/fogo-sh/insta-game/internal/cloud"
	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/lobby"
)

// Run against an installed isolated fixture containing one stopped, allowlisted BZFlag instance.
func TestAWSLobbyColdStart(t *testing.T) {
	path := os.Getenv("INSTA_GAME_AWS_LOBBY_CONFIG")
	if path == "" {
		t.Skip("set INSTA_GAME_AWS_LOBBY_CONFIG and INSTA_GAME_AWS_LOBBY_KEY_FILE for live AWS validation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	config, err := cloud.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if stackShortName(config.Stack) != "insta-game-lambda-check" {
		t.Fatal("refusing a deployment outside the isolated check stack")
	}
	if config.LobbyURL == "" {
		t.Fatal("fixture has no launch page")
	}
	key, err := os.ReadFile(os.Getenv("INSTA_GAME_AWS_LOBBY_KEY_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	host, err := cloud.NewHost(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if t.Failed() {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if err := host.Stop(cleanup); err != nil {
				t.Logf("stop failed fixture: %v", err)
			}
		}
	}()
	run := func(action, ip string, args ...string) []byte {
		t.Helper()
		var out bytes.Buffer
		flags := append([]string{action, "--url", "https://" + ip + ":3000", "--token-file", config.TokenPath}, args...)
		if err := Run(ctx, flags, &out); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	state, err := host.Inspect(ctx)
	if err != nil || state.State != "running" {
		t.Fatalf("fixture must initially be running: %+v %v", state, err)
	}
	var instances []control.Instance
	if err := json.Unmarshal(run("list", state.PublicIP), &instances); err != nil {
		t.Fatal(err)
	}
	var before control.Instance
	for _, instance := range instances {
		if instance.Desired == "deleted" && instance.Phase == "deleted" {
			continue
		}
		if instance.Desired != "stopped" || instance.Phase != "stopped" {
			t.Fatalf("fixture instance %s must be stopped", instance.ID)
		}
		if instance.ID == "bzflag" {
			before = instance
		}
	}
	if before.ID == "" || before.Game.Protocol != "bzflag" {
		t.Fatal("fixture must contain stopped bzflag")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	call := func(method, path, token string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(config.LobbyURL, "/")+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body
	}
	if err := host.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if code, _ := call("GET", "/", ""); code != 200 {
		t.Fatal(code)
	}
	if code, _ := call("POST", "/api/games/bzflag/start", "wrong"); code != 401 {
		t.Fatal(code)
	}
	if code, _ := call("POST", "/api/games/not-allowed/start", strings.TrimSpace(string(key))); code != 404 {
		t.Fatal(code)
	}
	if state, err := host.Inspect(ctx); err != nil || state.State != "stopped" {
		t.Fatal("unauthorized request woke the host", err)
	}
	began := time.Now()
	if code, body := call("POST", "/api/games/bzflag/start", strings.TrimSpace(string(key))); code != 202 {
		t.Fatalf("start: %d %s", code, body)
	}
	if code, _ := call("POST", "/api/games/bzflag/start", strings.TrimSpace(string(key))); code != 409 {
		t.Fatal("duplicate start was accepted", code)
	}
	waitReady := func() string {
		t.Helper()
		var ready struct {
			Host  string `json:"host"`
			Games []struct {
				ID, Phase, Desired, Address string
				Pending                     bool
				Players                     *int
			} `json:"games"`
		}
		address := ""
		for deadline := time.Now().Add(6 * time.Minute); time.Now().Before(deadline); {
			code, body := call("GET", "/api/games", strings.TrimSpace(string(key)))
			if code != 200 {
				t.Fatalf("status: %d %s", code, body)
			}
			if err := json.Unmarshal(body, &ready); err != nil {
				t.Fatal(err)
			}
			for _, game := range ready.Games {
				if game.ID == "bzflag" && game.Phase == "running" && !game.Pending && game.Players != nil {
					address = game.Address
				}
			}
			if address != "" {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(3 * time.Second):
			}
		}
		if address == "" {
			t.Fatal("cold game never became ready")
		}
		return address
	}
	address := waitReady()
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	greeting := make([]byte, 8)
	_, err = io.ReadFull(connection, greeting)
	connection.Close()
	if err != nil || string(greeting) != "BZFS0221" {
		t.Fatalf("public game connection: %q %v", greeting, err)
	}
	state, err = host.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var after control.Instance
	if err := json.Unmarshal(run("show", state.PublicIP, "bzflag"), &after); err != nil {
		t.Fatal(err)
	}
	if !before.CreatedAt.Equal(after.CreatedAt) || before.Config != after.Config {
		t.Fatal("saved game did not survive host stop/start")
	}
	t.Logf("cold launch ready after %s; direct BZFlag greeting verified at %s", time.Since(began).Round(time.Second), address)
	run("stop", state.PublicIP, "bzflag")
	for deadline := time.Now().Add(4 * time.Minute); ; {
		state, err = host.Inspect(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if state.State == "stopped" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("empty host did not shut down")
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
	if code, _ := call("GET", "/", ""); code != 200 {
		t.Fatal("website disappeared after idle shutdown", code)
	}
	_, body := call("GET", "/api/games", strings.TrimSpace(string(key)))
	if strings.Contains(string(body), address) {
		t.Fatal("sleeping host advertised an old connection address")
	}
	t.Log("idle shutdown retained the game; launch page remained available")

	// Simulate a request saved before its caller lost the initial wake attempt.
	// Only the deployed scheduled Lambda can wake this host; GET requests never do.
	sdk, err := cloud.LoadSDK(ctx, config.Region)
	if err != nil {
		t.Fatal(err)
	}
	table := &lobby.Table{Client: dynamodb.NewFromConfig(sdk), Name: config.LobbyTable}
	if err := table.Request(ctx, "bzflag", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	retryBegan := time.Now()
	address = waitReady()
	state, err = host.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scheduled retry recovered a saved start after %s at %s", time.Since(retryBegan).Round(time.Second), address)
	run("stop", state.PublicIP, "bzflag")
	if err := host.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
