package protocol

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"
)

// QueryValve sends A2S_INFO to a current Steam dedicated server.
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
	if len(packet) < 2 || packet[0] != 'I' {
		return nil, fmt.Errorf("expected A2S_INFO response")
	}
	data := packet[2:] // Response type and protocol version.
	var fields [4]string
	for i := range fields {
		value, rest, ok := bytes.Cut(data, []byte{0})
		if !ok {
			return nil, fmt.Errorf("truncated Valve server information")
		}
		fields[i], data = string(value), rest
	}
	// App ID (2 bytes), players, capacity, bots.
	if len(data) < 5 || data[4] > data[2] {
		return nil, fmt.Errorf("invalid Valve player counts")
	}
	return &ServerInfo{Hostname: fields[0], Map: fields[1], Players: int(data[2] - data[4])}, nil
}
