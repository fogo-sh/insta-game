package protocol

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// QueryXonotic sends a DarkPlaces UDP getinfo query to 127.0.0.1:port and
// returns server info. Returns nil if the server does not respond.
func QueryXonotic(port int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write([]byte("\xff\xff\xff\xffgetinfo"))
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	// Response: \xff\xff\xff\xffinfoResponse\n\key\val\key\val\...
	payload, ok := strings.CutPrefix(string(buf[:n]), "\xff\xff\xff\xffinfoResponse\n")
	if !ok {
		return nil, fmt.Errorf("unexpected Xonotic info response")
	}
	parts := strings.Split(payload, "\\")
	kv := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		kv[parts[i]] = parts[i+1]
	}

	players, err := strconv.Atoi(kv["clients"])
	if err != nil || players < 0 {
		return nil, fmt.Errorf("invalid Xonotic client count")
	}
	if value, ok := kv["bots"]; ok {
		bots, err := strconv.Atoi(value)
		if err != nil || bots < 0 || bots > players {
			return nil, fmt.Errorf("invalid Xonotic bot count")
		}
		players -= bots
	}

	return &ServerInfo{Players: players, Hostname: kv["hostname"], Map: kv["mapname"]}, nil
}
