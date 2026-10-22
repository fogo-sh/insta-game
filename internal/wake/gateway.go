// Package wake holds NetQuake clients before sign-on and relays their game connection.
package wake

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	attemptTimeout  = 3 * time.Minute
	failureCooldown = time.Minute
	readyTTL        = 5 * time.Second
	maxSessions     = 64
)

type Mapping struct{ Listen, GameID string }

type gameState struct {
	done    chan struct{}
	address string
	err     error
	expires time.Time
}

type listener struct {
	conn     net.PacketConn
	gameID   string
	sessions map[netip.AddrPort]*session
}

type Gateway struct {
	mappings []Mapping
	start    func(context.Context, string) (string, error)
	mu       sync.Mutex
	games    map[string]*gameState
	clients  int
	work     sync.WaitGroup
}

// New configures a gateway for one Serve call. The callback must honor cancellation
// and return an address only after the game is ready.
func New(mappings []Mapping, start func(context.Context, string) (string, error)) (*Gateway, error) {
	if len(mappings) == 0 || start == nil {
		return nil, fmt.Errorf("wake requires routes and a start callback")
	}
	seen := map[string]bool{}
	for _, m := range mappings {
		if strings.TrimSpace(m.GameID) == "" || m.Listen == "" || seen[m.Listen] {
			return nil, fmt.Errorf("wake requires unique addresses and nonempty game IDs")
		}
		if _, err := net.ResolveUDPAddr("udp", m.Listen); err != nil {
			return nil, err
		}
		seen[m.Listen] = true
	}
	return &Gateway{mappings: append([]Mapping(nil), mappings...), start: start, games: map[string]*gameState{}}, nil
}

func (g *Gateway) Serve(ctx context.Context) error {
	var listeners []*listener
	for _, m := range g.mappings {
		conn, err := (&net.ListenConfig{}).ListenPacket(ctx, "udp", m.Listen)
		if err != nil {
			for _, l := range listeners {
				l.conn.Close()
			}
			return fmt.Errorf("listen %s: %w", m.Listen, err)
		}
		listeners = append(listeners, &listener{conn: conn, gameID: m.GameID, sessions: map[netip.AddrPort]*session{}})
	}
	return g.serve(ctx, listeners)
}

func (g *Gateway) serve(ctx context.Context, listeners []*listener) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	failures := make(chan error, len(listeners))
	var readers sync.WaitGroup
	for _, l := range listeners {
		readers.Go(func() { failures <- g.read(runCtx, l); cancel() })
	}
	<-runCtx.Done()
	for _, l := range listeners {
		l.conn.Close()
	}
	readers.Wait()
	g.work.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return <-failures
}

func (g *Gateway) read(ctx context.Context, l *listener) error {
	packet := make([]byte, 65535)
	for {
		n, peer, err := l.conn.ReadFrom(packet)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		address, err := netip.ParseAddrPort(peer.String())
		if err != nil {
			continue
		}
		incoming := packet[:n]
		if controlRequest(incoming, 2) {
			g.mu.Lock()
			count := len(l.sessions)
			g.mu.Unlock()
			payload := append([]byte(l.conn.LocalAddr().String()+"\x00insta-game: "+l.gameID+"\x00on-demand\x00"), byte(count), maxSessions, 3)
			l.conn.WriteTo(controlPacket(0x83, payload), peer)
			continue
		}
		connect := controlRequest(incoming, 1)
		if !connect {
			if _, _, ok := frame(incoming); !ok {
				continue
			}
		}
		g.mu.Lock()
		s := l.sessions[address]
		created := false
		if s == nil && connect && g.clients < maxSessions {
			s = &session{gateway: g, listener: l, peer: peer, address: address, input: make(chan []byte, 8)}
			l.sessions[address] = s
			g.clients++
			created = true
		}
		g.mu.Unlock()
		if connect {
			if s == nil {
				l.conn.WriteTo(rejection("Gateway full; try again shortly."), peer)
				continue
			}
			_, port, _ := net.SplitHostPort(l.conn.LocalAddr().String())
			number, _ := strconv.Atoi(port)
			if _, err := l.conn.WriteTo(accept(number), peer); err != nil {
				if created {
					g.remove(s)
				}
				log.Printf("wake reply: %v", err)
				continue
			}
			if created {
				g.work.Go(func() {
					defer g.remove(s)
					if err := s.run(ctx); err != nil && ctx.Err() == nil {
						log.Printf("wake %s %s: %v", l.gameID, address, err)
					}
				})
			}
			continue
		}
		if s != nil {
			select {
			case s.input <- append([]byte(nil), incoming...):
			default:
			}
		}
	}
}

func (g *Gateway) resolve(ctx, parent context.Context, gameID string) (string, error) {
	g.mu.Lock()
	state := g.games[gameID]
	if state == nil || (!state.expires.IsZero() && time.Now().After(state.expires)) {
		state = &gameState{done: make(chan struct{})}
		g.games[gameID] = state
		g.work.Go(func() {
			wakeCtx, cancel := context.WithTimeout(parent, attemptTimeout)
			defer cancel()
			address, err := g.start(wakeCtx, gameID)
			if wakeCtx.Err() != nil {
				err = wakeCtx.Err()
			}
			if err == nil {
				parsed, parseErr := netip.ParseAddrPort(address)
				if parseErr != nil || parsed.Port() == 0 || parsed.Addr().IsUnspecified() {
					err = fmt.Errorf("invalid backend address")
				} else {
					address = netip.AddrPortFrom(parsed.Addr().Unmap(), parsed.Port()).String()
				}
			}
			g.mu.Lock()
			state.address, state.err = address, err
			state.expires = time.Now().Add(readyTTL)
			if err != nil {
				state.expires = time.Now().Add(failureCooldown)
			}
			close(state.done)
			g.mu.Unlock()
		})
	}
	g.mu.Unlock()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-state.done:
		return state.address, state.err
	}
}

func (g *Gateway) remove(s *session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(s.listener.sessions, s.address)
	g.clients--
}
