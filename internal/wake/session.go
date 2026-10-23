package wake

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"time"
)

const clientTimeout = time.Minute

type session struct {
	gateway  *Gateway
	listener *listener
	peer     net.Addr
	address  netip.AddrPort
	input    chan []byte
}

type destination struct {
	address string
	err     error
}
type backendRejection struct{ reason string }

func (e *backendRejection) Error() string { return e.reason }

type backendPacket struct {
	data []byte
	err  error
}

func (s *session) send(packet []byte) error {
	_, err := s.listener.conn.WriteTo(packet, s.peer)
	return err
}

func (s *session) run(ctx context.Context) error {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	bootCtx, bootCancel := context.WithTimeout(ctx, attemptTimeout)
	defer bootCancel()
	ready := make(chan destination, 1)
	resolve := func() {
		s.gateway.work.Go(func() {
			address, err := s.gateway.resolve(bootCtx, parent, s.listener.instanceID)
			ready <- destination{address, err}
		})
	}
	resolve()
	log.Printf("wake %s %s: connected; waiting for game", s.listener.instanceID, s.address)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastSeen := time.Now()
	nextMessage := time.Now()
	nextNotice := time.Now()
	began := time.Now()
	var prefix uint32
	var pending, initial []byte
	var sentAt time.Time
	var target string
	var backend *net.UDPConn
	var remote netip.AddrPort
	packets := make(chan backendPacket, 8)
	defer func() {
		if backend != nil {
			backend.Close()
		}
	}()
	for {
		if backend == nil && target != "" && pending == nil {
			connect := connectBackend
			if s.listener.protocol == "xonotic" {
				connect = connectDarkPlacesBackend
			}
			conn, peer, err := connect(bootCtx, target)
			if err != nil {
				var rejected *backendRejection
				if errors.As(err, &rejected) {
					s.fail(rejected.Error())
					return err
				}
				if bootCtx.Err() != nil {
					s.fail("Server did not start in time. Please try again.")
					return err
				}
				log.Printf("wake %s: backend handshake: %v", s.listener.instanceID, err)
				target = ""
				resolve()
			} else {
				backend, remote = conn, peer
				stopClose := context.AfterFunc(ctx, func() { conn.Close() })
				defer stopClose()
				s.gateway.work.Go(func() { readBackend(ctx, conn, peer, packets) })
				if initial != nil {
					backend.WriteToUDPAddrPort(initial, remote)
				}
				log.Printf("wake %s %s: relaying to %s", s.listener.instanceID, s.address, remote)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case result := <-ready:
			if result.err != nil {
				s.fail("Unable to start server. Please try again shortly.")
				return result.err
			}
			target = result.address
		case packet := <-s.input:
			flags, sequence, ok := frame(packet)
			if !ok {
				continue
			}
			lastSeen = time.Now()
			disconnect := flags&(flagData|flagUnreliable) != 0 && len(packet) == 9 && packet[8] == 2
			if flags&flagData != 0 && sequence == 0 && initial == nil {
				initial = append([]byte(nil), packet...)
			}
			if backend == nil {
				if flags&flagAck != 0 && pending != nil && sequence == prefix {
					prefix++
					pending = nil
				}
				if disconnect {
					return nil
				}
				continue
			}
			if flags&(flagAck|flagNak) != 0 {
				if sequence < prefix {
					continue
				}
				binary.BigEndian.PutUint32(packet[4:], sequence-prefix)
			}
			if _, err := backend.WriteToUDPAddrPort(packet, remote); err != nil {
				return err
			}
			if disconnect {
				return nil
			}
		case packet := <-packets:
			if packet.err != nil {
				return packet.err
			}
			flags, sequence, ok := frame(packet.data)
			if !ok {
				continue
			}
			// Extended game protocols acknowledge unreliable packet IDs in their payloads.
			// Only the reliable channel receives the acknowledged loading-message prefix.
			if flags&flagData != 0 {
				binary.BigEndian.PutUint32(packet.data[4:], sequence+prefix)
			}
			if err := s.send(packet.data); err != nil {
				return err
			}
		case now := <-ticker.C:
			if now.Sub(lastSeen) > clientTimeout {
				return nil
			}
			if backend != nil {
				continue
			}
			if bootCtx.Err() != nil {
				s.fail("Server did not start in time. Please try again.")
				return bootCtx.Err()
			}
			if pending != nil {
				if now.Sub(sentAt) >= time.Second {
					if err := s.send(pending); err != nil {
						return err
					}
					sentAt = now
				}
			} else if target == "" && !now.Before(nextMessage) {
				payload := []byte{1}
				if !now.Before(nextNotice) {
					payload = printMessage(fmt.Sprintf("Starting server... %ds elapsed. Please wait.\n", int(now.Sub(began).Seconds())))
					nextNotice = now.Add(5 * time.Second)
				}
				pending = dataPacket(prefix, payload)
				if err := s.send(pending); err != nil {
					return err
				}
				sentAt = now
				nextMessage = now.Add(2 * time.Second)
			}
		}
	}
}

func (s *session) fail(message string) {
	// No game is attached after failure, so an unreliable disconnect cannot affect frame ACKs.
	packet := dataPacket(0, append(printMessage(message+"\n"), 2))
	binary.BigEndian.PutUint32(packet, flagUnreliable|uint32(len(packet)))
	for range 3 {
		s.send(packet)
	}
}

func connectBackend(ctx context.Context, address string) (*net.UDPConn, netip.AddrPort, error) {
	remote, err := netip.ParseAddrPort(address)
	if err != nil {
		return nil, remote, err
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, remote, err
	}
	stopClose := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopClose()
	buffer := make([]byte, 1024)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err = conn.WriteToUDPAddrPort(backendConnect, remote); err != nil {
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
		if from != remote || n < 5 || binary.BigEndian.Uint32(buffer) != flagControl|uint32(n) {
			continue
		}
		if buffer[4] == 0x82 {
			reason, _, _ := strings.Cut(string(buffer[5:n]), "\x00")
			err = &backendRejection{"Server rejected connection: " + reason}
			break
		}
		if buffer[4] != 0x81 || n < 9 {
			continue
		}
		if n != 9 {
			err = &backendRejection{"Backend requires extended transport; configure FTE sv_listen_nq 1."}
			break
		}
		port := uint16(binary.LittleEndian.Uint32(buffer[5:9]))
		if port != 0 {
			remote = netip.AddrPortFrom(remote.Addr(), port)
		}
		conn.SetReadDeadline(time.Time{})
		return conn, remote, nil
	}
	conn.Close()
	if ctx.Err() != nil {
		return nil, remote, ctx.Err()
	}
	if err == nil {
		err = fmt.Errorf("backend handshake timed out")
	}
	return nil, remote, err
}

func readBackend(ctx context.Context, conn *net.UDPConn, remote netip.AddrPort, out chan<- backendPacket) {
	buffer := make([]byte, 65535)
	for {
		n, from, err := conn.ReadFromUDPAddrPort(buffer)
		if err != nil {
			select {
			case out <- backendPacket{err: err}:
			case <-ctx.Done():
			}
			return
		}
		from = netip.AddrPortFrom(from.Addr().Unmap(), from.Port())
		if from != remote {
			continue
		}
		packet := backendPacket{data: append([]byte(nil), buffer[:n]...)}
		select {
		case out <- packet:
		case <-ctx.Done():
			return
		}
	}
}
