package agent

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

func logSnapshot(r *RingLog) []string {
	history, _, cancel := r.Subscribe()
	cancel()
	return history
}

func TestRingLogFraming(t *testing.T) {
	for _, tc := range []struct {
		name         string
		chunks, want []string
	}{
		{"LF", []string{"hello\nworld\n"}, []string{"hello", "world"}},
		{"partial", []string{"no newline"}, []string{}},
		{"fragmented", []string{"hel", "lo\nworld\n"}, []string{"hello", "world"}},
		{"CR", []string{"hello\rworld\r"}, []string{"hello", "world"}},
		{"split CRLF", []string{"hello\r", "\nworld\r\n"}, []string{"hello", "world"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRingLog(10)
			for _, chunk := range tc.chunks {
				if n, err := r.Write([]byte(chunk)); n != len(chunk) || err != nil {
					t.Fatalf("Write = %d, %v", n, err)
				}
			}
			if got := logSnapshot(r); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("lines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRingLogReplayAndSubscribers(t *testing.T) {
	r := NewRingLog(2)
	r.Write([]byte("a\nb\n"))
	history, first, cancelFirst := r.Subscribe()
	defer cancelFirst()
	_, second, cancelSecond := r.Subscribe()
	defer cancelSecond()
	r.Write([]byte("c\n"))
	if !reflect.DeepEqual(history, []string{"a", "b"}) {
		t.Fatalf("replay changed after wrap: %q", history)
	}
	history[0] = "mutated"
	if got := logSnapshot(r); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("wrapped buffer = %q", got)
	}
	for _, ch := range []<-chan string{first, second} {
		select {
		case got := <-ch:
			if got != "c" {
				t.Fatalf("live line = %q", got)
			}
		default:
			t.Fatal("missing live line")
		}
	}
	cancelFirst()
	cancelFirst()
	r.Write([]byte("d\n"))
	if _, ok := <-first; ok {
		t.Fatal("canceled subscriber remains open")
	}
	select {
	case got := <-second:
		if got != "d" {
			t.Fatalf("remaining subscriber = %q", got)
		}
	default:
		t.Fatal("cancel disconnected the other subscriber")
	}
}

func TestRingLogConcurrentWritersWithSlowSubscriber(t *testing.T) {
	r := NewRingLog(100)
	_, _, _ = r.Subscribe()
	var wg sync.WaitGroup
	for w := range 5 {
		wg.Go(func() {
			for i := range 20 {
				r.Write(fmt.Appendf(nil, "writer%d-line%d\n", w, i))
				logSnapshot(r)
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("slow subscriber blocked writers")
	}
	if got := len(logSnapshot(r)); got != 100 {
		t.Fatalf("buffered %d lines, want 100", got)
	}
}

func TestLogsHandlerStreaming(t *testing.T) {
	old := gameLogs
	gameLogs = NewRingLog(10)
	defer func() { gameLogs = old }()
	server := httptest.NewServer(logsHandler(Config{Token: "secret"}))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", res.StatusCode)
	}

	gameLogs = NewRingLog(10)
	gameLogs.Write([]byte("<replay>\n"))
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	for key, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := res.Header.Get(key); got != want {
			t.Fatalf("%s = %q", key, got)
		}
	}
	scanner := bufio.NewScanner(res.Body)
	readEvent := func(want string) {
		t.Helper()
		if !scanner.Scan() || scanner.Text() != "data: "+want {
			t.Fatalf("event = %q, error = %v; want %q", scanner.Text(), scanner.Err(), want)
		}
		if !scanner.Scan() || scanner.Text() != "" {
			t.Fatal("missing event separator")
		}
	}
	replay, live := "<replay>", "<live>"
	readEvent(replay)
	gameLogs.Write([]byte("<live>\n"))
	readEvent(live)
}
