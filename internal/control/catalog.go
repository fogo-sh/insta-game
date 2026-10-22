package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func LoadCatalog(root string) ([]Game, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*", "game.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no game.json files in %s", root)
	}
	games := make([]Game, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var game Game
		if err := json.Unmarshal(data, &game); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if !validID.MatchString(game.ID) || seen[game.ID] || game.GameCmd == "" || game.GamePort < 1 || game.AgentPort < 1 {
			return nil, fmt.Errorf("%s: invalid or duplicate game definition", path)
		}
		if game.GamePort > 65535 || game.AgentPort > 65535 || game.GamePort == game.AgentPort {
			return nil, fmt.Errorf("%s: invalid game/agent port", path)
		}
		gamePortFound := false
		for key := range game.Ports {
			parts := strings.Split(key, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("%s: invalid port %q", path, key)
			}
			number, err := strconv.Atoi(parts[0])
			if err != nil || number < 1 || number > 65535 || (parts[1] != "tcp" && parts[1] != "udp") {
				return nil, fmt.Errorf("%s: invalid port %q", path, key)
			}
			if number == game.GamePort {
				gamePortFound = true
			}
		}
		if !gamePortFound {
			return nil, fmt.Errorf("%s: gamePort is missing from ports", path)
		}

		seen[game.ID] = true
		if game.Image == "" {
			game.Image = "ghcr.io/fogo-sh/insta-game:" + game.ID
		}
		if game.DataRoot == "" {
			game.DataRoot = "/opt"
		}
		if game.ConfigPath == "" {
			game.ConfigPath = "/opt/data/server.cfg"
		}
		candidates := []string{"server.cfg", "UnrealTournament.ini"}
		for _, name := range candidates {
			b, err := os.ReadFile(filepath.Join(filepath.Dir(path), name))
			if err == nil {
				game.DefaultConfig = string(b)
				break
			}
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		games = append(games, game)
	}
	sort.Slice(games, func(i, j int) bool { return games[i].DisplayName < games[j].DisplayName })
	return games, nil
}
