package wake

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var connectPacket = []byte("\x80\x00\x00\x0c\x01QUAKE\x00\x03")

func serveGateway(t *testing.T, mappings []Mapping, start func(context.Context, string) (string, error)) (*Gateway, []*net.UDPConn, context.CancelFunc, <-chan error) {
	t.Helper()
	g, err := New(mappings, start)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	listeners, err := g.listen(ctx)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- g.serve(ctx, listeners); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("gateway: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("gateway did not stop")
		}
	})
	clients := make([]*net.UDPConn, 0, len(listeners))
	for _, listener := range listeners {
		client, err := net.DialUDP("udp", nil, listener.conn.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { client.Close() })
		clients = append(clients, client)
	}
	return g, clients, cancel, done
}

func exchange(t *testing.T, client *net.UDPConn, packet []byte) string {
	t.Helper()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write(packet); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 256)
	n, err := client.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if n < 6 || binary.BigEndian.Uint32(buffer[:4]) != 0x80000000|uint32(n) || buffer[4] != 0x82 || buffer[n-1] != 0 {
		t.Fatalf("not a CCREP_REJECT: %x", buffer[:n])
	}
	return string(buffer[5 : n-1])
}

func awaitState(t *testing.T, g *Gateway, game string, check func(gameState) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		state := *g.games[game]
		g.mu.Unlock()
		if check(state) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("wake attempt did not finish")
}

func TestOnlyValidConnectWakesAndShutdownCancelsAttempt(t *testing.T) {
	var calls atomic.Int32
	started := make(chan string, 1)
	canceled := make(chan struct{})
	_, clients, cancel, done := serveGateway(t, []Mapping{{Listen: "127.0.0.1:0", GameID: "quake"}}, func(ctx context.Context, id string) (string, error) {
		calls.Add(1)
		started <- id
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > attemptTimeout {
			t.Error("callback has no bounded deadline")
		}
		<-ctx.Done()
		close(canceled)
		return "", ctx.Err()
	})
	client := clients[0]
	packets := [][]byte{
		[]byte("\xff\xff\xff\xffgetstatus\n"),
		[]byte("\xff\xff\xff\xffgetinfo\n"),
		[]byte("\x80\x00\x00\x0c\x02QUAKE\x00\x03"),
		[]byte("\x80\x00\x00\x06\x03\x00"),
		[]byte("\x80\x00\x00\x06\x04\x00"),
		[]byte("\x80\x00\x00\x0b\x01QUAKE\x00\x03"),
		[]byte("\x80\x00\x00\x0c\x01QUAKE\x00\x02"),
		[]byte("\x80\x00\x00\x0c\x01OTHER\x00\x03"),
		[]byte("\x00\x01\x00\x0c\x01QUAKE\x00\x03"),
		connectPacket[:5],
	}
	for _, packet := range packets {
		if _, err := client.Write(packet); err != nil {
			t.Fatal(err)
		}
	}
	client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := client.Read(make([]byte, 256)); err == nil {
		t.Fatal("query or malformed packet received a connect response")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("query or malformed packet woke the host")
	}
	// vkQuake can append ProQuake extension bytes to the core NetQuake request.
	extended := append(append([]byte(nil), connectPacket...), 1, 30, 0)
	binary.BigEndian.PutUint32(extended[:4], 0x80000000|uint32(len(extended)))
	if got := exchange(t, client, extended); got != startingMessage {
		t.Fatalf("connect response = %q", got)
	}
	select {
	case id := <-started:
		if id != "quake" {
			t.Fatalf("wrong mapping: %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("valid connect did not wake")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("callback context was not canceled")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("UDP listeners did not shut down")
	}
}

func TestConnectAttemptsDeduplicateAndRevalidateDirectAddress(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	g, clients, _, _ := serveGateway(t, []Mapping{
		{Listen: "127.0.0.1:0", GameID: "quake"},
		{Listen: "127.0.0.2:0", GameID: "quake"},
		{Listen: "127.0.0.3:0", GameID: "other"},
	}, func(ctx context.Context, gameID string) (string, error) {
		if gameID == "other" {
			return "192.0.2.3:26000", nil
		}
		attempt := calls.Add(1)
		select {
		case <-release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if attempt == 1 {
			return "192.0.2.1:26000", nil
		}
		return "192.0.2.2:26000", nil
	})
	for range 4 {
		for _, client := range clients[:2] {
			if got := exchange(t, client, connectPacket); got != startingMessage {
				t.Fatalf("pending response = %q", got)
			}
		}
	}
	exchange(t, clients[2], connectPacket)
	awaitState(t, g, "other", func(s gameState) bool { return s.address != "" })
	if got := exchange(t, clients[2], connectPacket); got != "connect 192.0.2.3:26000" {
		t.Fatalf("independent game mapping = %q", got)
	}
	close(release)
	awaitState(t, g, "quake", func(s gameState) bool { return s.address != "" })
	if calls.Load() != 1 {
		t.Fatalf("duplicate callbacks: %d", calls.Load())
	}
	if got := exchange(t, clients[0], connectPacket); got != "connect 192.0.2.1:26000" {
		t.Fatalf("ready response = %q", got)
	}
	g.mu.Lock()
	g.games["quake"].nextAttempt = time.Now().Add(-time.Second)
	g.mu.Unlock()
	if got := exchange(t, clients[1], connectPacket); got != startingMessage {
		t.Fatalf("expired address was reused: %q", got)
	}
	awaitState(t, g, "quake", func(s gameState) bool { return s.address == "192.0.2.2:26000" })
	if got := exchange(t, clients[0], connectPacket); got != "connect 192.0.2.2:26000" {
		t.Fatalf("changed host address = %q", got)
	}
}

func TestFailuresCoolDownWithoutAutomaticRetries(t *testing.T) {
	var calls atomic.Int32
	g, clients, _, _ := serveGateway(t, []Mapping{{Listen: "127.0.0.1:0", GameID: "quake"}}, func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			return "", errors.New("provider unavailable")
		}
		return "not-an-address\nsecret", nil
	})
	exchange(t, clients[0], connectPacket)
	awaitState(t, g, "quake", func(s gameState) bool { return !s.pending && !s.nextAttempt.IsZero() })
	for range 5 {
		if got := exchange(t, clients[0], connectPacket); got != failedMessage {
			t.Fatalf("failure response = %q", got)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("failure bypassed cooldown")
	}
	g.mu.Lock()
	g.games["quake"].nextAttempt = time.Now().Add(-time.Second)
	g.mu.Unlock()
	if calls.Load() != 1 {
		t.Fatal("retried without a new connect request")
	}
	exchange(t, clients[0], connectPacket)
	awaitState(t, g, "quake", func(s gameState) bool { return !s.pending })
	if got := exchange(t, clients[0], connectPacket); got != failedMessage || strings.Contains(got, "secret") {
		t.Fatalf("invalid callback address was advertised: %q", got)
	}
}
