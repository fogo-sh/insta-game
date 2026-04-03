package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadData(dataURL, defaultConfigPath, userAgent string) error {
	entries := parseDataURL(dataURL)

	if len(entries) == 0 {
		log.Printf("SIDECAR: No DATA_URL set, using bundled default config: %s", defaultConfigPath)
		return copyFile(defaultConfigPath, "/opt/data/server.cfg")
	}

	for _, e := range entries {
		sentinel := sentinelPath(e.path, e.url)
		if _, err := os.Stat(sentinel); err == nil {
			log.Printf("SIDECAR: Skipping %s (cached)", e.url)
			continue
		}

		log.Printf("SIDECAR: Downloading data from: %s", e.url)
		content, err := fetchWithRetry(e.url, userAgent)
		if err != nil {
			return fmt.Errorf("download %s: %w", e.url, err)
		}

		if isZip(content) {
			dest := e.path
			if dest == "" {
				dest = "/opt/"
			}
			log.Printf("SIDECAR: Extracting zip to %s", dest)
			if err := extractZip(content, dest); err != nil {
				return fmt.Errorf("extract %s: %w", e.url, err)
			}
		} else {
			log.Printf("SIDECAR: Saving file to %s", e.path)
			if err := os.WriteFile(e.path, content, 0644); err != nil {
				return fmt.Errorf("write %s: %w", e.path, err)
			}
		}

		if err := os.WriteFile(sentinel, []byte(e.url), 0644); err != nil {
			log.Printf("SIDECAR: Warning: failed to write cache sentinel: %v", err)
		}
	}
	return nil
}

// sentinelPath returns the path of the cache sentinel file for a given
// destination and URL. The sentinel is named .cache-<sha256-of-url> and lives
// in the destination directory (or /opt/ if dest is empty).
func sentinelPath(dest, url string) string {
	if dest == "" {
		dest = "/opt/"
	}
	// For non-directory destinations (raw file writes), place the sentinel
	// alongside the file in its parent directory.
	if !strings.HasSuffix(dest, "/") {
		dest = filepath.Dir(dest)
	}
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(dest, fmt.Sprintf(".cache-%x", sum[:8]))
}

type dataEntry struct {
	url  string
	path string
}

func parseDataURL(raw string) []dataEntry {
	var out []dataEntry
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		url, path, _ := strings.Cut(part, "=")
		out = append(out, dataEntry{url: strings.TrimSpace(url), path: strings.TrimSpace(path)})
	}
	return out
}

func fetchWithRetry(url, userAgent string) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

	var lastErr error
	for i := 0; i <= len(backoff); i++ {
		if i > 0 {
			time.Sleep(backoff[i-1])
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return readWithProgress(resp.Body, resp.ContentLength, url)
	}
	return nil, lastErr
}

// readWithProgress reads r into memory, logging progress every 5 seconds.
// total is the expected size from Content-Length (-1 if unknown).
func readWithProgress(r io.Reader, total int64, label string) ([]byte, error) {
	var buf bytes.Buffer
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				n := int64(buf.Len())
				if total > 0 {
					log.Printf("SIDECAR: downloading %s — %.1f / %.1f MB (%.0f%%)",
						label,
						float64(n)/1024/1024,
						float64(total)/1024/1024,
						float64(n)/float64(total)*100,
					)
				} else {
					log.Printf("SIDECAR: downloading %s — %.1f MB received",
						label,
						float64(n)/1024/1024,
					)
				}
			}
		}
	}()

	_, err := io.Copy(&buf, r)
	close(done)
	if err != nil {
		return nil, err
	}
	log.Printf("SIDECAR: download complete %s — %.1f MB", label, float64(buf.Len())/1024/1024)
	return buf.Bytes(), nil
}

func isZip(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04
}

func extractZip(data []byte, dest string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
