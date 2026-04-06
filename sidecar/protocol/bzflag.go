package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// QueryBZFlag connects to a local bzfs server, performs the same handshake as
// bzfquery.pl, and returns the number of active players.
func QueryBZFlag(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return nil, err
	}

	if _, err := io.WriteString(conn, "BZFLAG\r\n\r\n"); err != nil {
		return nil, err
	}

	hello := make([]byte, 9)
	if _, err := io.ReadFull(conn, hello); err != nil {
		return nil, err
	}
	if string(hello[:4]) != "BZFS" {
		return nil, fmt.Errorf("not a bzflag server")
	}
	if string(hello[4:8]) != "0221" {
		return nil, fmt.Errorf("unsupported bzflag protocol %q", string(hello[4:8]))
	}
	if hello[8] == 255 {
		return nil, fmt.Errorf("bzflag server rejected connection")
	}

	if err := binary.Write(conn, binary.BigEndian, [2]uint16{0, 0x7170}); err != nil {
		return nil, err
	}

	// Response is 8 bytes: msglen(2) + msgcode(2) + msglen2(2) + players(2)
	reply := make([]byte, 8)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return nil, err
	}
	if binary.BigEndian.Uint16(reply[2:4]) != 0x7170 {
		return nil, fmt.Errorf("unexpected bzflag player reply: %x", reply)
	}

	return &ServerInfo{
		Players: int(binary.BigEndian.Uint16(reply[6:8])),
	}, nil
}
