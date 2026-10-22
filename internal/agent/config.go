package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TLS             bool
	Protocol        string
	MetadataPath    string
	ProtocolFile    string
	GameDir         string
	GameCmd         string
	GameArgs        []string
	GameQuitCmd     string
	GameQuitTimeout time.Duration
	GamePort        int
	DataURL         string
	DefaultConfig   string
	ConfigPath      string
	RconPassword    string
	UserAgent       string
	Token           string
	LogToken        string
	ConfigText      string
	Host            string
	Port            int
}

func FromEnv() Config {
	gameArgs := []string{}
	if v := envOr("GAME_ARGS", ""); v != "" {
		gameArgs = strings.Fields(v)
	}
	quitTimeout, _ := strconv.Atoi(envOr("GAME_QUIT_TIMEOUT", "15"))
	gamePort, _ := strconv.Atoi(envOr("GAME_PORT", "26000"))
	port, _ := strconv.Atoi(envOr("PORT", "5001"))

	return Config{
		TLS:             envOr("AGENT_TLS", "false") == "true",
		MetadataPath:    envOr("GAME_METADATA_PATH", "/opt/game.json"),
		ProtocolFile:    envOr("PROTOCOL_FILE", "/opt/protocol.txt"),
		GameCmd:         envOr("GAME_CMD", ""),
		GameArgs:        gameArgs,
		GameQuitCmd:     envOr("GAME_QUIT_CMD", "quit"),
		GameQuitTimeout: time.Duration(quitTimeout) * time.Second,
		GamePort:        gamePort,
		DataURL:         envOr("DATA_URL", ""),
		DefaultConfig:   envOr("DEFAULT_CONFIG_PATH", "/opt/default-server.cfg"),
		ConfigPath:      envOr("CONFIG_PATH", "/opt/data/server.cfg"),
		RconPassword:    envOr("RCON_PASSWORD", ""),
		UserAgent:       envOr("DOWNLOAD_USER_AGENT", ""),
		Token:           envOr("TOKEN", ""),
		LogToken:        envOr("LOG_TOKEN", ""),
		ConfigText:      envOr("CONFIG_TEXT", ""),
		Host:            envOr("HOST", "0.0.0.0"),
		Port:            port,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func resolveProtocol(c Config) string {
	if protocol := strings.TrimSpace(os.Getenv("PROTOCOL")); protocol != "" {
		return protocol
	}

	if protocol, err := readMetadataProtocol(c.MetadataPath); err == nil {
		return protocol
	}

	if protocol, err := readProtocolFile(c.ProtocolFile); err == nil {
		return protocol
	}

	return inferProtocolFromConfig(c)
}

func readProtocolFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	protocol := strings.TrimSpace(string(content))
	if protocol == "" {
		return "", fmt.Errorf("protocol file %q is empty", path)
	}
	return protocol, nil
}

func readMetadataProtocol(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var metadata struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(content, &metadata); err != nil {
		return "", err
	}
	if strings.TrimSpace(metadata.Protocol) == "" {
		return "", fmt.Errorf("metadata file %q does not define a protocol", path)
	}
	return strings.TrimSpace(metadata.Protocol), nil
}

func inferProtocolFromConfig(c Config) string {
	switch strings.ToLower(filepath.Base(c.GameCmd)) {
	case "bzfs":
		return "bzflag"
	case "fteqw.sv":
		return "quake1"
	case "q2proded", "q2reproded", "q2pro":
		return "quake2"
	case "ioq3ded":
		return "quake3"
	case "start-ut99.sh", "ucc-bin-arm64":
		return "ut99"
	case "xonotic-linux-arm64-dedicated":
		return "xonotic"
	}

	switch strings.ToLower(filepath.Base(c.ConfigPath)) {
	case "unrealtournament.ini":
		return "ut99"
	}

	return "xonotic"
}
