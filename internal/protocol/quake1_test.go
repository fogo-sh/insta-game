package protocol

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestQueryQuake1RejectsIncompleteCounts(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
		packet := append([]byte{0, 0, 0, 0, 0x83}, []byte("local\x00Arena\x00dm6\x00")...)
		binary.BigEndian.PutUint32(packet[:4], 0x80000000|uint32(len(packet)))
		conn.WriteToUDP(packet, addr)
	})
	if _, err := QueryQuake1(port); err == nil {
		t.Fatal("missing counts must not be reported as an empty server")
	}
}
