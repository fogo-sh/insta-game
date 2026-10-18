package control

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt in with a catalog whose image is built for this Docker host's architecture.
func TestDockerTwoServersSurviveControllerRestart(t *testing.T) {
	catalog := os.Getenv("INSTA_GAME_DOCKER_CATALOG")
	if catalog == "" {
		t.Skip("set INSTA_GAME_DOCKER_CATALOG to run real Docker/game integration")
	}
	games, err := LoadCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var game Game
	for _, g := range games {
		if g.ID == "xonotic" {
			game = g
		}
	}
	if game.ID == "" {
		t.Fatal("integration catalog must contain xonotic")
	}
	path := filepath.Join(t.TempDir(), "instances.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := NewDocker("/var/run/docker.sock", "localhost", "127.0.0.1")
	controller := NewController(store, games, provider)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	prefix := "smoke-" + NewToken()[:10]
	ids := []string{prefix + "-a", prefix + "-b"}
	defer func() {
		for _, id := range ids {
			if i, err := controller.Store.Get(id); err == nil {
				cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				provider.Delete(cleanup, i)
				cancel()
			}
		}
		controller.Store.Close()
	}()
	for index, id := range ids {
		name := []string{"Friday instagib", "Sunday CTF"}[index]
		config := game.DefaultConfig + "\nhostname \"" + name + "\"\nminplayers 0\nminplayers_per_team 0\n"
		if _, err := controller.Create(CreateRequest{ID: id, Game: game.ID, Name: name, Config: &config}); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(id, phase string) Instance {
		t.Helper()
		for {
			controller.Reconcile(ctx)
			i, err := controller.Store.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			if i.Phase == phase {
				return i
			}
			if ctx.Err() != nil {
				t.Fatalf("waiting for %s=%s: phase=%s error=%s", id, phase, i.Phase, i.Error)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	first := wait(ids[0], "running")
	second := wait(ids[1], "running")
	if first.Connect() == "" || first.Connect() == second.Connect() {
		t.Fatalf("shared or absent endpoint: %q %q", first.Connect(), second.Connect())
	}
	for _, i := range []Instance{first, second} {
		observation, err := provider.Inspect(ctx, i)
		if err != nil {
			t.Fatal(err)
		}

		request, _ := http.NewRequestWithContext(ctx, "GET", observation.AgentURL+"/config", nil)
		request.Header.Set("Authorization", "Bearer "+i.AgentToken)
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		config, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || !strings.Contains(string(config), i.Name) {
			t.Fatalf("configuration crossed instances: status %d wanted %q", res.StatusCode, i.Name)
		}

	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	controller = NewController(reopened, games, provider)
	wait(ids[0], "running")
	wait(ids[1], "running")
	if err := controller.SetDesired(ids[0], "stopped"); err != nil {
		t.Fatal(err)
	}
	wait(ids[0], "stopped")
	other, err := provider.Inspect(ctx, second)
	if err != nil || !other.Running {
		t.Fatalf("stopping first affected second: %v", err)
	}
	if err := controller.SetDesired(ids[0], "running"); err != nil {
		t.Fatal(err)
	}
	wait(ids[0], "running")
	if err := controller.SetDesired(ids[0], "deleted"); err != nil {
		t.Fatal(err)
	}
	wait(ids[0], "deleted")
	observation, err := provider.Inspect(ctx, first)
	if err != nil || observation.Exists {
		t.Fatalf("delete retained container: %v", err)
	}
	other, err = provider.Inspect(ctx, second)
	if err != nil || !other.Running {
		t.Fatalf("deleting first affected second: %v", err)
	}
	t.Logf("Verified separate Xonotic endpoints %s and %s, configuration, recovery, stop/start and deletion", first.Connect(), second.Connect())
}
