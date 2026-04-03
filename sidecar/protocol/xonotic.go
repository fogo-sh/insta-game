package protocol

import (
	"fmt"
	"net"
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
	payload := string(buf[17:n])
	parts := strings.Split(payload, "\\")
	kv := make(map[string]string)
	for i := 1; i+1 < len(parts); i += 2 {
		kv[parts[i]] = parts[i+1]
	}

	players := 0
	if v, ok := kv["clients"]; ok {
		fmt.Sscanf(v, "%d", &players)
	}

	return &ServerInfo{Players: players}, nil
}
