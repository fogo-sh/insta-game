package command

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

// HTTP, persistence, reconciliation, route parsing, and the gateway run their
// production implementations; infrastructure and the game are test doubles.
func TestWakeGatewayStartsOnlyRequestedSavedGame(t *testing.T) {
	backend, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	go func() {
		packet := make([]byte, 4096)
		for {
			n, peer, err := backend.ReadFrom(packet)
			if err != nil {
				return
			}
			backend.WriteTo(append([]byte("backend:"), packet[:n]...), peer)
		}
	}()
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(control.AgentStatus{Running: true, Ready: true})
	}))
	defer agent.Close()
	store, err := control.OpenStore(filepath.Join(t.TempDir(), "instances.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	provider := &wakeTestProvider{states: map[string]control.Observation{}, agent: agent.URL, address: backend.LocalAddr().String()}
	game := control.Game{ID: "arena", Protocol: "quake3", GamePort: 27960}
	controller := control.NewController(store, []control.Game{game}, map[string]control.Provider{"test": provider}, "test")
	for _, id := range []string{"requested", "untouched"} {
		if _, err := controller.Create(control.CreateRequest{ID: id, Name: id, Game: game.ID}); err != nil {
			t.Fatal(err)
		}
		if err := controller.SetDesired(id, "stopped"); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(controller.Handler("integration-token"))
	defer server.Close()
	t.Setenv("INSTA_GAME_TOKEN", "integration-token")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reconciled := make(chan struct{})
	go func() {
		defer close(reconciled)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			controller.Reconcile(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	defer func() { cancel(); <-reconciled }()
	// Reserve an address before handing it to the real CLI listener.
	reservation, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.LocalAddr().String()
	reservation.Close()
	done := make(chan error, 1)
	go func() {
		done <- wakeCommand(ctx, []string{"--url", server.URL, "--route", "quake3://" + address + "=requested/27960"}, io.Discard)
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	client, err := net.Dial("udp4", address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	handshake := []byte("\xff\xff\xff\xffgetchallenge 123")
	buffer := make([]byte, 4096)
	for {
		client.Write(handshake)
		client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := client.Read(buffer)
		if err == nil {
			if string(buffer[:n]) != "backend:"+string(handshake) {
				t.Fatalf("unexpected handshake: %q", buffer[:n])
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("gateway never reached backend: %v", err)
		}
	}
	// Gameplay keeps the same public socket and reaches the selected backend.
	if _, err := client.Write([]byte("gameplay")); err != nil {
		t.Fatal(err)
	}
	client.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, err := client.Read(buffer)
		if err != nil {
			t.Fatal(err)
		}
		if string(buffer[:n]) == "backend:gameplay" {
			break
		}
		if string(buffer[:n]) != "backend:"+string(handshake) {
			t.Fatalf("unexpected gameplay response: %q", buffer[:n])
		}
	}
	requested, err := store.Get("requested")
	if err != nil {
		t.Fatal(err)
	}
	untouched, err := store.Get("untouched")
	if err != nil {
		t.Fatal(err)
	}
	if requested.Phase != "running" || untouched.Desired != "stopped" || untouched.Connect() != "" {
		t.Fatalf("requested=%+v untouched=%+v", requested, untouched)
	}
}

type wakeTestProvider struct {
	mu             sync.Mutex
	states         map[string]control.Observation
	agent, address string
}

func (p *wakeTestProvider) Ensure(_ context.Context, i control.Instance) (control.Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := control.Observation{Exists: true, AgentURL: p.agent, Endpoints: []control.Endpoint{{Port: 27960, Protocol: "udp", Address: p.address}}}
	p.states[i.ID] = o
	return o, nil
}
func (p *wakeTestProvider) Inspect(_ context.Context, i control.Instance) (control.Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.states[i.ID], nil
}
func (p *wakeTestProvider) Start(_ context.Context, i control.Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.states[i.ID]
	o.Running = true
	o.Started = true
	p.states[i.ID] = o
	return nil
}
func (p *wakeTestProvider) Stop(_ context.Context, i control.Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.states[i.ID]
	o.Running = false
	p.states[i.ID] = o
	return nil
}
func (p *wakeTestProvider) Delete(_ context.Context, i control.Instance) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.states, i.ID)
	return nil
}
