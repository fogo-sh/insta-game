package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func QueryQuake1(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("\x80\x00\x00\x0c\x02QUAKE\x00\x03")); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < 5 || binary.BigEndian.Uint32(buf[:4]) != 0x80000000|uint32(n) || buf[4] != 0x83 {
		return nil, fmt.Errorf("unexpected Quake server info header")
	}
	payload := buf[5:n]
	var fields [3][]byte
	for i := range fields {
		var found bool
		fields[i], payload, found = bytes.Cut(payload, []byte{0})
		if !found {
			return nil, fmt.Errorf("incomplete Quake server info")
		}
	}
	if len(payload) != 3 {
		return nil, fmt.Errorf("invalid Quake server counts")
	}
	return &ServerInfo{Players: int(payload[0]), Hostname: string(fields[1]), Map: string(fields[2])}, nil
}
