package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedCopiesExecutableAndPreservesDataAfterRestart(t *testing.T) {
	source := t.TempDir()
	data := t.TempDir()
	engine := filepath.Join(source, "engine")
	if err := os.WriteFile(engine, []byte("engine"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := seedData(context.Background(), source, data); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(data, "engine")
	info, err := os.Stat(copied)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatal("lost engine executable permissions")
	}
	if err := os.WriteFile(copied, []byte("saved changes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := seedData(context.Background(), source, data); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(copied); err != nil || string(contents) != "saved changes" {
		t.Fatalf("restart replaced existing data: %q %v", contents, err)
	}
}

func TestCanceledSeedCanResume(t *testing.T) {
	source := t.TempDir()
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "asset"), []byte("asset"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := seedData(ctx, source, data); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := seedData(context.Background(), source, data); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(data, "asset")); err != nil || string(content) != "asset" {
		t.Fatalf("resume failed: %q %v", content, err)
	}
}
