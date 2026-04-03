package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

func downloadData(dataURL, defaultConfigPath, configPath, userAgent string) error {
	// Sentinels live in the same directory as the game config, which is
	// always on a volume mount and therefore persists across restarts.
	sentinelDir := filepath.Dir(configPath)
	entries := parseDataURL(dataURL)

	if len(entries) == 0 {
		log.Printf("SIDECAR: No DATA_URL set, using bundled default config: %s", defaultConfigPath)
		return copyFile(defaultConfigPath, configPath)
	}

	for _, e := range entries {
		sentinel := sentinelPath(sentinelDir, e.url)
		if _, err := os.Stat(sentinel); err == nil {
			log.Printf("SIDECAR: Skipping %s (cached)", e.url)
			continue
		}

		log.Printf("SIDECAR: Downloading data from: %s", e.url)
		tmpPath, err := fetchWithRetry(e.url, userAgent)
		if err != nil {
			return fmt.Errorf("download %s: %w", e.url, err)
		}

		if zip, err := isZip(tmpPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("check zip %s: %w", e.url, err)
		} else if zip {
			dest := e.path
			if dest == "" {
				dest = "/opt/"
			}
			log.Printf("SIDECAR: Extracting zip to %s", dest)
			if err := extractZip(tmpPath, dest); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("extract %s: %w", e.url, err)
			}
		} else {
			log.Printf("SIDECAR: Saving file to %s", e.path)
			if err := os.Rename(tmpPath, e.path); err != nil {
				// Rename can fail across devices; fall back to copy.
				if err2 := copyFile(tmpPath, e.path); err2 != nil {
					os.Remove(tmpPath)
					return fmt.Errorf("write %s: %w", e.path, err2)
				}
			}
			tmpPath = "" // renamed or copied — no removal needed
		}

		if tmpPath != "" {
			os.Remove(tmpPath)
		}

		if err := os.WriteFile(sentinel, []byte(e.url), 0644); err != nil {
			log.Printf("SIDECAR: Warning: failed to write cache sentinel: %v", err)
		}
	}

	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	log.Printf(
		"SIDECAR: No %s found in downloaded data, using %s",
		filepath.Base(configPath),
		defaultConfigPath,
	)
	return copyFile(defaultConfigPath, configPath)
}

// sentinelPath returns the path of the cache sentinel file for a given URL.
// Sentinels are written to sentinelDir, which should be a volume-mounted
// directory that persists across container restarts.
func sentinelPath(sentinelDir, url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(sentinelDir, fmt.Sprintf(".cache-%x", sum[:8]))
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

// fetchWithRetry downloads url to a temp file and returns its path.
// The caller is responsible for removing the file when done.
func fetchWithRetry(url, userAgent string) (string, error) {
	// Force HTTP/1.1 — HTTP/2 can deadlock under QEMU ARM emulation.
	transport := &http.Transport{
		ForceAttemptHTTP2:   false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{Transport: transport}
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

	ua := userAgent
	if ua == "" {
		ua = "fogo-sh/insta-game"
	}

	var lastErr error
	for i := 0; i <= len(backoff); i++ {
		if i > 0 {
			log.Printf("SIDECAR: retrying %s (attempt %d)", url, i+1)
			time.Sleep(backoff[i-1])
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", ua)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		path, err := streamToFile(resp.Body, resp.ContentLength, url)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return "", lastErr
}

// countingReader wraps an io.Reader and atomically tracks bytes read.
type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// streamToFile streams r to a temp file, logging progress every 5 seconds.
// total is the expected size from Content-Length (-1 if unknown).
// Returns the path of the temp file; the caller must remove it when done.
func streamToFile(r io.Reader, total int64, label string) (string, error) {
	tmp, err := os.CreateTemp("", "sidecar-download-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cr := &countingReader{r: r}
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				n := cr.n.Load()
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

	_, err = io.Copy(tmp, cr)
	close(done)
	tmp.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	log.Printf("SIDECAR: download complete %s — %.1f MB", label, float64(cr.n.Load())/1024/1024)
	return tmpPath, nil
}

// isZip peeks the first 4 bytes of the file to check for a PK zip header.
func isZip(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var magic [4]byte
	n, err := f.Read(magic[:])
	if err != nil && err != io.EOF {
		return false, err
	}
	if n < 4 {
		return false, nil
	}
	return magic[0] == 'P' && magic[1] == 'K' && magic[2] == 0x03 && magic[3] == 0x04, nil
}

// extractZip extracts the zip at zipPath into dest, streaming each entry to disk.
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

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
		mode := f.Mode()
		if mode == 0 {
			mode = 0644
		}
		if chmodErr := out.Chmod(mode); chmodErr != nil {
			out.Close()
			return chmodErr
		}
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
