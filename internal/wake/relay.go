package wake

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/netip"
	"time"
)

// Other protocols keep their own handshake and packet numbers end to end.
// Clients whose connection timeout is shorter than startup must retry the same address.
func connectionRequest(protocol string, packet []byte) bool {
	if len(packet) == 0 || len(packet) > 4096 {
		return false
	}
	switch protocol {
	case "quake2", "quake3", "xonotic":
		if !bytes.HasPrefix(packet, []byte{255, 255, 255, 255}) {
			return false
		}
		command := bytes.TrimSpace(bytes.TrimRight(packet[4:], "\x00"))
		return bytes.Equal(command, []byte("getchallenge")) || bytes.HasPrefix(command, []byte("getchallenge "))
	case "ut99":
		// GameSpy status requests are separate from the binary game handshake.
		return len(packet) >= 3 && packet[0] != '\\'
	case "bzflag":
		// UDP link requests carry the slot assigned over TCP.
		return len(packet) == 5 && binary.BigEndian.Uint16(packet[:2]) == 1 && binary.BigEndian.Uint16(packet[2:4]) == 0x6f66
	}
	return false
}

func (s *session) relay(ctx context.Context, first []byte) error {
	ctx, end := context.WithCancel(ctx)
	defer end()
	bootCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	address, err := s.gateway.resolve(bootCtx, ctx, s.listener.instanceID)
	if err != nil {
		return err
	}
	remote, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	backend, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		return err
	}
	defer backend.Close()
	stopClose := context.AfterFunc(ctx, func() { backend.Close() })
	defer stopClose()
	// Retain only the first handshake while booting. Later retransmits are redundant.
	for range cap(s.input) {
		select {
		case <-s.input:
		default:
			goto drained
		}
	}
drained:
	log.Printf("wake %s %s: relaying to %s", s.listener.instanceID, s.address, address)
	if _, err = backend.Write(first); err != nil {
		return err
	}
	packets := make(chan backendPacket, 8)
	s.gateway.work.Go(func() {
		buffer := make([]byte, 65535)
		for {
			n, err := backend.Read(buffer)
			select {
			case packets <- backendPacket{append([]byte(nil), buffer[:n]...), err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	})
	timer := time.NewTimer(clientTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return nil
		case packet := <-s.input:
			timer.Reset(clientTimeout)
			if _, err := backend.Write(packet); err != nil {
				return err
			}
		case packet := <-packets:
			if packet.err != nil {
				return packet.err
			}
			if err := s.send(packet.data); err != nil {
				return err
			}
		}
	}
}

func (g *Gateway) tcp(ctx context.Context, l *listener, socket net.Listener) error {
	for {
		client, err := socket.Accept()
		if err != nil {
			return err
		}
		g.mu.Lock()
		admitted := g.clients < maxSessions
		if admitted {
			g.clients++
		}
		g.mu.Unlock()
		if !admitted {
			client.Close()
			continue
		}
		g.work.Go(func() {
			defer func() { g.mu.Lock(); g.clients--; g.mu.Unlock() }()
			defer client.Close()
			stopClose := context.AfterFunc(ctx, func() { client.Close() })
			defer stopClose()
			client.SetDeadline(time.Now().Add(5 * time.Second))
			header := make([]byte, len("BZFLAG\r\n\r\n"))
			if _, err := io.ReadFull(client, header); err != nil || string(header) != "BZFLAG\r\n\r\n" {
				return
			}
			client.SetDeadline(time.Now().Add(attemptTimeout))
			peer, err := netip.ParseAddrPort(client.RemoteAddr().String())
			if err != nil {
				return
			}
			g.mu.Lock()
			l.tcpPeers[peer.Addr()]++
			g.mu.Unlock()
			defer func() {
				g.mu.Lock()
				l.tcpPeers[peer.Addr()]--
				if l.tcpPeers[peer.Addr()] == 0 {
					delete(l.tcpPeers, peer.Addr())
				}
				g.mu.Unlock()
			}()
			bootCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
			defer cancel()
			address, err := g.resolve(bootCtx, ctx, l.instanceID)
			if err != nil {
				log.Printf("wake %s: %v", l.instanceID, err)
				return
			}
			backend, err := (&net.Dialer{}).DialContext(bootCtx, "tcp", address)
			if err != nil {
				log.Printf("wake %s: %v", l.instanceID, err)
				return
			}
			defer backend.Close()
			if _, err := backend.Write(header); err != nil {
				return
			}
			client.SetDeadline(time.Time{})
			// Gameplay can stay on UDP while its TCP control connection is quiet.
			client.(*net.TCPConn).SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: time.Minute, Interval: 20 * time.Second, Count: 3})
			done := make(chan struct{})
			go func() {
				io.Copy(writeDeadlineConnection{backend}, writeDeadlineConnection{client})
				backend.Close()
				close(done)
			}()
			io.Copy(writeDeadlineConnection{client}, writeDeadlineConnection{backend})
			client.Close()
			<-done
		})
	}
}

type writeDeadlineConnection struct{ net.Conn }

func (c writeDeadlineConnection) Write(p []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(clientTimeout))
	return c.Conn.Write(p)
}
