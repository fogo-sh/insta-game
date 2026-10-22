package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

const dataOwnerFile = ".insta-game-owner"

func claimData(directory, owner string) error {
	if owner == "" {
		return nil
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	path := filepath.Join(directory, dataOwnerFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return checkDataOwner(directory, owner)
	}
	if err != nil {
		return err
	}
	_, err = file.WriteString(owner)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func checkDataOwner(directory, owner string) error {
	path := filepath.Join(directory, dataOwnerFile)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("data ownership marker is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if owner == "" || string(data) != owner {
		return fmt.Errorf("data ownership does not match this instance")
	}
	return nil
}

// CleanData is used by a one-off task mounting only the instance's access point.
// Keep the ownership marker until every other entry has been removed, so failed
// deletion can be retried without granting access to a different directory.
func CleanData(directory, owner string) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) == "/" || owner == "" {
		return fmt.Errorf("an absolute instance data directory and owner are required")
	}
	if err := checkDataOwner(directory, owner); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		entries, readErr := os.ReadDir(directory)
		if readErr != nil {
			return readErr
		}
		if len(entries) == 0 {
			return nil
		}
		return fmt.Errorf("refusing to clean nonempty data without its ownership marker")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == dataOwnerFile {
			continue
		}
		if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(filepath.Join(directory, dataOwnerFile))
}
