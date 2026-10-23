package agent

import (
	"fmt"
	"net/http"
)

func logsHandler(c Config) http.HandlerFunc {
	return authorize(c.Token, func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		history, updates, cancel := gameLogs.Subscribe()
		defer cancel()
		for _, line := range history {
			fmt.Fprintf(w, "data: %s\n\n", line)
		}
		flusher.Flush()
		for {
			select {
			case line, ok := <-updates:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}
