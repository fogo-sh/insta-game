package command

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/fogo-sh/insta-game/internal/control"
)

func discordCommand(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || (args[0] != "commands" && args[0] != "register") {
		return fmt.Errorf("use discord commands or register")
	}
	action := args[0]
	flags := flag.NewFlagSet("discord "+action, flag.ContinueOnError)
	flags.SetOutput(out)
	catalog := flags.String("catalog", "docker-containers", "Game catalog")
	application := flags.String("application", os.Getenv("DISCORD_APP_ID"), "Discord application ID")
	guild := flags.String("guild", os.Getenv("DISCORD_GUILD_ID"), "Allowed guild ID")
	if err := flags.Parse(args[1:]); err != nil {
		return flagError(err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected Discord command arguments")
	}
	games, err := control.LoadCatalog(*catalog)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(control.DiscordCommands(games), "", "  ")
	if err != nil {
		return err
	}
	if action == "commands" {
		_, err := fmt.Fprintln(out, string(body))
		return err
	}
	numeric := regexp.MustCompile(`^[0-9]+$`)
	if !numeric.MatchString(*application) || !numeric.MatchString(*guild) || os.Getenv("DISCORD_BOT_TOKEN") == "" {
		return fmt.Errorf("registration requires numeric application/guild IDs and DISCORD_BOT_TOKEN")
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", "https://discord.com/api/v10/applications/"+*application+"/guilds/"+*guild+"/commands", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+os.Getenv("DISCORD_BOT_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("Discord HTTP %d: %s", res.StatusCode, data)
	}
	_, err = fmt.Fprintln(out, "Registered create, start, stop and status commands for guild "+*guild)
	return err
}
