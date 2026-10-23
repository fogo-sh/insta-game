package wake

import (
	"context"
	"errors"
	"net"
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
