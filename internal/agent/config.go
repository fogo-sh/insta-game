package agent

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	SeedPath        string
	DataRoot        string
	DataOwner       string
	TLS             bool
	Protocol        string
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
	ConfigText      *string
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

	var configText *string
	if text, present := os.LookupEnv("CONFIG_TEXT"); present {
		configText = &text
	}

	return Config{
		Protocol:        envOr("PROTOCOL", ""),
		SeedPath:        envOr("DATA_SEED", ""),
		DataRoot:        envOr("DATA_ROOT", "/opt"),
		DataOwner:       envOr("INSTANCE_OWNER", ""),
		TLS:             envOr("AGENT_TLS", "false") == "true",
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
		ConfigText:      configText,
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
