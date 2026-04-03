package protocol

import (
	"fmt"
	"net"
	"time"
)

// QueryQuake1 sends a NetQuake CCREQ_SERVER_INFO control packet to
// 127.0.0.1:port and returns server info. Returns nil if the server does not
// respond or the response is malformed.
func QueryQuake1(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte("\x80\x00\x00\x0c\x02QUAKE\x00\x03"))
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	if n < 7 || buf[0] != 0x80 || buf[1] != 0x00 || buf[4] != 0x83 {
		return nil, fmt.Errorf("unexpected response header")
	}

	payload := buf[5:n]
	parts := splitNull(payload)
	if len(parts) < 3 {
		return nil, fmt.Errorf("response too short")
	}

	hostname := string(parts[1])
	mapName := string(parts[2])
	players := 0
	if len(payload) >= 3 {
		players = int(payload[len(payload)-3])
	}

	return &ServerInfo{
		Players:  players,
		Hostname: hostname,
		Map:      mapName,
	}, nil
}

func splitNull(b []byte) [][]byte {
	var parts [][]byte
	start := 0
	for i, c := range b {
		if c == 0x00 {
			parts = append(parts, b[start:i])
			start = i + 1
		}
	}
	parts = append(parts, b[start:])
	return parts
}
