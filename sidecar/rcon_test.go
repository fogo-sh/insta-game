package main

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

func TestResolveProtocolPrefersEnv(t *testing.T) {
	t.Setenv("PROTOCOL", "quake2")

	got := resolveProtocol(cfg{
		ProtocolFile: filepath.Join(t.TempDir(), "missing"),
		GameCmd:      "./xonotic-linux-arm64-dedicated",
	})

	if got != "quake2" {
		t.Fatalf("expected env protocol, got %q", got)
	}
}

func TestResolveProtocolReadsProtocolFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protocol.txt")
	if err := os.WriteFile(path, []byte("ut99\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := resolveProtocol(cfg{
		ProtocolFile: path,
		GameCmd:      "./xonotic-linux-arm64-dedicated",
	})

	if got != "ut99" {
		t.Fatalf("expected file protocol, got %q", got)
	}
}

func TestResolveProtocolReadsMetadataFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.json")
	if err := os.WriteFile(path, []byte("{\"protocol\":\"bzflag\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := resolveProtocol(cfg{
		MetadataPath: path,
		ProtocolFile: filepath.Join(t.TempDir(), "missing"),
		GameCmd:      "./xonotic-linux-arm64-dedicated",
	})

	if got != "bzflag" {
		t.Fatalf("expected metadata protocol, got %q", got)
	}
}

func TestResolveProtocolFallsBackToGameCmd(t *testing.T) {
	got := resolveProtocol(cfg{
		MetadataPath: filepath.Join(t.TempDir(), "missing-game-json"),
		ProtocolFile: filepath.Join(t.TempDir(), "missing"),
		GameCmd:      "./fteqw.sv",
	})

	if got != "quake1" {
		t.Fatalf("expected inferred protocol, got %q", got)
	}
}

func TestResolveProtocolFallsBackToIoquake3GameCmd(t *testing.T) {
	got := resolveProtocol(cfg{
		MetadataPath: filepath.Join(t.TempDir(), "missing-game-json"),
		ProtocolFile: filepath.Join(t.TempDir(), "missing"),
		GameCmd:      "./ioq3ded",
	})

	if got != "quake3" {
		t.Fatalf("expected inferred protocol, got %q", got)
	}
}
