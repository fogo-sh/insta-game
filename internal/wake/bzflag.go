package wake

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"strings"
	"time"
)

type bzClient struct {
	virtual  byte
	ip       netip.Addr
	accepted bool     // protected by Gateway.mu
	udpPeer  net.Addr // protected by Gateway.mu
	udpAddr  netip.AddrPort
	input    chan []byte
}

func (g *Gateway) bzflagTCP(ctx context.Context, l *listener, socket net.Listener) error {
	for {
		conn, err := socket.Accept()
		if err != nil {
			return err
		}
		peer, err := netip.ParseAddrPort(conn.RemoteAddr().String())
		if err != nil {
			conn.Close()
			continue
		}
		g.mu.Lock()
		if l.bzClients == nil {
			l.bzClients = make(map[byte]*bzClient)
		}
		var client *bzClient
		if g.clients < maxSessions {
			for slot := byte(0); slot < 200; slot++ {
				if l.bzClients[slot] == nil {
					client = &bzClient{virtual: slot, ip: peer.Addr().Unmap(), input: make(chan []byte, 8)}
					l.bzClients[slot] = client
					g.clients++
					break
				}
			}
		}
		g.mu.Unlock()
		if client == nil {
			conn.Close()
			continue
		}
		g.work.Go(func() {
			defer conn.Close()
			defer func() {
				g.mu.Lock()
				delete(l.bzClients, client.virtual)
				g.clients--
				g.mu.Unlock()
			}()
			if err := g.bzflagSession(ctx, l, conn, client); err != nil && ctx.Err() == nil {
				log.Printf("wake %s BZFlag %s: %v", l.instanceID, peer, err)
			}
		})
	}
}

func (g *Gateway) bzflagUDP(l *listener, address netip.AddrPort, peer net.Addr, packet []byte) {
	if len(packet) < 4 || len(packet) > 1024 || int(binary.BigEndian.Uint16(packet))+4 != len(packet) {
		return
	}
	address = netip.AddrPortFrom(address.Addr().Unmap(), address.Port())
	g.mu.Lock()
	defer g.mu.Unlock()
	var client *bzClient
	if len(packet) == 5 && binary.BigEndian.Uint16(packet[2:]) == bzMsgUDPLinkRequest {
		client = l.bzClients[packet[4]]
		if client == nil || !client.accepted || client.ip != address.Addr() || (client.udpAddr.IsValid() && client.udpAddr != address) {
			return
		}
		for _, other := range l.bzClients {
			if other != client && other.udpAddr == address {
				return
			}
		}
		client.udpAddr, client.udpPeer = address, peer
	} else {
		for _, candidate := range l.bzClients {
			if candidate.udpAddr == address {
				client = candidate
				break
			}
		}
	}
	if client != nil {
		select {
		case client.input <- append([]byte(nil), packet...):
		default:
		}
	}
}

type bzBackend struct {
	tcp net.Conn
	udp *net.UDPConn
	id  byte
}

