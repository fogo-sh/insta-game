package wake

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalogUDPRelaysKeepHandshakesAndGameplay(t *testing.T) {
	for _, protocol := range []string{"quake2", "quake3", "xonotic", "ut99"} {
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
			expectSilence(t, client)
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

func TestBZFlagTCPAndUDPShareTheStablePort(t *testing.T) {
	backend := listenUDP(t)
	server, err := net.Listen("tcp", backend.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	tcpDone := make(chan error, 1)
	go func() {
		c, err := server.Accept()
		if err != nil {
			tcpDone <- err
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(5 * time.Second))
		header := make([]byte, 10)
		if _, err = io.ReadFull(c, header); err != nil {
			tcpDone <- err
			return
		}
		if string(header) != "BZFLAG\r\n\r\n" {
			tcpDone <- io.ErrUnexpectedEOF
			return
		}
		_, err = c.Write([]byte("BZFS0221\x01"))
		if err != nil {
			tcpDone <- err
			return
		}
		payload := make([]byte, 4)
		_, err = io.ReadFull(c, payload)
		if err == nil {
			_, err = c.Write(payload)
		}
		tcpDone <- err
	}()
	front := listenUDP(t)
	socket, err := net.Listen("tcp", front.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	var starts atomic.Int32
	g, err := New([]Mapping{{Listen: front.LocalAddr().String(), InstanceID: "tanks", Protocol: "bzflag"}}, func(context.Context, string) (string, error) { starts.Add(1); return backend.LocalAddr().String(), nil })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- g.serve(ctx, []*listener{{conn: front, tcp: socket, protocol: "bzflag", instanceID: "tanks", tcpPeers: map[netip.Addr]int{}, sessions: map[netip.AddrPort]*session{}}})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("TCP relay leaked workers")
		}
	}()
	udp := listenUDP(t)
	link := []byte{0, 1, 0x6f, 0x66, 1}
	sendUDP(t, udp, front.LocalAddr().(*net.UDPAddr), link)
	expectSilence(t, udp)
	if starts.Load() != 0 {
		t.Fatal("unattached UDP woke BZFlag")
	}
	client, err := net.Dial("tcp", socket.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(3 * time.Second))
	client.Write([]byte("BZFLAG\r\n\r\n"))
	greeting := make([]byte, 9)
	if _, err := io.ReadFull(client, greeting); err != nil {
		t.Fatal(err)
	}
	if string(greeting) != "BZFS0221\x01" {
		t.Fatalf("greeting changed: %x", greeting)
	}
	sendUDP(t, udp, front.LocalAddr().(*net.UDPAddr), link)
	peer := expectPacket(t, backend, link)
	sendUDP(t, backend, peer, []byte("UDP reply"))
	expectPacket(t, udp, []byte("UDP reply"))
	client.Write([]byte("play"))
	payload := make([]byte, 4)
	if _, err := io.ReadFull(client, payload); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, []byte("play")) {
		t.Fatal("TCP gameplay changed")
	}
	if err := <-tcpDone; err != nil {
		t.Fatal(err)
	}
}
