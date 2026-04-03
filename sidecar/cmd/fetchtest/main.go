package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	url := "https://files.catbox.moe/cr3eol.zip"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	log.Printf("testing fetch of: %s", url)

	// Test 1: default client (HTTP/2 allowed)
	log.Printf("--- Test 1: default http.Client (HTTP/2 allowed) ---")
	testFetch(url, &http.Client{Timeout: 10 * time.Second})

	// Test 2: force HTTP/1.1
	log.Printf("--- Test 2: HTTP/1.1 forced ---")
	testFetch(url, &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2:   false,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	})

	// Test 3: browser User-Agent
	log.Printf("--- Test 3: browser User-Agent ---")
	testFetchWithUA(url, &http.Client{Timeout: 10 * time.Second},
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
}

func testFetch(url string, client *http.Client) {
	testFetchWithUA(url, client, "")
}

func testFetchWithUA(url string, client *http.Client, ua string) {
	start := time.Now()
	log.Printf("  sending request (UA: %q)...", ua)
	req, _ := http.NewRequest("GET", url, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  FAILED after %s: %v", time.Since(start), err)
		return
	}
	defer resp.Body.Close()
	log.Printf("  got response in %s: HTTP %d, Content-Length: %d", time.Since(start), resp.StatusCode, resp.ContentLength)

	buf := make([]byte, 32*1024)
	n, err := resp.Body.Read(buf)
	log.Printf("  first read: %d bytes, err: %v, elapsed: %s", n, err, time.Since(start))

	_, err = io.Copy(io.Discard, resp.Body)
	log.Printf("  drain complete, err: %v, elapsed: %s", err, time.Since(start))
}
