package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- RingLog.Write tests ---------------------------------------------------

func TestRingLog_WriteAppendsCompleteLines(t *testing.T) {
	rl := NewRingLog(10)
	rl.Write([]byte("hello\nworld\n"))

	lines := rl.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "hello")
	}
	if lines[1] != "world" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "world")
	}
}

func TestRingLog_WriteIncompleteLineNotBuffered(t *testing.T) {
	rl := NewRingLog(10)
	rl.Write([]byte("no newline here"))

	lines := rl.Lines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for incomplete line, got %d: %v", len(lines), lines)
	}
}

func TestRingLog_WriteIncompleteLineFlushedOnNextWrite(t *testing.T) {
	rl := NewRingLog(10)
	rl.Write([]byte("hel"))
	rl.Write([]byte("lo\nworld\n"))

	lines := rl.Lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "hello")
	}
}

func TestRingLog_WriteReturnsLenOfInput(t *testing.T) {
	rl := NewRingLog(10)
	input := []byte("test\n")
	n, err := rl.Write(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(input) {
		t.Errorf("n = %d, want %d", n, len(input))
	}
}

// ---- RingLog ring-wrap tests -----------------------------------------------

func TestRingLog_WrapAroundKeepsLastN(t *testing.T) {
	rl := NewRingLog(3)
	for i := 0; i < 5; i++ {
		rl.Write([]byte(fmt.Sprintf("line%d\n", i)))
	}

	lines := rl.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after wrap, got %d: %v", len(lines), lines)
	}
	// Should keep lines 2, 3, 4
	expected := []string{"line2", "line3", "line4"}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], want)
		}
	}
}

func TestRingLog_LinesReturnsSnapshot(t *testing.T) {
	rl := NewRingLog(10)
	rl.Write([]byte("a\nb\n"))

	snap1 := rl.Lines()
	rl.Write([]byte("c\n"))
	snap2 := rl.Lines()

	if len(snap1) != 2 {
		t.Errorf("snap1 len = %d, want 2", len(snap1))
	}
	if len(snap2) != 3 {
		t.Errorf("snap2 len = %d, want 3", len(snap2))
	}
	// snap1 should not be mutated
	if len(snap1) != 2 {
		t.Errorf("snap1 was mutated, now len = %d", len(snap1))
	}
}

// ---- RingLog.Subscribe tests -----------------------------------------------

func TestRingLog_SubscribeReceivesNewLines(t *testing.T) {
	rl := NewRingLog(10)
	ch, cancel := rl.Subscribe()
	defer cancel()

	rl.Write([]byte("hello\n"))

	select {
	case got := <-ch:
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for subscribed line")
	}
}

func TestRingLog_SubscribeCancelStopsDelivery(t *testing.T) {
	rl := NewRingLog(10)
	ch, cancel := rl.Subscribe()
	cancel() // cancel immediately

	rl.Write([]byte("after-cancel\n"))

	// The channel should be closed or no message delivered
	select {
	case msg, ok := <-ch:
		if ok {
			t.Errorf("received message after cancel: %q", msg)
		}
	case <-time.After(50 * time.Millisecond):
		// This is acceptable — channel may just be closed or empty
	}
}

func TestRingLog_MultipleSubscribers(t *testing.T) {
	rl := NewRingLog(10)
	ch1, cancel1 := rl.Subscribe()
	ch2, cancel2 := rl.Subscribe()
	defer cancel1()
	defer cancel2()

	rl.Write([]byte("broadcast\n"))

	for i, ch := range []<-chan string{ch1, ch2} {
		select {
		case got := <-ch:
			if got != "broadcast" {
				t.Errorf("subscriber %d got %q, want %q", i+1, got, "broadcast")
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d timed out", i+1)
		}
	}
}

// ---- Concurrency safety test -----------------------------------------------

func TestRingLog_ConcurrentWritesAndReads(t *testing.T) {
	rl := NewRingLog(100)
	var wg sync.WaitGroup
	const writers = 5
	const linesPerWriter = 20

	// Start a subscriber
	ch, cancel := rl.Subscribe()
	defer cancel()

	// Count received lines
	received := make(chan int, 1)
	go func() {
		count := 0
		timeout := time.After(2 * time.Second)
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					received <- count
					return
				}
				count++
				if count == writers*linesPerWriter {
					received <- count
					return
				}
			case <-timeout:
				received <- count
				return
			}
		}
	}()

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < linesPerWriter; i++ {
				rl.Write([]byte(fmt.Sprintf("writer%d-line%d\n", id, i)))
			}
		}(w)
	}

	wg.Wait()

	total := <-received
	if total != writers*linesPerWriter {
		t.Errorf("received %d lines, want %d", total, writers*linesPerWriter)
	}
}

// ---- logsHandler SSE tests -------------------------------------------------

func TestLogsHandler_RequiresAuth(t *testing.T) {
	c := cfg{Token: "secret"}
	handler := logsHandler(c)

	req := newTestRequest("GET", "/logs", "")
	w := newTestResponseRecorder()
	handler(w, req)

	if w.code != 401 {
		t.Errorf("expected 401 without auth, got %d", w.code)
	}
}

func TestLogsHandler_SSEHeaders(t *testing.T) {
	c := cfg{Token: "secret"}
	handler := logsHandler(c)

	req := newTestRequest("GET", "/logs", "Bearer secret")
	w := newTestResponseRecorder()

	// Run handler in goroutine since SSE blocks until context done
	ctx, cancelCtx := withCancelContext(req)
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		handler(w, req)
		close(done)
	}()

	// Give it a moment to write headers
	time.Sleep(20 * time.Millisecond)
	cancelCtx()
	<-done

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	xab := w.Header().Get("X-Accel-Buffering")
	if xab != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", xab)
	}
}

func TestLogsHandler_FlushesExistingLines(t *testing.T) {
	// Pre-populate gameLogs
	oldLogs := gameLogs
	gameLogs = NewRingLog(200)
	defer func() { gameLogs = oldLogs }()

	gameLogs.Write([]byte("existing-line\n"))

	c := cfg{Token: "tok"}
	handler := logsHandler(c)

	req := newTestRequest("GET", "/logs", "Bearer tok")
	w := newTestResponseRecorder()

	ctx, cancelCtx := withCancelContext(req)
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		handler(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancelCtx()
	<-done

	body := w.body.String()
	if !strings.Contains(body, "data: existing-line") {
		t.Errorf("expected 'data: existing-line' in body, got: %q", body)
	}
}

func TestLogsHandler_StreamsNewLines(t *testing.T) {
	oldLogs := gameLogs
	gameLogs = NewRingLog(200)
	defer func() { gameLogs = oldLogs }()

	c := cfg{Token: "tok"}
	handler := logsHandler(c)

	req := newTestRequest("GET", "/logs", "Bearer tok")
	w := newTestResponseRecorder()

	ctx, cancelCtx := withCancelContext(req)
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		handler(w, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	gameLogs.Write([]byte("streamed-line\n"))
	time.Sleep(50 * time.Millisecond)
	cancelCtx()
	<-done

	body := w.body.String()
	if !strings.Contains(body, "data: streamed-line") {
		t.Errorf("expected 'data: streamed-line' in body, got: %q", body)
	}
}
