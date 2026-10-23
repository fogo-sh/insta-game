package wake

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func listenUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func serveGateway(t *testing.T, start func(context.Context, string) (string, error)) (*Gateway, *net.UDPAddr, context.CancelFunc) {
	t.Helper()
	conn := listenUDP(t)
	g, err := New([]Mapping{{Listen: conn.LocalAddr().String(), InstanceID: "quake"}}, start)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- g.serve(ctx, []*listener{{conn: conn, instanceID: "quake", sessions: map[netip.AddrPort]*session{}}})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("gateway shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("gateway did not release its listeners and workers")
		}
	})
	return g, conn.LocalAddr().(*net.UDPAddr), cancel
}

func sendUDP(t *testing.T, conn *net.UDPConn, peer *net.UDPAddr, packet []byte) {
	t.Helper()
	if _, err := conn.WriteToUDP(packet, peer); err != nil {
		t.Fatal(err)
	}
}

func receiveUDP(t *testing.T, conn *net.UDPConn) ([]byte, *net.UDPAddr) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buffer := make([]byte, 65535)
	n, peer, err := conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatal(err)
	}
	return buffer[:n], peer
}

func expectPacket(t *testing.T, conn *net.UDPConn, want []byte) *net.UDPAddr {
	t.Helper()
	got, peer := receiveUDP(t, conn)
	if !bytes.Equal(got, want) {
		t.Fatalf("packet = %x, want %x", got, want)
	}
	return peer
}

func expectSilence(t *testing.T, conn *net.UDPConn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buffer := make([]byte, 65535)
	n, _, err := conn.ReadFromUDP(buffer)
	if err == nil {
		t.Fatalf("unexpected packet: %x", buffer[:n])
	}
	if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal(err)
	}
}

func transportPacket(flags, sequence uint32, payload ...byte) []byte {
	packet := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(packet, flags|uint32(len(packet)))
	binary.BigEndian.PutUint32(packet[4:], sequence)
	copy(packet[8:], payload)
	return packet
}

func connectClient(t *testing.T, address *net.UDPAddr) *net.UDPConn {
	t.Helper()
	conn := listenUDP(t)
	sendUDP(t, conn, address, backendConnect)
	peer := expectPacket(t, conn, accept(address.Port))
	if peer.String() != address.String() {
		t.Fatalf("ACCEPT source = %s, want stable gateway %s", peer, address)
	}
	return conn
}

func receiveLimbo(t *testing.T, client *net.UDPConn) []byte {
	t.Helper()
	packet, _ := receiveUDP(t, client)
	flags, sequence, ok := frame(packet)
	if !ok || flags != flagData|flagEOM || sequence != 0 || !bytes.Contains(packet[8:], []byte("Starting server")) {
		t.Fatalf("first limbo message = %x", packet)
	}
	return packet
}

func awaitClients(t *testing.T, g *Gateway, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		got := g.clients
		g.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("gateway did not reach %d clients", want)
}

