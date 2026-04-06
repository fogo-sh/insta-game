package protocol

import (
	"encoding/binary"
	"net"
	"reflect"
	"testing"
)

func TestQueryXonotic(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		if string(payload) != "\xff\xff\xff\xffgetinfo" {
			t.Fatalf("unexpected query payload: %q", string(payload))
		}

		response := []byte("\xff\xff\xff\xffinfoResponse\n\\clients\\3\\hostname\\Arena")
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	info, err := QueryXonotic(port)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{Players: 3}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestQueryQuake1(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		if string(payload) != "\x80\x00\x00\x0c\x02QUAKE\x00\x03" {
			t.Fatalf("unexpected query payload: %q", string(payload))
		}

		response := append(
			[]byte{0x80, 0x00, 0x00, 0x14, 0x83},
			[]byte("1\x00Test Server\x00dm6\x00\x05\x10\x00")...,
		)
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	info, err := QueryQuake1(port)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{
		Players:  5,
		Hostname: "Test Server",
		Map:      "dm6",
	}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestQueryQuake2(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		if string(payload) != "\xff\xff\xff\xffgetstatus" {
			t.Fatalf("unexpected query payload: %q", string(payload))
		}

		response := []byte(
			"\xff\xff\xff\xffstatusResponse\n\\hostname\\Q2 Server\\mapname\\q2dm1\n0 10 \"alice\"\n0 20 \"bob\"\n",
		)
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	info, err := QueryQuake2(port)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{
		Players:  2,
		Hostname: "Q2 Server",
		Map:      "q2dm1",
	}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestQueryBZFlag(t *testing.T) {
	port := startTCPServer(t, func(conn net.Conn) {
		buf := make([]byte, len("BZFLAG\r\n\r\n"))
		if _, err := conn.Read(buf); err != nil {
			t.Fatalf("read hello: %v", err)
		}
		if string(buf) != "BZFLAG\r\n\r\n" {
			t.Fatalf("unexpected handshake: %q", string(buf))
		}

		if _, err := conn.Write([]byte("BZFS0221\x00")); err != nil {
			t.Fatalf("write hello: %v", err)
		}

		header := make([]byte, 4)
		if _, err := conn.Read(header); err != nil {
			t.Fatalf("read player request: %v", err)
		}

		// Real server sends 8 bytes total: msglen(2) + msgcode(2) + msglen2(2) + players(2)
		response := make([]byte, 8)
		binary.BigEndian.PutUint16(response[0:2], 0x0004)
		binary.BigEndian.PutUint16(response[2:4], 0x7170)
		binary.BigEndian.PutUint16(response[4:6], 0x0008)
		binary.BigEndian.PutUint16(response[6:8], 7)
		if _, err := conn.Write(response); err != nil {
			t.Fatalf("write player response: %v", err)
		}
	})

	info, err := QueryBZFlag(port)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{Players: 7}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestQueryUT99(t *testing.T) {
	queryPort := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		if string(payload) != "\\status\\" {
			t.Fatalf("unexpected query payload: %q", string(payload))
		}

		response := []byte("\\hostname\\Facing Worlds\\mapname\\CTF-Face\\numplayers\\4\\final\\")
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	info, err := QueryUT99(queryPort - 1)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{
		Players:  4,
		Hostname: "Facing Worlds",
		Map:      "CTF-Face",
	}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestCompactLines(t *testing.T) {
	got := compactLines(" one \n\n two \n  \nthree\n")
	want := []string{"one", "two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected lines: got %#v want %#v", got, want)
	}
}

func TestQueryQuake3IgnoresBots(t *testing.T) {
	port := startUDPServer(t, func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte) {
		if string(payload) != "\xff\xff\xff\xffgetstatus\n" {
			t.Fatalf("unexpected query payload: %q", string(payload))
		}

		response := []byte(
			"\xff\xff\xff\xffstatusResponse\n\\sv_hostname\\OpenArena\\mapname\\oa_dm5\n15 48 \"alice\"\n4 0 \"sarge\"\n7 63 \"bob\"\n",
		)
		if _, err := conn.WriteToUDP(response, addr); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})

	info, err := QueryQuake3(port)
	if err != nil {
		t.Fatal(err)
	}

	want := &ServerInfo{
		Players:  2,
		Hostname: "OpenArena",
		Map:      "oa_dm5",
	}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("unexpected info: got %+v want %+v", info, want)
	}
}

func TestCountQuake3PlayersCountsConnectingHumans(t *testing.T) {
	got := countQuake3Players([]string{
		"0 CNCT \"joining\"",
		"0 0 \"sarge\"",
		"12 55 \"alice\"",
	})

	if got != 2 {
		t.Fatalf("unexpected player count: got %d want 2", got)
	}
}

func TestParseUT99StatusCountsPlayersFallback(t *testing.T) {
	values := parseUT99Status("\\servername\\Deck16\\maptitle\\DM-Deck16][\\player_0\\alice\\player_1\\bob\\final\\")

	if values["servername"] != "Deck16" {
		t.Fatalf("unexpected servername: %q", values["servername"])
	}
	if values["maptitle"] != "DM-Deck16][" {
		t.Fatalf("unexpected maptitle: %q", values["maptitle"])
	}

	got := countUT99Players(values)
	if got != 2 {
		t.Fatalf("unexpected player count: got %d want 2", got)
	}
}

func startUDPServer(t *testing.T, handler func(conn *net.UDPConn, addr *net.UDPAddr, payload []byte)) int {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 4096)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		handler(conn, addr, buf[:n])
	}()

	return conn.LocalAddr().(*net.UDPAddr).Port
}

func startTCPServer(t *testing.T, handler func(conn net.Conn)) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()

	return listener.Addr().(*net.TCPAddr).Port
}
