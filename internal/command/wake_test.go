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

func TestWakeStartsSavedGameAndSelectsCatalogPortDespiteClockSkew(t *testing.T) {
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
		json.NewEncoder(w).Encode(control.Instance{ID: "saved", Game: control.Game{Protocol: "quake3", GamePort: 27960}, Desired: desired, Phase: phase, UpdatedAt: time.Now().Add(-time.Hour), Players: &players, Endpoints: []control.Endpoint{{Port: 27960, Protocol: "udp", Address: "127.0.0.1:41000"}, {Port: 27961, Protocol: "udp", Address: "127.0.0.1:41001"}}})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	address, err := wakeGame(ctx, nil, server.URL, "private-token", wakeTarget{id: "saved", port: 27961, protocol: "quake3"})
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:41001" || starts.Load() != 1 {
		t.Fatalf("address=%s starts=%d", address, starts.Load())
	}
	if _, err := wakeGame(ctx, nil, server.URL, "private-token", wakeTarget{id: "saved", protocol: "quake2"}); err == nil {
		t.Fatal("wrong protocol was accepted")
	}
}

func TestWakeMissingGameFailsWithoutRetry(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := wakeGame(ctx, nil, server.URL, "token", wakeTarget{id: "missing", protocol: "quake1"})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestWakeRejectedCredentialsFailWithoutRetry(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(http.StatusText(status)+"/"+method, func(t *testing.T) {
				var rejections atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == method {
						rejections.Add(1)
						w.WriteHeader(status)
						return
					}
					json.NewEncoder(w).Encode(control.Instance{Game: control.Game{Protocol: "quake3"}, Desired: "stopped", Phase: "stopped", UpdatedAt: time.Now()})
				}))
				defer server.Close()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, err := wakeGame(ctx, nil, server.URL, "wrong-token", wakeTarget{id: "saved", protocol: "quake3"})
				var response controllerHTTPError
				if !errors.As(err, &response) || int(response) != status || rejections.Load() != 1 {
					t.Fatalf("error=%v rejections=%d", err, rejections.Load())
				}
			})
		}
	}
}
