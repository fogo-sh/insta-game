package gamefiles

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func archiveWith(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "assets.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0755)
	entry, err := archive.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	return path
}
func TestExtractionRejectsTraversalAndExistingEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "game")
	os.Mkdir(destination, 0755)
	for _, name := range []string{"../outside", "/outside", "link/outside"} {
		if name == "link/outside" {
			if err := os.Symlink(root, filepath.Join(destination, "link")); err != nil {
				t.Fatal(err)
			}
		}
		if err := ExtractZip(context.Background(), archiveWith(t, name, "overwrite"), destination); err == nil {
			t.Fatal("unsafe archive extracted", name)
		}
	}
	contents, _ := os.ReadFile(outside)
	if string(contents) != "keep" {
		t.Fatal("archive wrote outside game directory")
	}
}
func TestExtractionKeepsExecutableAssetsAndCanBeCancelled(t *testing.T) {
	oldMask := syscall.Umask(0077)
	defer syscall.Umask(oldMask)
	path := archiveWith(t, "System/engine", "game")
	destination := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExtractZip(ctx, path, destination); err == nil {
		t.Fatal("cancelled extraction succeeded")
	}
	if err := ExtractZip(context.Background(), path, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "System/engine"))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.Stat(filepath.Join(destination, "System"))
	if err != nil {
		t.Fatal(err)
	}
	if directory.Mode().Perm() != 0755 {
		t.Fatal("host umask made assets unreadable to game user")
	}
	if info.Mode().Perm() != 0755 {
		t.Fatal("engine lost executable permissions")
	}
}
