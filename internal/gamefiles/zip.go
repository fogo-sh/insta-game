package gamefiles

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip confines game data to destination, including existing symlinks.
func ExtractZip(ctx context.Context, path, destination string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	if err = os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	var size uint64
	for _, entry := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if !filepath.IsLocal(name) || entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe asset archive path %q", entry.Name)
		}
		if entry.UncompressedSize64 > 8<<30-size {
			return fmt.Errorf("asset archive exceeds 8 GiB")
		}
		size += entry.UncompressedSize64
		if entry.FileInfo().IsDir() {
			if err = makeDirectories(root, name); err != nil {
				return err
			}
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("unsupported archive file %q", entry.Name)
		}
		if err = makeDirectories(root, filepath.Dir(name)); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode().Perm()|0444)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, contextReader{ctx, io.LimitReader(input, int64(entry.UncompressedSize64)+1)})
		input.Close()
		modeErr := output.Chmod(entry.Mode().Perm() | 0444)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if modeErr != nil {
			return modeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func makeDirectories(root *os.Root, path string) error {
	if err := root.MkdirAll(path, 0755); err != nil {
		return err
	}
	current := ""
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := root.Chmod(current, 0755); err != nil {
			return err
		}
	}
	return nil
}
