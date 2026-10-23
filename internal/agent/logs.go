package agent

import (
	"strings"
	"sync"
)

// RingLog is a thread-safe ring buffer that stores the last N log lines and
// fans them out to SSE subscribers.
type RingLog struct {
	mu   sync.Mutex
	buf  []string
	head int // index of the oldest entry (wraps)
	size int

	partial   string // incomplete line fragment waiting for a newline
	pendingCR bool   // previous write ended with \r, so skip a leading \n next write

	subs map[chan string]struct{}
}

func NewRingLog(capacity int) *RingLog {
	return &RingLog{
		buf:  make([]string, capacity),
		subs: make(map[chan string]struct{}),
	}
}

// Write implements io.Writer. It splits p on newlines and appends each
// complete line to the ring. An incomplete trailing fragment is held until
// the next Write completes it.
func (r *RingLog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	chunk := string(p)
	if r.pendingCR {
		chunk = strings.TrimPrefix(chunk, "\n")
		r.pendingCR = false
	}
	text := r.partial + chunk

	start := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\n':
			r.appendLocked(text[start:i])
			start = i + 1
		case '\r':
			r.appendLocked(text[start:i])
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}

	r.partial = text[start:]
	if len(text) > 0 && text[len(text)-1] == '\r' {
		r.pendingCR = true
		r.partial = ""
	}

	return len(p), nil
}

// Caller must hold r.mu.
func (r *RingLog) appendLocked(line string) {
	if r.size < len(r.buf) {
		r.buf[(r.head+r.size)%len(r.buf)] = line
		r.size++
	} else {
		r.buf[r.head] = line
		r.head = (r.head + 1) % len(r.buf)
	}

	for ch := range r.subs {
		select {
		case ch <- line:
		default:
			// Slow reader: drop the line rather than block the writer.
		}
	}
}

// Subscribe captures history and registers live delivery under one lock, so no
// lines fall between replay and subscription. Network writes happen after return.
func (r *RingLog) Subscribe() ([]string, <-chan string, func()) {
	ch := make(chan string, 64)
	r.mu.Lock()
	history := make([]string, r.size)
	for i := range history {
		history[i] = r.buf[(r.head+i)%len(r.buf)]
	}
	r.subs[ch] = struct{}{}
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.subs, ch)
			r.mu.Unlock()
			close(ch)
		})
	}
	return history, ch, cancel
}
