package protocol

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"
)

// QueryValve supports modern A2S_INFO and the legacy GoldSrc info response.
func QueryValve(port int) (*ServerInfo, error) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return nil, err
	}
	request := []byte("\xff\xff\xff\xffTSource Engine Query\x00")
	buffer := make([]byte, 65535)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := conn.Write(request); err != nil {
			return nil, err
		}
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}
		packet := buffer[:n]
		if n < 5 || !bytes.Equal(packet[:4], []byte{255, 255, 255, 255}) {
			return nil, fmt.Errorf("invalid Valve info packet")
		}
		if packet[4] == 'A' {
			if n != 9 || attempt != 0 {
				return nil, fmt.Errorf("invalid or repeated Valve challenge")
			}
			request = append(request, packet[5:9]...)
			continue
		}
		return parseValveInfo(packet[4:])
	}
	return nil, fmt.Errorf("Valve info challenge was not completed")
}

func parseValveInfo(packet []byte) (*ServerInfo, error) {
	if len(packet) < 2 {
		return nil, fmt.Errorf("truncated Valve info")
	}
	kind := packet[0]
	data := packet[1:]
	readString := func() (string, bool) {
		end := bytes.IndexByte(data, 0)
		if end < 0 {
			return "", false
		}
		s := string(data[:end])
		data = data[end+1:]
		return s, true
	}
	switch kind {
	case 'I':
		data = data[1:] // Protocol version.
	case 'm':
		if _, ok := readString(); !ok {
			return nil, fmt.Errorf("missing GoldSrc address")
		}
	default:
		return nil, fmt.Errorf("unsupported Valve info response %x", kind)
	}
	hostname, ok := readString()
	if !ok {
		return nil, fmt.Errorf("missing Valve hostname")
	}
	level, ok := readString()
	if !ok {
		return nil, fmt.Errorf("missing Valve map")
	}
	for range 2 {
		if _, ok := readString(); !ok {
			return nil, fmt.Errorf("truncated Valve game information")
		}
	}
	if kind == 'I' {
		if len(data) < 5 {
			return nil, fmt.Errorf("missing Valve player counts")
		}
		// App ID occupies two bytes before players, capacity, and bots.
		players, bots := int(data[2]), int(data[4])
		if bots > players {
			return nil, fmt.Errorf("Valve bot count exceeds player count")
		}
		return &ServerInfo{Hostname: hostname, Map: level, Players: players - bots}, nil
	}
	if len(data) < 7 {
		return nil, fmt.Errorf("missing GoldSrc player counts")
	}
	players, mod := int(data[0]), data[6] != 0
	data = data[7:]
	if mod {
		for range 2 {
			if _, ok := readString(); !ok {
				return nil, fmt.Errorf("truncated GoldSrc mod information")
			}
		}
		if len(data) < 11 {
			return nil, fmt.Errorf("truncated GoldSrc mod properties")
		}
		// Separator, version, download size, and two mod flags.
		data = data[11:]
	}
	if len(data) < 2 {
		return nil, fmt.Errorf("missing GoldSrc bot count")
	}
	bots := int(data[1])
	if bots > players {
		return nil, fmt.Errorf("GoldSrc bot count exceeds player count")
	}
	return &ServerInfo{Hostname: hostname, Map: level, Players: players - bots}, nil
}
