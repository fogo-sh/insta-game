package control

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type discordInteraction struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Guild string `json:"guild_id"`
	Data  struct {
		Name    string `json:"name"`
		Options []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"options"`
	} `json:"data"`
}

func (c *Controller) discord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid interaction body"})
		return
	}
	timestamp := r.Header.Get("X-Signature-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	signature, decodeErr := hex.DecodeString(r.Header.Get("X-Signature-Ed25519"))
	age := time.Since(time.Unix(seconds, 0))
	if err != nil || decodeErr != nil || age > 5*time.Minute || age < -5*time.Minute || !ed25519.Verify(c.DiscordKey, append([]byte(timestamp), body...), signature) {
		writeJSON(w, 401, map[string]string{"error": "invalid interaction signature"})
		return
	}
	var interaction discordInteraction
	if err := json.Unmarshal(body, &interaction); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid interaction"})
		return
	}
	if interaction.Type == 1 {
		writeJSON(w, 200, map[string]int{"type": 1})
		return
	}
	if interaction.Guild != c.DiscordGuild {
		writeJSON(w, 403, map[string]string{"error": "guild is not allowed"})
		return
	}
	if interaction.Type != 2 {
		writeJSON(w, 400, map[string]string{"error": "unsupported interaction"})
		return
	}
	options := map[string]string{}
	for _, option := range interaction.Data.Options {
		options[option.Name] = option.Value
	}
	content, err := c.discordCommand(interaction.ID, interaction.Data.Name, options)
	if err != nil {
		content = err.Error()
	}
	if len(content) > 1800 {
		content = strings.ToValidUTF8(content[:1800], "") + "…"
	}
	writeJSON(w, 200, map[string]any{"type": 4, "data": map[string]any{"content": content, "allowed_mentions": map[string]any{"parse": []string{}}}})
}

func (c *Controller) discordCommand(id, name string, options map[string]string) (string, error) {
	switch name {
	case "create":
		if id == "" {
			return "", fmt.Errorf("missing interaction ID")
		}
		instance, err := c.Create(CreateRequest{ID: "discord-" + id, Game: options["game"], Name: options["name"], Provider: options["provider"], IdleMinutes: 10, MaxMinutes: 120})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Created `%s` (%s). Use /status to check readiness.", instance.ID, instance.Phase), nil
	case "start", "stop":
		desired := map[string]string{"start": "running", "stop": "stopped"}[name]
		if err := c.SetDesired(options["server"], desired); err != nil {
			return "", err
		}
		return fmt.Sprintf("`%s`: requested %s.", options["server"], desired), nil
	case "status":
		if options["server"] != "" {
			instance, err := c.Store.Get(options["server"])
			if err != nil {
				return "", err
			}
			return discordStatus(instance), nil
		}
		instances, err := c.Store.List()
		if err != nil {
			return "", err
		}
		lines := []string{}
		for _, instance := range instances {
			if instance.Phase == "deleted" {
				continue
			}
			lines = append(lines, discordStatus(instance))
			if len(lines) == 20 {
				lines = append(lines, "See the web UI or CLI for the full list.")
				break
			}
		}
		if len(lines) == 0 {
			return "No servers.", nil
		}
		return strings.Join(lines, "\n"), nil
	default:
		return "", fmt.Errorf("unknown command %q", name)
	}
}

func discordStatus(i Instance) string {
	content := fmt.Sprintf("`%s`: %s", i.ID, i.Phase)
	if address := i.Connect(); address != "" {
		content += " — `" + address + "`"
	}
	if i.Players != nil {
		content += fmt.Sprintf(" — %d players", *i.Players)
	}
	return content
}

func DiscordCommands(games []Game) []any {
	option := func(name, description string, required bool) map[string]any {
		return map[string]any{"name": name, "description": description, "type": 3, "required": required}
	}
	game := option("game", "Game to launch", true)
	choices := []any{}
	for _, g := range games {
		choices = append(choices, map[string]string{"name": g.DisplayName, "value": g.ID})
	}
	if len(choices) <= 25 {
		game["choices"] = choices
	}
	return []any{
		map[string]any{"name": "create", "description": "Create an independent game server", "options": []any{game, option("name", "Server name", true), option("provider", "Execution provider (controller default if omitted)", false)}},
		map[string]any{"name": "start", "description": "Start a stopped server", "options": []any{option("server", "Server instance ID", true)}},
		map[string]any{"name": "stop", "description": "Stop a server and retain its data", "options": []any{option("server", "Server instance ID", true)}},
		map[string]any{"name": "status", "description": "Show server status and addresses", "options": []any{option("server", "Server instance ID (omit to list)", false)}},
	}
}
