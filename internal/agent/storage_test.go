package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanDataRequiresOwnershipAndDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "instance")
	outside := filepath.Join(root, "other-instance")
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(outside, "saved")
	if err := os.WriteFile(saved, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := claimData(data, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(data, "link")); err != nil {
		t.Fatal(err)
	}
	if err := CleanData(data, "other-owner"); err == nil {
		t.Fatal("deleted another instance's data")
	}
	if err := CleanData(data, "owner"); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(saved); err != nil || string(content) != "keep" {
		t.Fatalf("followed symlink: %q %v", content, err)
	}
	if err := CleanData(data, "owner"); err != nil {
		t.Fatal("empty cleanup retry failed", err)
	}
	if err := os.WriteFile(filepath.Join(data, "unowned"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CleanData(data, "owner"); err == nil {
		t.Fatal("deleted nonempty data without an ownership marker")
	}
}
