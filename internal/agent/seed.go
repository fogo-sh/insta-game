package agent

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func seedData(ctx context.Context, source, destination string) error {
	return seedFiles(ctx, source, destination, ".insta-game-seeded")
}
func seedFiles(ctx context.Context, source, destination, markerName string) error {
	if source == "" {
		return nil
	}
	marker := filepath.Join(destination, markerName)
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(destination) || filepath.Clean(source) == filepath.Clean(destination) {
		return fmt.Errorf("seed and data directories must be different absolute paths")
	}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if _, err := os.Lstat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported seed file %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		info, err := entry.Info()
		if err != nil {
			return err
		}
		temporary, err := os.CreateTemp(filepath.Dir(target), ".insta-game-seed-")
		if err != nil {
			return err
		}
		defer os.Remove(temporary.Name())
		_, copyErr := io.Copy(temporary, input)
		modeErr := temporary.Chmod(info.Mode().Perm())
		closeErr := temporary.Close()
		if copyErr != nil {
			return copyErr
		}
		if modeErr != nil {
			return modeErr
		}
		if closeErr != nil {
			return closeErr
		}
		return os.Rename(temporary.Name(), target)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(source), 0600)
}
