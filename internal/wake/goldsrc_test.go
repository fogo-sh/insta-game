package wake

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestGoldSrcPacketMatchesEngineVector(t *testing.T) {
	// Payload checked against ReHLDS COM_Munge2, including the unmodified tail.
	want, _ := hex.DecodeString("01000080020000803a774c097c6e79250a00")
	if got := goldsrcPacket(0x80000001, 0x80000002, []byte("\x08Loading\n\x00")); !bytes.Equal(got, want) {
		t.Fatalf("packet %x, want %x", got, want)
	}
}

func TestGoldSrcLoadingThenAuthenticatedHandshakeRelay(t *testing.T) {
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
	accepted, _ := receiveUDP(t, client)
	if !bytes.HasPrefix(accepted, []byte("\xff\xff\xff\xffB ")) {
		t.Fatalf("loading accept: %x", accepted)
	}
	ack := make([]byte, 8)
	loading, _ := receiveUDP(t, client)
	if binary.LittleEndian.Uint32(loading) != 0x80000001 || binary.LittleEndian.Uint32(loading[4:]) != 0 {
		t.Fatalf("loading header: %x", loading)
	}
	binary.LittleEndian.PutUint32(ack, 1)
	binary.LittleEndian.PutUint32(ack[4:], 0x80000001)
	sendUDP(t, client, front, ack)
	close(ready)
	expectPacket(t, client, goldsrcPacket(0x80000002, 1, []byte("\x09retry\n\x00")))
	sendUDP(t, client, front, handshake)
	peer := expectPacket(t, backend, handshake)
	challenge := []byte("\xff\xff\xff\xffA00000000 123 3 90071996842377217 1\n\x00")
	sendUDP(t, backend, peer, challenge)
	expectPacket(t, client, challenge)
	ticket := []byte("\xff\xff\xff\xffconnect 48 123 \"\\prot\\3\" \"\\name\\test\"\n\x00\xffopaque-ticket")
	sendUDP(t, client, front, ticket)
	expectPacket(t, backend, ticket)
	rejection := []byte("\xff\xff\xff\xff9STEAM validation rejected\n\x00")
	sendUDP(t, backend, peer, rejection)
	expectPacket(t, client, rejection)
	second := listenUDP(t)
	sendUDP(t, second, front, handshake)
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

func TestGoldSrcIdleLoadingReleasesSlotAndPreservesWake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ready := make(chan struct{})
		var startup context.Context
		g, err := New([]Mapping{{Listen: "127.0.0.1:27015", InstanceID: "saved", Protocol: "goldsrc"}}, func(ctx context.Context, _ string) (string, error) {
			startup = ctx
			select {
			case <-ready:
				return "127.0.0.1:27016", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		peer := netip.MustParseAddrPort("127.0.0.1:12345")
		l := &listener{conn: discardLoadingPackets{}, protocol: "goldsrc", instanceID: "saved", sessions: map[netip.AddrPort]*session{}}
		s := &session{gateway: g, listener: l, address: peer, input: make(chan []byte, 1)}
		l.sessions[peer], g.clients = s, 1
		done := make(chan error, 1)
		g.work.Go(func() { defer g.remove(s); done <- s.goldsrc(ctx, []byte("\xff\xff\xff\xffgetchallenge steam")) })
		synctest.Wait()
		time.Sleep(clientTimeout + 2*time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("idle wait succeeded")
			}
		default:
			t.Fatal("idle client retained a slot")
		}
		if g.clients != 0 || len(l.sessions) != 0 || startup == nil || startup.Err() != nil {
			t.Fatal("idle cleanup cancelled shared wake or leaked admission")
		}
		close(ready)
		g.work.Wait()
	})
}
