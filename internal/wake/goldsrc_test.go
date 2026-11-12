package wake

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestGoldSrcWaitThenSteamHandshake(t *testing.T) {
	backend := listenUDP(t)
	ready := make(chan struct{})
	var starts atomic.Int32
	_, front, _ := serveGateway(t, "goldsrc", func(ctx context.Context, _ string) (string, error) {
		starts.Add(1)
		select {
		case <-ready:
			return backend.LocalAddr().String(), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	client := listenUDP(t)
	sendUDP(t, client, front, []byte("\xff\xff\xff\xffTSource Engine Query\x00"))
	expectSilence(t, client)
	if starts.Load() != 0 {
		t.Fatal("browser query started the backend")
	}
	handshake := []byte("\xff\xff\xff\xffgetchallenge steam\n")
	sendUDP(t, client, front, handshake)
	expectPacket(t, client, []byte("\xff\xff\xff\xffA00000000 0 2\n\x00"))
	holding := []byte("\xff\xff\xff\xffconnect 48 0 \"\\unique\\-1\\prot\\2\" \"\\name\\test\"\nopaque-legacy-certificate")
	sendUDP(t, client, front, holding)
	expectSilence(t, backend)
	close(ready)
	peer := expectPacket(t, backend, handshake)
	// A reply already in flight when readiness changes must not reach the server.
	sendUDP(t, client, front, holding)
	expectSilence(t, backend)
	challenge := []byte("\xff\xff\xff\xffA00000000 123 3 90071996842377217 1\n\x00")
	sendUDP(t, backend, peer, challenge)
	expectPacket(t, client, challenge)
	ticket := []byte("\xff\xff\xff\xffconnect 48 123 \"\\prot\\3\" \"\\name\\test\"\n\x00\xffopaque-ticket")
	sendUDP(t, client, front, ticket)
	expectPacket(t, backend, ticket)
	rejection := []byte("\xff\xff\xff\xff9STEAM validation rejected\n\x00")
	sendUDP(t, backend, peer, rejection)
	expectPacket(t, client, rejection)
	gameplay := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	sendUDP(t, client, front, gameplay)
	expectPacket(t, backend, gameplay)
	second := listenUDP(t)
	sendUDP(t, second, front, handshake)
	expectPacket(t, second, []byte("\xff\xff\xff\xffA00000000 0 2\n\x00"))
	secondPeer := expectPacket(t, backend, handshake)
	if secondPeer.String() == peer.String() {
		t.Fatal("clients share the backend socket")
	}
	sendUDP(t, backend, secondPeer, challenge)
	expectPacket(t, second, challenge)
	if starts.Load() != 1 {
		t.Fatalf("duplicate startup: %d", starts.Load())
	}
}

func TestGoldSrcStartupFailureRejectsConnectingClient(t *testing.T) {
	_, front, _ := serveGateway(t, "goldsrc", func(context.Context, string) (string, error) {
		return "", errors.New("provider failed")
	})
	client := listenUDP(t)
	sendUDP(t, client, front, []byte("\xff\xff\xff\xffgetchallenge steam\n"))
	expectPacket(t, client, []byte("\xff\xff\xff\xffA00000000 0 2\n\x00"))
	expectPacket(t, client, []byte("\xff\xff\xff\xff9Unable to start server. Please try again.\n\x00"))
}
