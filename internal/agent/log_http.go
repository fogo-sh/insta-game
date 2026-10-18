package agent

import (
	"fmt"
	terminal "github.com/buildkite/terminal-to-html/v3"
	"net/http"
	"regexp"
	"strings"
)

// bareAnsi matches ANSI-like sequences that are missing the ESC byte (e.g. q2repro output)
var bareAnsi = regexp.MustCompile(`\[[\d;]*m`)

// toHTML converts a single log line from ANSI escape codes to HTML.
// Bare bracket sequences (missing ESC) are stripped first.
func toHTML(line string) string {
	line = bareAnsi.ReplaceAllString(line, "")
	return strings.TrimRight(string(terminal.Render([]byte(line))), "\n")
}

func logsHandler(c Config) http.HandlerFunc {
	token := c.LogToken
	if token == "" {
		token = c.Token
	}
	return authorizeLogs(token, func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")

		// Flush existing buffered lines first.
		for _, line := range gameLogs.Lines() {
			fmt.Fprintf(w, "data: %s\n\n", formatLog(line, r.URL.Query().Get("format")))
		}
		flusher.Flush()

		// Subscribe to new lines.
		ch, cancel := gameLogs.Subscribe()
		defer cancel()

		for {
			select {
			case line, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", formatLog(line, r.URL.Query().Get("format")))
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}

func formatLog(line, format string) string {
	if format == "text" {
		return line
	}
	return toHTML(line)
}