func TestLimboDrainsReliableACKBeforeRelayingBothChannels(t *testing.T) {
	admission, backend := listenUDP(t), listenUDP(t)
	ready := make(chan struct{})
	_, address, _ := serveGateway(t, func(ctx context.Context, _ string) (string, error) {
		select {
		case <-ready:
			return admission.LocalAddr().String(), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	client := connectClient(t, address)
	initial := transportPacket(flagData|flagEOM, 0, 1)
	sendUDP(t, client, address, initial)
	limbo := receiveLimbo(t, client)
	close(ready)

	// Readiness must not splice a backend into an unacknowledged reliable stream.
	expectSilence(t, admission)
	expectPacket(t, client, limbo) // Lost ACK retransmits exactly the same sequence and payload.
	expectSilence(t, admission)
	sendUDP(t, client, address, transportPacket(flagAck, 0))
	peer := expectPacket(t, admission, backendConnect)
	sendUDP(t, admission, peer, accept(backend.LocalAddr().(*net.UDPAddr).Port))
	backendPeer := expectPacket(t, backend, initial)
	if peer.String() != backendPeer.String() {
		t.Fatal("gateway changed client socket after backend negotiated its game port")
	}

	for i, flags := range []uint32{flagData, flagData | flagEOM} {
		sequence := uint32(i)
		payload := bytes.Repeat([]byte{byte(20 + i)}, 900)
		sendUDP(t, backend, peer, transportPacket(flags, sequence, payload...))
		if source := expectPacket(t, client, transportPacket(flags, sequence+1, payload...)); source.String() != address.String() {
			t.Fatal("gameplay packet exposed the backend address")
		}
		sendUDP(t, client, address, transportPacket(flagAck, sequence+1))
		expectPacket(t, backend, transportPacket(flagAck, sequence))
	}
	sendUDP(t, client, address, transportPacket(flagNak, 2))
	expectPacket(t, backend, transportPacket(flagNak, 1))

	// Old loading ACKs disappear, but the client's independent reliable sequence is unchanged.
	sendUDP(t, client, address, transportPacket(flagAck, 0))
	clientData := transportPacket(flagData|flagEOM, 1, 4, 'n', 'a', 'm', 'e', 0)
	sendUDP(t, client, address, clientData)
	expectPacket(t, backend, clientData)
	serverACK := transportPacket(flagAck, 1)
	sendUDP(t, backend, peer, serverACK)
	expectPacket(t, client, serverACK)

	serverSnapshot := transportPacket(flagUnreliable, 57, 1, 0x33, 0x44)
	sendUDP(t, backend, peer, serverSnapshot)
	expectPacket(t, client, serverSnapshot)
	clientMove := transportPacket(flagUnreliable, 9, 3, 0x39, 0, 0, 0)
	sendUDP(t, client, address, clientMove)
	expectPacket(t, backend, clientMove)

	stranger := listenUDP(t)
	sendUDP(t, stranger, peer, transportPacket(flagData|flagEOM, 2, 2))
	expectSilence(t, client)
	sendUDP(t, backend, peer, serverSnapshot)
	expectPacket(t, client, serverSnapshot)
}

func TestClientsShareWakeButHaveIndependentBackendSockets(t *testing.T) {
	backend := listenUDP(t)
	ready := make(chan struct{})
	var calls atomic.Int32
	_, address, _ := serveGateway(t, func(ctx context.Context, _ string) (string, error) {
		calls.Add(1)
		select {
		case <-ready:
			return backend.LocalAddr().String(), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	clients := []*net.UDPConn{connectClient(t, address), connectClient(t, address)}
	for i, client := range clients {
		sendUDP(t, client, address, transportPacket(flagData|flagEOM, 0, bytes.Repeat([]byte{1}, i+1)...))
		receiveLimbo(t, client)
		sendUDP(t, client, address, transportPacket(flagAck, 0))
	}
	close(ready)
	peers := make(map[int]*net.UDPAddr)
	for len(peers) < 2 {
		packet, peer := receiveUDP(t, backend)
		if bytes.Equal(packet, backendConnect) {
			sendUDP(t, backend, peer, accept(0))
			continue
		}
		flags, sequence, ok := frame(packet)
		if !ok || flags != flagData|flagEOM || sequence != 0 || len(packet) < 9 || len(packet) > 10 {
			t.Fatalf("unexpected initial client data: %x", packet)
		}
		peers[len(packet)-9] = peer
	}
	if peers[0].String() == peers[1].String() || calls.Load() != 1 {
		t.Fatalf("backend peers %v; wake calls %d", peers, calls.Load())
	}
	for i, client := range clients {
		message := transportPacket(flagUnreliable, 17, byte(40+i))
		sendUDP(t, backend, peers[i], message)
		expectPacket(t, client, message)
	}
}

func TestDisconnectDoesNotCancelAnotherClientsWake(t *testing.T) {
	started := make(chan context.Context, 1)
	g, address, cancel := serveGateway(t, func(ctx context.Context, _ string) (string, error) {
		started <- ctx
		<-ctx.Done()
		return "", ctx.Err()
	})
	first, second := connectClient(t, address), connectClient(t, address)
	var wakeCtx context.Context
	select {
	case wakeCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("wake callback did not start")
	}
	sendUDP(t, first, address, transportPacket(flagUnreliable, 0, 2))
	awaitClients(t, g, 1)
	select {
	case <-wakeCtx.Done():
		t.Fatal("first disconnect canceled another client's pending wake")
	default:
	}
	receiveLimbo(t, second)
	cancel()
	select {
	case <-wakeCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("gateway shutdown did not cancel shared wake")
	}
}

func TestStatusAndMalformedPacketsDoNotWakeAndSessionsAreBounded(t *testing.T) {
	var calls atomic.Int32
	g, address, _ := serveGateway(t, func(ctx context.Context, _ string) (string, error) {
		calls.Add(1)
		<-ctx.Done()
		return "", ctx.Err()
	})
	queryClient := listenUDP(t)
	for _, packet := range [][]byte{
		[]byte("\xff\xff\xff\xffgetstatus\n"),
		[]byte("\x80\x00\x00\x0b\x01QUAKE\x00\x03"),
		[]byte("\x80\x00\x00\x0c\x01QUAKE\x00\x02"),
		[]byte("\x80\x00\x00\x0c\x01OTHER\x00\x03"),
		backendConnect[:5],
		transportPacket(flagData|flagEOM, 0, 1),
	} {
		sendUDP(t, queryClient, address, packet)
	}
	query := append([]byte(nil), backendConnect...)
	query[4] = 2
	sendUDP(t, queryClient, address, query)
	status, _ := receiveUDP(t, queryClient)
	if len(status) < 8 || status[4] != 0x83 || !bytes.Contains(status, []byte("insta-game: quake\x00")) || !bytes.Equal(status[len(status)-3:], []byte{0, maxSessions, 3}) {
		t.Fatalf("local status = %x", status)
	}
	expectSilence(t, queryClient)
	if calls.Load() != 0 {
		t.Fatal("status query or invalid request woke the host")
	}

	first := connectClient(t, address)
	extended := append(append([]byte(nil), backendConnect...), 1, 33, 0)
	binary.BigEndian.PutUint32(extended, flagControl|uint32(len(extended)))
	sendUDP(t, first, address, extended)
	expectPacket(t, first, accept(address.Port))
	for i := 1; i < maxSessions; i++ {
		connectClient(t, address)
	}
	awaitClients(t, g, maxSessions)
	overflow := listenUDP(t)
	sendUDP(t, overflow, address, backendConnect)
	expectPacket(t, overflow, rejection("Gateway full; try again shortly."))
	if calls.Load() != 1 {
		t.Fatalf("shared wake callbacks = %d", calls.Load())
	}
	sendUDP(t, first, address, transportPacket(flagUnreliable, 0, 2))
	awaitClients(t, g, maxSessions-1)
	sendUDP(t, overflow, address, backendConnect)
	expectPacket(t, overflow, accept(address.Port))
	awaitClients(t, g, maxSessions)
}

func TestFailureCooldownDoesNotLeakProviderErrorsOrRetryAutomatically(t *testing.T) {
	var calls atomic.Int32
	g, address, _ := serveGateway(t, func(context.Context, string) (string, error) {
		calls.Add(1)
		return "", errors.New("private provider detail")
	})
	for range 3 {
		client := connectClient(t, address)
		failure, _ := receiveUDP(t, client)
		flags, _, ok := frame(failure)
		if !ok || flags != flagUnreliable || !bytes.Contains(failure[8:], []byte("Unable to start server")) || bytes.Contains(failure, []byte("private")) || failure[len(failure)-1] != 2 {
			t.Fatalf("failure packet = %x", failure)
		}
		awaitClients(t, g, 0)
	}
	if calls.Load() != 1 {
		t.Fatalf("failed wake retried during cooldown: %d calls", calls.Load())
	}
}

func TestBackendRejectionDisconnectsWithoutRetrying(t *testing.T) {
	proquake := append(accept(26000), 1, 33, 0)
	binary.BigEndian.PutUint32(proquake, flagControl|uint32(len(proquake)))
	for _, test := range []struct {
		name    string
		reply   []byte
		message string
	}{
		{"full", rejection("Server is full"), "Server is full"},
		{"incompatible transport", proquake, "extended transport"},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := listenUDP(t)
			g, address, _ := serveGateway(t, func(context.Context, string) (string, error) {
				return backend.LocalAddr().String(), nil
			})
			client := connectClient(t, address)
			peer := expectPacket(t, backend, backendConnect)
			sendUDP(t, backend, peer, test.reply)
			failure, _ := receiveUDP(t, client)
			flags, _, ok := frame(failure)
			if !ok || flags != flagUnreliable || !bytes.Contains(failure[8:], []byte(test.message)) || failure[len(failure)-1] != 2 {
				t.Fatalf("backend failure packet = %x", failure)
			}
			awaitClients(t, g, 0)
			expectSilence(t, backend)
		})
	}
}
