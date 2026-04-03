package protocol

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// QueryQuake2 sends a Quake II getstatus query to 127.0.0.1:port and returns
// basic server info. It falls back to the legacy status query if needed.
func QueryQuake2(port int) (*ServerInfo, error) {
	for _, query := range []string{"\xff\xff\xff\xffgetstatus", "\xff\xff\xff\xffstatus"} {
		info, err := queryQuake2(port, query)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("no quake2 status response")
}

func queryQuake2(port int, query string) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(query)); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < 5 || string(buf[:4]) != "\xff\xff\xff\xff" {
		return nil, fmt.Errorf("unexpected response header")
	}

	lines := compactLines(string(buf[4:n]))
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty status response")
	}
	if lines[0] == "print" || lines[0] == "statusResponse" {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("missing server info")
	}

	parts := strings.Split(lines[0], "\\")
	values := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		values[parts[i]] = parts[i+1]
	}

	return &ServerInfo{
		Players:  len(lines) - 1,
		Hostname: values["hostname"],
		Map:      values["mapname"],
	}, nil
}

func compactLines(value string) []string {
	rawLines := strings.Split(value, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
