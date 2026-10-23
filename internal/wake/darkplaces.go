package wake

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

// DP7 servers disable the legacy NetQuake connection request. Once accepted,
// their reliable transport uses the same framing as the NetQuake loading screen.
func connectDarkPlacesBackend(ctx context.Context, address string) (*net.UDPConn, netip.AddrPort, error) {
	remote, err := netip.ParseAddrPort(address)
	if err != nil {
		return nil, remote, err
	}
	remote = netip.AddrPortFrom(remote.Addr().Unmap(), remote.Port())
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, remote, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	request := []byte("\xff\xff\xff\xffgetchallenge")
	buffer := make([]byte, 2048)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err = conn.WriteToUDPAddrPort(request, remote); err != nil {
			break
		}
		var n int
		var from netip.AddrPort
		n, from, err = conn.ReadFromUDPAddrPort(buffer)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			break
		}
		from = netip.AddrPortFrom(from.Addr().Unmap(), from.Port())
		if from != remote || n < 4 || !bytes.Equal(buffer[:4], []byte{255, 255, 255, 255}) {
			continue
		}
		// A challenge can have a binary crypto extension after its NUL terminator.
		message, _, _ := strings.Cut(string(buffer[4:n]), "\x00")
		switch {
		case strings.HasPrefix(message, "challenge "):
			challenge := strings.TrimPrefix(message, "challenge ")
			if len(challenge) == 0 || len(challenge) > 49 || strings.ContainsAny(challenge, "\\\r\n") {
				continue
			}
			request = []byte("\xff\xff\xff\xffconnect\\protocol\\darkplaces 3\\protocols\\DP7\\challenge\\" + challenge)
		case message == "accept" && string(request) != "\xff\xff\xff\xffgetchallenge":
			conn.SetReadDeadline(time.Time{})
			return conn, remote, nil
		case strings.HasPrefix(message, "reject "):
			err = &backendRejection{"Server rejected connection: " + strings.TrimPrefix(message, "reject ")}
			conn.Close()
			return nil, remote, err
		}
	}
	conn.Close()
	if ctx.Err() != nil {
		return nil, remote, ctx.Err()
	}
	if err == nil {
		err = fmt.Errorf("DarkPlaces backend handshake timed out")
	}
	return nil, remote, err
}
