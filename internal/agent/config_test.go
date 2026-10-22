package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartupDistinguishesEmptyConfigFromAbsentConfig(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "absent"
		if present {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("CONFIG_TEXT", "")
			if !present {
				os.Unsetenv("CONFIG_TEXT")
			}
			directory := t.TempDir()
			defaults := filepath.Join(directory, "default.cfg")
			path := filepath.Join(directory, "server.cfg")
			if err := os.WriteFile(defaults, []byte("bundled defaults"), 0600); err != nil {
				t.Fatal(err)
			}
			c := FromEnv()
			c.GameCmd, c.GameArgs, c.Token, c.Protocol = "/bin/true", nil, "test-token", "quake1"
			c.DefaultConfig, c.ConfigPath, c.DataRoot = defaults, path, directory
			c.DataURL, c.SeedPath, c.DataOwner, c.RconPassword = "", "", "", ""
			c.Host, c.Port, c.TLS = "127.0.0.1", 0, false
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if err := Run(ctx, c); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "bundled defaults"
			if present {
				want = ""
			}
			if string(got) != want {
				t.Fatalf("config = %q, want %q", got, want)
			}
		})
	}
}
