package wake

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type discardLoadingPackets struct{ net.PacketConn }

func (discardLoadingPackets) WriteTo(packet []byte, _ net.Addr) (int, error) {
	return len(packet), nil
}

func TestIdleHeldClientReleasesAdmissionWithoutCancelingSharedWake(t *testing.T) {
	for _, protocol := range []string{"quake2", "quake3", "ut99"} {
		t.Run(protocol, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ready := make(chan struct{})
				var wakeCtx context.Context
				starts := 0
				g, err := New([]Mapping{{Listen: "127.0.0.1:26000", InstanceID: "saved", Protocol: protocol}}, func(ctx context.Context, _ string) (string, error) {
					wakeCtx = ctx
					starts++
					select {
					case <-ready:
						return "127.0.0.1:26001", nil
					case <-ctx.Done():
						return "", ctx.Err()
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				peer := netip.MustParseAddrPort("127.0.0.1:12345")
				l := &listener{conn: discardLoadingPackets{}, protocol: protocol, instanceID: "saved", sessions: map[netip.AddrPort]*session{}}
				s := &session{gateway: g, listener: l, address: peer, input: make(chan []byte, 1)}
				l.sessions[peer], g.clients = s, 1
				done := make(chan error, 1)
				g.work.Go(func() {
					defer g.remove(s)
					done <- s.relay(ctx, []byte("initial handshake"))
				})
				synctest.Wait()
				shared := make(chan destination, 1)
				go func() {
					address, err := g.resolve(ctx, ctx, "saved")
					shared <- destination{address, err}
				}()
				time.Sleep(clientTimeout / 2)
				s.input <- []byte("client retransmission")
				synctest.Wait()
				time.Sleep(clientTimeout / 2)
				synctest.Wait()
				select {
				case err := <-done:
					t.Fatalf("client activity did not extend the hold: %v", err)
				default:
				}
				time.Sleep(clientTimeout / 2)
				synctest.Wait()
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("idle hold succeeded without a backend")
					}
				default:
					t.Fatal("inactive client still holds an admission slot")
				}
				if g.clients != 0 || len(l.sessions) != 0 {
					t.Fatal("inactive client's admission was not released")
				}
				if wakeCtx == nil || wakeCtx.Err() != nil {
					t.Fatal("idle client cancelled the shared startup")
				}
				// The relay has exited before readiness, so it cannot send the
				// abandoned handshake when another client's shared wake finishes.
				close(ready)
				result := <-shared
				if result.err != nil || result.address != "127.0.0.1:26001" || starts != 1 {
					t.Fatalf("shared wake did not survive: %+v, starts=%d", result, starts)
				}
				g.work.Wait()
			})
		})
	}
}

func TestCatalogUDPRelaysKeepHandshakesAndGameplay(t *testing.T) {
	for _, protocol := range []string{"quake2", "quake3", "ut99"} {
		t.Run(protocol, func(t *testing.T) {
			backend := listenUDP(t)
			front := listenUDP(t)
			var starts atomic.Int32
			ready := make(chan struct{})
			g, err := New([]Mapping{{Listen: front.LocalAddr().String(), InstanceID: "saved", Protocol: protocol}}, func(ctx context.Context, _ string) (string, error) {
				starts.Add(1)
				select {
				case <-ready:
					return net.JoinHostPort("localhost", strconv.Itoa(backend.LocalAddr().(*net.UDPAddr).Port)), nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- g.serve(ctx, []*listener{{conn: front, protocol: protocol, instanceID: "saved", sessions: map[netip.AddrPort]*session{}}})
			}()
			defer func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(3 * time.Second):
					t.Error("relay leaked workers")
				}
			}()
			address := front.LocalAddr().(*net.UDPAddr)
			client := listenUDP(t)
			query := []byte("\xff\xff\xff\xffgetstatus")
			handshake := []byte("\xff\xff\xff\xffgetchallenge 123")
			if protocol == "ut99" {
				query = []byte("\\status\\")
				handshake = []byte{0x80, 0x01, 0x42, 0x11}
			}
			sendUDP(t, client, address, query)
			expectSilence(t, client)
			if starts.Load() != 0 {
				t.Fatal("browser query started a sleeping game")
			}
			sendUDP(t, client, address, handshake)
			if protocol == "ut99" {
				expectPacket(t, client, []byte{0xff, 0x7f})
			} else {
				expectPacket(t, client, []byte("\xff\xff\xff\xffprint\nStarting server...\n\x00"))
			}
			close(ready)
			peer := expectPacket(t, backend, handshake)
			sendUDP(t, backend, peer, []byte("challenge response"))
			expectPacket(t, client, []byte("challenge response"))
			gameplay := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
			sendUDP(t, client, address, gameplay)
			if got := expectPacket(t, backend, gameplay); got.String() != peer.String() {
				t.Fatal("client backend socket changed")
			}
			second := listenUDP(t)
			sendUDP(t, second, address, handshake)
			if protocol != "ut99" {
				expectPacket(t, second, []byte("\xff\xff\xff\xffprint\nStarting server...\n\x00"))
			}
			secondPeer := expectPacket(t, backend, handshake)
			if secondPeer.String() == peer.String() {
				t.Fatal("clients share a backend socket")
			}
			sendUDP(t, backend, secondPeer, []byte("second player"))
			expectPacket(t, second, []byte("second player"))
			expectSilence(t, client)
			if starts.Load() != 1 {
				t.Fatal("concurrent clients duplicated the wake")
			}
		})
	}
}
