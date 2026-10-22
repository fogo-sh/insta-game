package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureRconQuakeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.cfg")
	err := os.WriteFile(path, []byte("hostname test\nrcon_password \"old\"\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = configureRcon("quake1", path, "new-secret")
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "rcon_password \"new-secret\"") {
		t.Fatalf("expected updated rcon password, got:\n%s", got)
	}
	if strings.Contains(got, "old") {
		t.Fatalf("expected old password to be removed, got:\n%s", got)
	}
}

func TestConfigureRconQuake3Config(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.cfg")
	err := os.WriteFile(path, []byte("set sv_hostname \"test\"\nseta rconPassword \"old\"\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = configureRcon("quake3", path, "new-secret")
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "seta rconPassword \"new-secret\"") {
		t.Fatalf("expected updated quake3 rcon password, got:\n%s", got)
	}
	if strings.Contains(got, "\"old\"") {
		t.Fatalf("expected old password to be removed, got:\n%s", got)
	}
}

func TestConfigureRconUT99INI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "UnrealTournament.ini")
	err := os.WriteFile(path, []byte("[Engine.AccessControl]\nAdminPassword=\nGamePassword=\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = configureRcon("ut99", path, "new-secret")
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "[Engine.AccessControl]\nAdminPassword=new-secret\nGamePassword=") {
		t.Fatalf("expected updated admin password, got:\n%s", got)
	}
}

func TestConfigureRconBZFlagConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.cfg")
	err := os.WriteFile(path, []byte("-advertise NONE\n-j\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = configureRcon("bzflag", path, "new-secret")
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "-passwd new-secret") {
		t.Fatalf("expected appended BZFlag password, got:\n%s", got)
	}
}

func TestApplyRestartConfigWritesTextAndRcon(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.cfg")
	text := "set sv_hostname \"Custom\"\nset sv_maxclients 10"
	err := applyRestartConfig(Config{
		Protocol:     "quake3",
		ConfigPath:   path,
		RconPassword: "new-secret",
	}, restartRequest{
		ConfigText: &text,
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "set sv_hostname \"Custom\"") {
		t.Fatalf("expected custom config content, got:\n%s", got)
	}
	if !strings.Contains(got, "seta rconPassword \"new-secret\"") {
		t.Fatalf("expected rcon password to be restored, got:\n%s", got)
	}
}
