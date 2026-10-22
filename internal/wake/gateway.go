// Package wake handles NetQuake connection attempts without proxying gameplay.
package wake

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	attemptTimeout  = 3 * time.Minute
	failureCooldown = time.Minute
	readyTTL        = 5 * time.Second
	startingMessage = "Server starting; retry shortly."
	failedMessage   = "Server unavailable; retry in a minute."
)

type Mapping struct {
	Listen string
	GameID string
}

type gameState struct {
	pending     bool
	address     string
	nextAttempt time.Time
}

type listener struct {
	conn   net.PacketConn
	gameID string
}

type Gateway struct {
	mappings []Mapping
	start    func(context.Context, string) (string, error)
	mu       sync.Mutex
	games    map[string]*gameState
	attempts sync.WaitGroup
}

// New configures listeners without opening sockets. The callback must honor its
// context and return a direct IP:port only after the saved game is ready.
func New(mappings []Mapping, start func(context.Context, string) (string, error)) (*Gateway, error) {
	if len(mappings) == 0 || start == nil {
		return nil, fmt.Errorf("wake requires mappings and a start callback")
	}
	seen := make(map[string]bool)
	games := make(map[string]*gameState)
	for _, mapping := range mappings {
		if strings.TrimSpace(mapping.GameID) == "" || mapping.Listen == "" || seen[mapping.Listen] {
			return nil, fmt.Errorf("wake requires unique listen addresses and nonempty game IDs")
		}
		if _, err := net.ResolveUDPAddr("udp", mapping.Listen); err != nil {
			return nil, fmt.Errorf("wake listen %q: %w", mapping.Listen, err)
		}
		seen[mapping.Listen] = true
		games[mapping.GameID] = &gameState{}
	}
	return &Gateway{mappings: append([]Mapping(nil), mappings...), start: start, games: games}, nil
}

// Serve binds every configured address and serves until cancellation or a socket
// failure. Call it once per Gateway. Cancellation also stops in-flight callbacks.
func (g *Gateway) Serve(ctx context.Context) error {
	listeners, err := g.listen(ctx)
	if err != nil {
		return err
	}
	return g.serve(ctx, listeners)
}

func (g *Gateway) listen(ctx context.Context) ([]listener, error) {
	var listeners []listener
	for _, mapping := range g.mappings {
		conn, err := (&net.ListenConfig{}).ListenPacket(ctx, "udp", mapping.Listen)
		if err != nil {
			for _, listener := range listeners {
				listener.conn.Close()
			}
			return nil, fmt.Errorf("wake listen %s: %w", mapping.Listen, err)
		}
		listeners = append(listeners, listener{conn: conn, gameID: mapping.GameID})
	}
	return listeners, nil
}

func (g *Gateway) serve(ctx context.Context, listeners []listener) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	failures := make(chan error, len(listeners))
	var readers sync.WaitGroup
	for _, listener := range listeners {
		readers.Go(func() {
			failures <- g.read(runCtx, listener)
			cancel()
		})
	}
	<-runCtx.Done()
	for _, listener := range listeners {
		listener.conn.Close()
	}
	readers.Wait()
	g.attempts.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return <-failures
}

func (g *Gateway) read(ctx context.Context, listener listener) error {
	packet := make([]byte, 1025)
	for {
		n, peer, err := listener.conn.ReadFrom(packet)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !connectRequest(packet[:n]) {
			continue
		}
		// CCREP_REJECT is explanatory text, never an ACCEPT or transparent redirect.
		message := g.connect(ctx, listener.gameID)
		if _, err := listener.conn.WriteTo(rejection(message), peer); err != nil {
			log.Printf("wake reply: %v", err)
		}
	}
}

func (g *Gateway) connect(ctx context.Context, gameID string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.games[gameID]
	if state.pending {
		return startingMessage
	}
	if time.Now().Before(state.nextAttempt) {
		if state.address != "" {
			return "connect " + state.address
		}
		return failedMessage
	}
	state.pending, state.address = true, ""
	g.attempts.Go(func() {
		wakeCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		defer cancel()
		address, err := g.start(wakeCtx, gameID)
		if wakeCtx.Err() != nil {
			err = wakeCtx.Err()
		}
		if err == nil {
			parsed, parseErr := netip.ParseAddrPort(address)
			if parseErr != nil || parsed.Port() == 0 || parsed.Addr().IsUnspecified() {
				err = fmt.Errorf("wake returned an invalid direct address")
			}
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		state.pending = false
		state.nextAttempt = time.Now().Add(failureCooldown)
		if err != nil {
			log.Printf("wake %s: %v", gameID, err)
			return
		}
		state.address = address
		state.nextAttempt = time.Now().Add(readyTTL)
	})
	return startingMessage
}

func connectRequest(packet []byte) bool {
	// The optional trailing bytes carry extensions used by clients such as vkQuake.
	return len(packet) >= 12 && len(packet) <= 1024 &&
		binary.BigEndian.Uint32(packet[:4]) == 0x80000000|uint32(len(packet)) &&
		packet[4] == 0x01 && string(packet[5:11]) == "QUAKE\x00" && packet[11] == 3
}

func rejection(message string) []byte {
	packet := make([]byte, 6+len(message))
	binary.BigEndian.PutUint32(packet[:4], 0x80000000|uint32(len(packet)))
	packet[4] = 0x82
	copy(packet[5:], message)
	return packet
}
