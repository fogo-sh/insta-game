package protocol

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// QueryQuake3 sends a Quake III Arena getstatus query to 127.0.0.1:port and
// returns basic server info.
func QueryQuake3(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("\xff\xff\xff\xffgetstatus\n")); err != nil {
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
	if len(lines) < 2 || lines[0] != "statusResponse" {
		return nil, fmt.Errorf("unexpected status response")
	}

	parts := strings.Split(lines[1], "\\")
	values := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		values[parts[i]] = parts[i+1]
	}

	return &ServerInfo{
		Players:  countQuake3Players(lines[2:]),
		Hostname: values["sv_hostname"],
		Map:      values["mapname"],
	}, nil
}

func countQuake3Players(lines []string) int {
	players := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		ping, err := strconv.Atoi(fields[1])
		switch {
		case err == nil && ping > 0:
			players++
		case err != nil:
			// Non-numeric pings like CNCT/ZMBI indicate a real client state.
			players++
		}
	}
	return players
}
