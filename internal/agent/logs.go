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
	cap  int
	head int // index of the oldest entry (wraps)
	size int // number of valid entries (<= cap)

	partial   string // incomplete line fragment waiting for a newline
	pendingCR bool   // previous write ended with \r, so skip a leading \n next write

	subs map[int]chan string
	next int
}

func NewRingLog(capacity int) *RingLog {
	return &RingLog{
		buf:  make([]string, capacity),
		cap:  capacity,
		subs: make(map[int]chan string),
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

// appendLocked inserts a line into the ring and notifies subscribers.
// Caller must hold r.mu.
func (r *RingLog) appendLocked(line string) {
	if r.size < r.cap {
		r.buf[(r.head+r.size)%r.cap] = line
		r.size++
	} else {
		r.buf[r.head] = line
		r.head = (r.head + 1) % r.cap
	}

	for _, ch := range r.subs {
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
		history[i] = r.buf[(r.head+i)%r.cap]
	}
	id := r.next
	r.next++
	r.subs[id] = ch
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.subs, id)
			r.mu.Unlock()
			close(ch)
		})
	}
	return history, ch, cancel
}
