//go:build linux || darwin

package control

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeAgentsRecoverAndKeepSeparateData(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "insta-game")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/insta-game")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build agent: %v: %s", err, output)
	}
	assets := filepath.Join(root, "assets")
	if err := os.MkdirAll(assets, 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf 'started\\n'\nwhile IFS= read -r command; do\n case \"$command\" in quit) exit 0;; *) printf '%s\\n' \"$command\" >> saved.txt;; esac\ndone\n"
	command := filepath.Join(root, "game.sh")
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	game := Game{ID: "test", Protocol: "unsupported", GamePort: 26000, SidecarPort: 5001, GameQuitCmd: "quit", GameQuitTimeout: 1, Ports: map[string]json.RawMessage{"26000/udp": json.RawMessage(`{}`)}}
	n := &Native{Root: filepath.Join(root, "instances"), Binary: binary, PublicHost: "localhost", Games: map[string]NativeGame{"test": {Command: command, Assets: assets, ConfigPath: "server.cfg"}}}
	instances := []Instance{
		{ID: "first", Game: game, AgentToken: NewToken(), RconPassword: NewToken(), Config: "hostname first"},
		{ID: "second", Game: game, AgentToken: NewToken(), RconPassword: NewToken(), Config: "hostname second"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer func() {
		for _, i := range instances {
			cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
			if err := n.Delete(cleanup, i); err != nil {
				t.Errorf("cleanup: %v", err)
			}
			done()
		}
	}()
	wait := func(i Instance, running bool) Observation {
		t.Helper()
		for {
			observed, err := n.Inspect(ctx, i)
			if err == nil && observed.Running == running {
				return observed
			}
			select {
			case <-ctx.Done():
				t.Fatalf("waiting running=%v: %v, %v", running, observed, err)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	for _, i := range instances {
		if _, err := n.Ensure(ctx, i); err != nil {
			t.Fatal(err)
		}
		if err := n.Start(ctx, i); err != nil {
			t.Fatal(err)
		}
		wait(i, true)
	}
	first, second := wait(instances[0], true), wait(instances[1], true)
	if first.Endpoints[0].Address == second.Endpoints[0].Address {
		t.Fatal("shared game endpoint")
	}
	// Reconstructing the provider must discover agents without spawning replacements.
	n = &Native{Root: n.Root, Binary: binary, PublicHost: n.PublicHost, Games: n.Games}
	for _, i := range instances {
		wait(i, true)
	}
	state, err := n.state(instances[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := n.request(ctx, instances[0], state, "POST", "/console", strings.NewReader(`{"command":"saved-by-first"}`), nil); err != nil {
		t.Fatal(err)
	}
	if err := n.Stop(ctx, instances[0]); err != nil {
		t.Fatal(err)
	}
	wait(instances[0], false)
	wait(instances[1], true)
	saved := filepath.Join(n.directory(instances[0]), "work", "saved.txt")
	if data, err := os.ReadFile(saved); err != nil || string(data) != "saved-by-first\n" {
		t.Fatalf("saved data: %q %v", data, err)
	}
	if err := n.Start(ctx, instances[0]); err != nil {
		t.Fatal(err)
	}
	wait(instances[0], true)
	if _, err := os.Stat(saved); err != nil {
		t.Fatal("restart lost data", err)
	}
	wrong := instances[0]
	wrong.AgentToken = NewToken()
	if err := n.Delete(ctx, wrong); err == nil {
		t.Fatal("deleted an unowned native instance")
	}
	if err := n.Delete(ctx, instances[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(n.directory(instances[0])); !os.IsNotExist(err) {
		t.Fatalf("delete left data: %v", err)
	}
	wait(instances[1], true)
}
