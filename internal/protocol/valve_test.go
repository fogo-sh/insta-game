package protocol

import (
	"bytes"
	"net"
	"testing"
)

func valvePacket() []byte {
	return append([]byte{'I', 48}, []byte("Test server\x00crossfire\x00valve\x00Half-Life\x00\x46\x00\x03\x0c\x01")...)
}

func TestValveChallengeAndHumanPlayers(t *testing.T) {
	requests := 0
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		requests++
		request := []byte("\xff\xff\xff\xffTSource Engine Query\x00")
		if requests == 1 {
			if !bytes.Equal(payload, request) {
				t.Errorf("unexpected request: %x", payload)
			}
			conn.WriteToUDP([]byte("\xff\xff\xff\xffA\x01\x02\x03\x04"), addr)
		} else {
			if !bytes.Equal(payload, append(request, 1, 2, 3, 4)) {
				t.Errorf("challenge was not echoed: %x", payload)
			}
			conn.WriteToUDP(append([]byte{255, 255, 255, 255}, valvePacket()...), addr)
		}
	})
	info, err := QueryValve(port)
	if err != nil {
		t.Fatal(err)
	}
	if *info != (ServerInfo{Hostname: "Test server", Map: "crossfire", Players: 2}) {
		t.Fatalf("status: %+v", info)
	}
}

func TestValveMalformedInfo(t *testing.T) {
	good := valvePacket()
	for n := range len(good) {
		if _, err := parseValveInfo(good[:n]); err == nil {
			t.Fatalf("accepted truncation at byte %d", n)
		}
	}
	invalid := append([]byte(nil), good...)
	invalid[len(invalid)-1] = 4
	if _, err := parseValveInfo(invalid); err == nil {
		t.Fatal("accepted more bots than players")
	}
}

func TestValveRepeatedChallengeIsBounded(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, _ []byte) {
		conn.WriteToUDP([]byte("\xff\xff\xff\xffA\x01\x02\x03\x04"), addr)
	})
	if _, err := QueryValve(port); err == nil {
		t.Fatal("accepted repeated challenge")
	}
}
