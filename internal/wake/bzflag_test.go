package wake

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBZFlagUDPBindingBelongsToOneTCPClient(t *testing.T) {
	address := netip.MustParseAddrPort("127.0.0.1:1234")
	first := &bzClient{accepted: true, ip: address.Addr(), udpAddr: address, input: make(chan []byte, 1)}
	second := &bzClient{accepted: true, ip: address.Addr(), input: make(chan []byte, 1)}
	l := &listener{bzClients: map[byte]*bzClient{0: first, 1: second}}
	g := &Gateway{}
	g.bzflagUDP(l, address, net.UDPAddrFromAddrPort(address), bzPacket(bzMsgUDPLinkRequest, []byte{1}))
	if second.udpAddr.IsValid() || len(second.input) != 0 {
		t.Fatal("one UDP endpoint claimed two TCP clients")
	}
	g.bzflagUDP(l, address, net.UDPAddrFromAddrPort(address), bzPacket(bzMsgUDPLinkRequest, []byte{0}))
	if len(first.input) != 1 {
		t.Fatal("the owner could not repeat its UDP handshake")
	}
}

func TestBZFlagBackendGreetingAndIndependentUDPPort(t *testing.T) {
	for _, test := range []struct{ name, greeting, rejection string }{
		{"accept", bzVersion + "\x07", ""},
		{"full", bzVersion + "\xff", "Server is full"},
		{"version", "BZFS9999", "Unsupported BZFlag server protocol"},
		{"replay", bzVersion + "\xc8", "replay servers are not supported"},
		{"banned", "REFUSED: test ban\x00", "Server rejected connection: test ban"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			type result struct {
				backend *bzBackend
				err     error
			}
			done := make(chan result, 1)
			go func() {
				backend, err := bzConnect(ctx, server.Addr().String())
				done <- result{backend, err}
			}()
			server.(*net.TCPListener).SetDeadline(time.Now().Add(2 * time.Second))
			conn, err := server.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			greeting := make([]byte, 10)
			if _, err := io.ReadFull(conn, greeting); err != nil || string(greeting) != "BZFLAG\r\n\r\n" {
				t.Fatalf("client greeting %q: %v", greeting, err)
			}
			// An unrelated UDP listener may already occupy the TCP source port.
			if test.rejection == "" {
				peer := conn.RemoteAddr().(*net.TCPAddr)
				occupied, err := net.ListenUDP("udp", &net.UDPAddr{IP: peer.IP, Port: peer.Port})
				if err != nil {
					t.Fatal(err)
				}
				defer occupied.Close()
			}
			io.WriteString(conn, test.greeting)
			got := <-done
			if test.rejection != "" {
				var rejection *backendRejection
				if !errors.As(got.err, &rejection) || !strings.Contains(rejection.reason, test.rejection) {
					t.Fatalf("rejection = %v, want %q", got.err, test.rejection)
				}
				return
			}
			if got.err != nil {
				t.Fatal(got.err)
			}
			got.backend.tcp.Close()
			got.backend.udp.Close()
		})
	}
}

