package wake

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"time"
)

func connectionRequest(protocol string, packet []byte) bool {
	if len(packet) == 0 || len(packet) > 4096 {
		return false
	}
	switch protocol {
	case "quake2", "quake3":
		if !bytes.HasPrefix(packet, []byte{255, 255, 255, 255}) {
			return false
		}
		command := bytes.TrimSpace(bytes.TrimRight(packet[4:], "\x00"))
		return bytes.Equal(command, []byte("getchallenge")) || bytes.HasPrefix(command, []byte("getchallenge "))
	case "ut99":
		// GameSpy status requests are separate from the binary game handshake.
		return len(packet) >= 3 && packet[0] != '\\'

	}
	return false
}

func (s *session) relay(ctx context.Context, first []byte) error {
	parent := ctx
	ctx, end := context.WithCancel(ctx)
	defer end()
	bootCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	address, err := s.waitForGame(bootCtx, parent)
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

// Hold clients before the backend handshake without consuming game sequences.
func (s *session) waitForGame(ctx, parent context.Context) (string, error) {
	interval := 5 * time.Second
	var progress []byte
	switch s.listener.protocol {
	case "quake2", "quake3":
		progress = []byte("\xff\xff\xff\xffprint\nStarting server...\n\x00")
	case "ut99":
		// Packet -1 refreshes receive time without advancing the initial sequence.
		progress, interval = []byte{0xff, 0x7f}, time.Second
	default:
		return s.gateway.resolve(ctx, parent, s.listener.instanceID)
	}
	ready := make(chan destination, 1)
	s.gateway.work.Go(func() {
		address, err := s.gateway.resolve(ctx, parent, s.listener.instanceID)
		ready <- destination{address, err}
	})
	notice := func(message string) error {
		return s.send(append([]byte{255, 255, 255, 255}, []byte("print\n"+message+"\n\x00")...))
	}
	fail := func(message string) {
		if s.listener.protocol == "ut99" {
			for range 3 {
				s.send(ut99Rejection())
			}
		} else {
			notice(message)
		}
	}
	if s.listener.protocol != "ut99" {
		if err := s.send(progress); err != nil {
			return "", err
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	idle := time.NewTimer(clientTimeout)
	defer idle.Stop()
	for {
		select {
		case <-idle.C:
			return "", errors.New("client inactive while waiting for server")
		case <-ctx.Done():
			if parent.Err() == nil {
				fail("Server did not start in time. Please try again.")
			}
			return "", ctx.Err()
		case result := <-ready:
			if result.err != nil {
				fail("Unable to start server. Please try again.")
			}
			return result.address, result.err
		case <-ticker.C:
			if err := s.send(progress); err != nil {
				return "", err
			}
		case <-s.input:
			// The client resends its challenge request until the backend answers.
			idle.Reset(clientTimeout)
		}
	}
}

type writeDeadlineConnection struct{ net.Conn }

func (c writeDeadlineConnection) Write(p []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(clientTimeout))
	return c.Conn.Write(p)
}
