package wake

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestDarkPlacesBackendHandshake(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "accept", true: "reject"}[reject], func(t *testing.T) {
			server := listenUDP(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			type result struct {
				conn *net.UDPConn
				err  error
			}
			done := make(chan result, 1)
			go func() {
				conn, _, err := connectDarkPlacesBackend(ctx, server.LocalAddr().String())
				done <- result{conn, err}
			}()
			server.SetReadDeadline(time.Now().Add(2 * time.Second))
			buffer := make([]byte, 2048)
			n, peer, err := server.ReadFromUDP(buffer)
			if err != nil || string(buffer[:n]) != "\xff\xff\xff\xffgetchallenge" {
				t.Fatalf("challenge request %q: %v", buffer[:n], err)
			}
			server.WriteToUDP([]byte("\xff\xff\xff\xffchallenge abcd1234\x00crypto-extension"), peer)
			n, secondPeer, err := server.ReadFromUDP(buffer)
			want := "\xff\xff\xff\xffconnect\\protocol\\darkplaces 3\\protocols\\DP7\\challenge\\abcd1234"
			if err != nil || string(buffer[:n]) != want || peer.String() != secondPeer.String() {
				t.Fatalf("connect request %q from %v: %v", buffer[:n], secondPeer, err)
			}
			response := "accept"
			if reject {
				response = "reject Server is full"
			}
			server.WriteToUDP([]byte("\xff\xff\xff\xff"+response), peer)
			got := <-done
			if reject {
				var rejection *backendRejection
				if !errors.As(got.err, &rejection) || rejection.reason != "Server rejected connection: Server is full" {
					t.Fatalf("rejection = %v", got.err)
				}
				return
			}
			if got.err != nil {
				t.Fatal(got.err)
			}
			defer got.conn.Close()
			if got.conn.LocalAddr().(*net.UDPAddr).Port != peer.Port {
				t.Fatal("relay changed the challenge socket")
			}
		})
	}
}

func TestXonoticHoldsNetQuakeClientAndNegotiatesDarkPlacesBackend(t *testing.T) {
	backend := listenUDP(t)
	var starts atomic.Int32
	ready := make(chan struct{})
	_, front, _ := serveGateway(t, "xonotic", func(ctx context.Context, _ string) (string, error) {
		starts.Add(1)
		select {
		case <-ready:
			return backend.LocalAddr().String(), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	client := listenUDP(t)
	sendUDP(t, client, front, []byte("\xff\xff\xff\xffgetchallenge"))
	expectSilence(t, client)
	if starts.Load() != 0 {
		t.Fatal("legacy challenge started a second transport")
	}
	sendUDP(t, client, front, backendConnect)
	expectPacket(t, client, accept(front.Port))
	initial := transportPacket(flagData|flagEOM, 0, 1)
	sendUDP(t, client, front, initial)
	receiveLimbo(t, client)
	close(ready)
	sendUDP(t, client, front, transportPacket(flagAck, 0))
	peer := expectPacket(t, backend, []byte("\xff\xff\xff\xffgetchallenge"))
	sendUDP(t, backend, peer, []byte("\xff\xff\xff\xffchallenge smoke\x00"))
	expectPacket(t, backend, []byte("\xff\xff\xff\xffconnect\\protocol\\darkplaces 3\\protocols\\DP7\\challenge\\smoke"))
	sendUDP(t, backend, peer, []byte("\xff\xff\xff\xffaccept"))
	expectPacket(t, backend, initial)
	sendUDP(t, backend, peer, transportPacket(flagData|flagEOM, 0, 8, 'o', 'k', 0))
	expectPacket(t, client, transportPacket(flagData|flagEOM, 1, 8, 'o', 'k', 0))
	sendUDP(t, client, front, transportPacket(flagAck, 1))
	expectPacket(t, backend, transportPacket(flagAck, 0))
	if starts.Load() != 1 {
		t.Fatal("wake was not shared with the loading connection")
	}
}