func TestBZFlagColdClientsKeepTheirSlotsAcrossTCPAndUDP(t *testing.T) {
	backend := listenUDP(t)
	server, err := net.Listen("tcp", backend.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	front := listenUDP(t)
	socket, err := net.Listen("tcp", front.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	var starts atomic.Int32
	g, err := New([]Mapping{{Listen: front.LocalAddr().String(), InstanceID: "tanks", Protocol: "bzflag"}}, func(ctx context.Context, _ string) (string, error) {
		starts.Add(1)
		select {
		case <-ready:
			return backend.LocalAddr().String(), nil
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
		done <- g.serve(ctx, []*listener{{conn: front, tcp: socket, protocol: "bzflag", instanceID: "tanks", sessions: map[netip.AddrPort]*session{}}})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("BZFlag workers did not stop")
		}
	}()
	stranger := listenUDP(t)
	sendUDP(t, stranger, front.LocalAddr().(*net.UDPAddr), bzPacket(bzMsgUDPLinkRequest, []byte{0}))
	expectSilence(t, stranger)
	if starts.Load() != 0 {
		t.Fatal("unattached UDP started the server")
	}
	// Graphical, admin, and query clients use different first messages.
	initial := [][]byte{bzPacket(bzMsgNegotiateFlags, []byte{'G', 'M'}), bzPacket(bzMsgEnter, make([]byte, 247)), bzPacket(bzMsgQueryGame, nil)}
	clients := make([]net.Conn, len(initial))
	udpClients := make([]*net.UDPConn, len(initial))
	for i := range clients {
		clients[i], err = net.Dial("tcp", socket.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer clients[i].Close()
		clients[i].SetDeadline(time.Now().Add(3 * time.Second))
		io.WriteString(clients[i], "BZFLAG\r\n\r\n")
		var hello [9]byte
		if _, err := io.ReadFull(clients[i], hello[:]); err != nil {
			t.Fatal(err)
		}
		if string(hello[:8]) != bzVersion || hello[8] != byte(i) {
			t.Fatalf("unexpected loading greeting: %x", hello)
		}
		clients[i].Write(initial[i])
		clients[i].Write(bzPacket(bzMsgMessage, []byte{byte(i), 'h', 0}))
		udpClients[i] = listenUDP(t)
		sendUDP(t, udpClients[i], front.LocalAddr().(*net.UDPAddr), bzPacket(bzMsgUDPLinkRequest, []byte{byte(i)}))
	}
	close(ready)
	// Read the queued request to identify each independent connection.
	// Backend assignment deliberately differs from both virtual slots.
	for index := range clients {
		server.(*net.TCPListener).SetDeadline(time.Now().Add(3 * time.Second))
		connection, err := server.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		connection.SetDeadline(time.Now().Add(3 * time.Second))
		header := make([]byte, 10)
		if _, err := io.ReadFull(connection, header); err != nil || string(header) != "BZFLAG\r\n\r\n" {
			t.Fatalf("backend greeting: %q %v", header, err)
		}
		actual := byte(7 + index)
		connection.Write(append([]byte(bzVersion), actual))
		request, err := bzRead(connection)
		if err != nil {
			t.Fatal(err)
		}
		virtual := -1
		for i, first := range initial {
			if bytes.Equal(request, first) {
				virtual = i
			}
		}
		if virtual < 0 {
			t.Fatalf("lost initial request: %x", request)
		}
		v := byte(virtual)
		if p, err := bzRead(connection); err != nil || !bytes.Equal(p, bzPacket(bzMsgMessage, []byte{actual, 'h', 0})) {
			t.Fatalf("queued packet IDs were not translated: %x %v", p, err)
		}
		udpPeer := expectPacket(t, backend, bzPacket(bzMsgUDPLinkRequest, []byte{actual}))
		sendUDP(t, backend, udpPeer, bzPacket(bzMsgUDPLinkEstablished, nil))
		expectPacket(t, udpClients[virtual], bzPacket(bzMsgUDPLinkEstablished, nil))
		connection.Write(bzPacket(bzMsgMessage, []byte{v, actual, 0, 'h', 0}))
		if p, err := bzRead(clients[virtual]); err != nil || !bytes.Equal(p, bzPacket(bzMsgMessage, []byte{actual, v, 0, 'h', 0})) {
			t.Fatalf("TCP IDs were not permuted: %x %v", p, err)
		}
		clients[virtual].Write(bzPacket(bzMsgMessage, []byte{actual, 'h', 0}))
		if p, err := bzRead(connection); err != nil || !bytes.Equal(p, bzPacket(bzMsgMessage, []byte{v, 'h', 0})) {
			t.Fatalf("outgoing recipient was not permuted: %x %v", p, err)
		}
		update := bzPacket(bzMsgPlayerUpdateSmall, []byte{0, 0, 0, 0, actual, 5})
		shot := bzPacket(bzMsgShotEnd, []byte{v, 0, 1, 0, 0})
		sendUDP(t, backend, udpPeer, append(update, shot...))
		want := append(bzPacket(bzMsgPlayerUpdateSmall, []byte{0, 0, 0, 0, v, 5}), bzPacket(bzMsgShotEnd, []byte{actual, 0, 1, 0, 0})...)
		expectPacket(t, udpClients[virtual], want)
		// A second UDP port cannot take over a TCP client's established link.
		sendUDP(t, stranger, front.LocalAddr().(*net.UDPAddr), bzPacket(bzMsgUDPLinkRequest, []byte{v}))
		expectSilence(t, backend)
	}
	if starts.Load() != 1 {
		t.Fatalf("concurrent clients started %d servers", starts.Load())
	}
}

func TestBZFlagEmbeddedPlayerIDsAndMalformedPackets(t *testing.T) {
	for _, test := range []struct {
		name    string
		code    uint16
		payload []byte
		offsets []int
	}{
		{"flag owners", bzMsgFlagUpdate, append([]byte{0, 2}, make([]byte, 114)...), []int{10, 67}},
		{"flag transfer", bzMsgTransferFlag, make([]byte, 59), []int{0, 1, 10}},
		{"guided missile", bzMsgGMUpdate, make([]byte, 34), []int{0, 33}},
		{"scores", bzMsgScore, append([]byte{2}, make([]byte, 14)...), []int{1, 8}},
		{"admin addresses", bzMsgAdminInfo, []byte{2, 5, 0, 4, 127, 0, 0, 1, 5, 0, 4, 127, 0, 0, 1}, []int{2, 9}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for i, offset := range test.offsets {
				test.payload[offset] = []byte{3, 7}[i%2]
			}
			p := bzPacket(test.code, test.payload)
			original := append([]byte(nil), p...)
			if err := bzTranslate(p, 3, 7, true); err != nil {
				t.Fatal(err)
			}
			for i, offset := range test.offsets {
				if p[4+offset] != []byte{7, 3}[i%2] {
					t.Fatalf("ID at %d was not swapped", offset)
				}
			}
			if err := bzTranslate(p, 3, 7, true); err != nil || !bytes.Equal(p, original) {
				t.Fatal("ID permutation did not round-trip")
			}
		})
	}
	for _, p := range [][]byte{bzPacket(0xffff, nil), bzPacket(bzMsgReplayReset, []byte{1}), bzPacket(bzMsgFlagUpdate, []byte{0, 2}), bzPacket(bzMsgGMUpdate, make([]byte, 33)), bzPacket(bzMsgAdminInfo, []byte{1, 255})} {
		if err := bzTranslate(p, 3, 7, true); err == nil {
			t.Fatalf("accepted malformed or unsupported packet %x", p)
		}
	}
}