func (g *Gateway) bzflagSession(parent context.Context, l *listener, conn net.Conn, client *bzClient) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	header := make([]byte, len("BZFLAG\r\n\r\n"))
	if _, err := io.ReadFull(conn, header); err != nil || string(header) != "BZFLAG\r\n\r\n" {
		return fmt.Errorf("invalid BZFlag greeting")
	}
	g.mu.Lock()
	client.accepted = true
	g.mu.Unlock()
	if _, err := conn.Write(append([]byte(bzVersion), client.virtual)); err != nil {
		return err
	}
	conn.SetDeadline(time.Time{})
	bootCtx, bootCancel := context.WithTimeout(ctx, attemptTimeout)
	defer bootCancel()
	type result struct {
		backend *bzBackend
		err     error
	}
	ready := make(chan result)
	g.work.Go(func() {
		address, err := g.resolve(bootCtx, parent, l.instanceID)
		var backend *bzBackend
		if err == nil {
			backend, err = bzConnect(bootCtx, address)
		}
		select {
		case ready <- result{backend, err}:
		case <-ctx.Done():
			if backend != nil {
				backend.tcp.Close()
				backend.udp.Close()
			}
		}
	})
	packets := make(chan backendPacket, 8)
	g.work.Go(func() { bzReadStream(ctx, conn, packets) })
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	began := time.Now()
	var pending [][]byte
	var udpRequest []byte
	var backend *bzBackend
	for backend == nil {
		select {
		case <-ctx.Done():
			return nil
		case result := <-ready:
			if result.err != nil {
				message := "Unable to start server. Please try again shortly."
				var rejected *backendRejection
				if errors.As(result.err, &rejected) {
					message = rejected.reason
				}
				bzReject(conn, message)
				return result.err
			}
			backend = result.backend
		case packet := <-packets:
			if packet.err != nil {
				return packet.err
			}
			code := binary.BigEndian.Uint16(packet.data[2:])
			if code == bzMsgExit {
				return nil
			}
			if len(pending) >= 8 {
				return fmt.Errorf("BZFlag startup packet queue is full")
			}
			if err := bzTranslate(packet.data, client.virtual, client.virtual, false); err != nil {
				return err
			}
			pending = append(pending, packet.data)
		case packet := <-client.input:
			if len(packet) == 5 && binary.BigEndian.Uint16(packet[2:]) == bzMsgUDPLinkRequest {
				udpRequest = packet
			}
		case now := <-ticker.C:
			message := fmt.Sprintf("Starting server... %ds elapsed. Please wait.", int(now.Sub(began).Seconds()))
			payload := append([]byte{253, 254, 0}, append([]byte(message), 0)...)
			if _, err := (writeDeadlineConnection{conn}).Write(bzPacket(bzMsgMessage, payload)); err != nil {
				return err
			}
		}
	}
	defer backend.tcp.Close()
	defer backend.udp.Close()
	stopBackend := context.AfterFunc(ctx, func() { backend.tcp.Close(); backend.udp.Close() })
	defer stopBackend()
	for _, p := range pending {
		if err := bzTranslate(p, client.virtual, backend.id, false); err != nil {
			return err
		}
		if _, err := (writeDeadlineConnection{backend.tcp}).Write(p); err != nil {
			return err
		}
	}
	if udpRequest != nil {
		udpRequest[4] = backend.id
		if _, err := backend.udp.Write(udpRequest); err != nil {
			return err
		}
	}
	serverTCP := make(chan backendPacket, 8)
	serverUDP := make(chan backendPacket, 8)
	g.work.Go(func() { bzReadStream(ctx, backend.tcp, serverTCP) })
	g.work.Go(func() {
		buffer := make([]byte, 1025)
		for {
			n, err := backend.udp.Read(buffer)
			select {
			case serverUDP <- backendPacket{append([]byte(nil), buffer[:n]...), err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	})
	for {
		var packet backendPacket
		var destination io.Writer
		var fromServer bool
		select {
		case <-ctx.Done():
			return nil
		case packet = <-packets:
			destination = writeDeadlineConnection{backend.tcp}
		case packet.data = <-client.input:
			destination = backend.udp
		case packet = <-serverTCP:
			fromServer, destination = true, writeDeadlineConnection{conn}
		case packet = <-serverUDP:
			fromServer = true
		}
		if packet.err != nil {
			return packet.err
		}
		var err error
		if destination == nil {
			err = bzTranslateDatagram(packet.data, client.virtual, backend.id)
		} else {
			err = bzTranslate(packet.data, client.virtual, backend.id, fromServer)
		}
		if err != nil {
			bzReject(conn, "Unsupported server protocol. Please contact the server operator.")
			return err
		}
		if destination != nil {
			if _, err := destination.Write(packet.data); err != nil {
				return err
			}
		} else {
			g.mu.Lock()
			peer := client.udpPeer
			g.mu.Unlock()
			if peer != nil {
				if _, err := l.conn.WriteTo(packet.data, peer); err != nil {
					return err
				}
			}
		}
	}
}

func bzConnect(ctx context.Context, address string) (*bzBackend, error) {
	tcp, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { tcp.Close() })
	defer stop()
	ok := false
	defer func() {
		if !ok {
			tcp.Close()
		}
	}()
	tcp.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(tcp, "BZFLAG\r\n\r\n"); err != nil {
		return nil, err
	}
	var hello [8]byte
	if _, err := io.ReadFull(tcp, hello[:]); err != nil {
		return nil, err
	}
	if string(hello[:]) == "REFUSED:" {
		reason, _ := bufio.NewReader(io.LimitReader(tcp, 127)).ReadString(0)
		reason = strings.TrimSpace(strings.TrimSuffix(reason, "\x00"))
		if reason == "" {
			reason = "connection refused"
		}
		return nil, &backendRejection{"Server rejected connection: " + reason}
	}
	if string(hello[:]) != bzVersion {
		return nil, &backendRejection{"Unsupported BZFlag server protocol; expected " + bzVersion + "."}
	}
	var id [1]byte
	if _, err := io.ReadFull(tcp, id[:]); err != nil {
		return nil, err
	}
	if id[0] == 255 {
		return nil, &backendRejection{"Server is full. Please try again later."}
	}
	if id[0] >= 200 {
		return nil, &backendRejection{"BZFlag replay servers are not supported by the wake gateway."}
	}
	remote, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	local := tcp.LocalAddr().(*net.TCPAddr)
	udp, err := net.DialUDP("udp", &net.UDPAddr{IP: local.IP, Zone: local.Zone}, remote)
	if err != nil {
		return nil, err
	}
	tcp.SetDeadline(time.Time{})
	tcp.(*net.TCPConn).SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: time.Minute, Interval: 20 * time.Second, Count: 3})
	ok = true
	return &bzBackend{tcp, udp, id[0]}, nil
}

func bzReadStream(ctx context.Context, conn net.Conn, packets chan<- backendPacket) {
	for {
		packet, err := bzRead(conn)
		select {
		case packets <- backendPacket{packet, err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func bzReject(conn net.Conn, message string) {
	payload := make([]byte, 130)
	binary.BigEndian.PutUint16(payload, 1)
	copy(payload[2:129], message)
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	conn.Write(bzPacket(bzMsgReject, payload))
}
