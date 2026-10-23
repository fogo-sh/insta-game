package command

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

func TestWakeLocalControllerStartsSavedGameAndSelectsCatalogPort(t *testing.T) {
	var starts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-token" {
			t.Error("missing controller authorization")
		}
		if r.Method == http.MethodPost {
			starts.Add(1)
			return
		}
		desired, phase := "stopped", "stopped"
		if starts.Load() > 0 {
			desired, phase = "running", "running"
		}
		players := 0
		json.NewEncoder(w).Encode(control.Instance{ID: "saved", Game: control.Game{Protocol: "quake3", GamePort: 27960}, Desired: desired, Phase: phase, UpdatedAt: time.Now(), Players: &players, Endpoints: []control.Endpoint{{Port: 27960, Protocol: "udp", Address: "127.0.0.1:41000"}, {Port: 27961, Protocol: "udp", Address: "127.0.0.1:41001"}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	address, err := wakeGame(ctx, nil, server.URL, "private-token", "saved/27961", "quake3")
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:41001" || starts.Load() != 1 {
		t.Fatalf("address=%s starts=%d", address, starts.Load())
	}
	if _, err := wakeGame(ctx, nil, server.URL, "private-token", "saved", "quake2"); err == nil {
		t.Fatal("wrong protocol was accepted")
	}
}

func TestWakeMissingGameFailsWithoutRetry(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := wakeGame(ctx, nil, server.URL, "token", "missing", "quake1")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
