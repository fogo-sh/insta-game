package protocol

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// QueryUT99 sends an Unreal Engine 1 GameSpy status query to the local query
// port and returns basic server info. UT99 serves game traffic on gamePort and
// query responses on gamePort+1.
func QueryUT99(gamePort int) (*ServerInfo, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", gamePort+1)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte("\\status\\")); err != nil {
		return nil, err
	}

	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	values := parseUT99Status(string(buf[:n]))
	if len(values) == 0 {
		return nil, fmt.Errorf("empty ut99 status response")
	}

	players, _ := strconv.Atoi(values["numplayers"])
	if players == 0 {
		players = countUT99Players(values)
	}

	hostname := values["hostname"]
	if hostname == "" {
		hostname = values["servername"]
	}

	mapName := values["mapname"]
	if mapName == "" {
		mapName = values["maptitle"]
	}

	return &ServerInfo{
		Players:  players,
		Hostname: hostname,
		Map:      mapName,
	}, nil
}

func parseUT99Status(response string) map[string]string {
	parts := strings.Split(strings.Trim(response, "\\"), "\\")
	values := make(map[string]string, len(parts)/2)
	for i := 0; i+1 < len(parts); i += 2 {
		key := strings.TrimSpace(parts[i])
		if key == "" || key == "final" || key == "queryid" {
			continue
		}
		values[key] = parts[i+1]
	}
	return values
}

func countUT99Players(values map[string]string) int {
	players := 0
	for key, value := range values {
		if strings.HasPrefix(key, "player_") && strings.TrimSpace(value) != "" {
			players++
		}
	}
	return players
}
