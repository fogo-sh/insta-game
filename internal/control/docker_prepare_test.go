package control

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDockerPreparationCoalescesAndReportsStreamFailure(t *testing.T) {
	d := NewDocker("unused", "localhost", "127.0.0.1")
	defer d.Close()
	var pulls atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	d.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("missing"))}, nil
		}
		pulls.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		body := "{}\n"
		if fail.Load() {
			body = "{\"error\":\"registry unavailable\"}\n"
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	instance := Instance{Game: Game{ID: "quake", Image: "image"}, StartedAt: time.Now()}
	if ready, err := d.Prepare(context.Background(), instance); err != nil || ready {
		t.Fatal(ready, err)
	}
	<-entered
	for range 10 {
		if ready, err := d.Prepare(context.Background(), instance); err != nil || ready {
			t.Fatal(ready, err)
		}
	}
	if pulls.Load() != 1 {
		t.Fatal("concurrent servers started duplicate pulls")
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		ready, err := d.Prepare(context.Background(), instance)
		if err != nil {
			if !strings.Contains(err.Error(), "registry unavailable") {
				t.Fatal(err)
			}
			break
		}
		if ready || time.Now().After(deadline) {
			t.Fatal("image stream failure was hidden")
		}
		time.Sleep(time.Millisecond)
	}
	fail.Store(false)
	instance.StartedAt = time.Now().Add(time.Second)
	if ready, err := d.Prepare(context.Background(), instance); err != nil || ready {
		t.Fatal("explicit restart did not retry", ready, err)
	}
	<-entered
	deadline = time.Now().Add(time.Second)
	for {
		ready, err := d.Prepare(context.Background(), instance)
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("successful retry never completed")
		}
		time.Sleep(time.Millisecond)
	}
	if pulls.Load() != 2 {
		t.Fatal("retry did not pull exactly once")
	}
}
func TestDockerCloseCancelsDownload(t *testing.T) {
	d := NewDocker("unused", "localhost", "127.0.0.1")
	entered := make(chan struct{})
	d.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	_, err := d.Prepare(context.Background(), Instance{Game: Game{ID: "quake", Image: "image"}})
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	done := make(chan struct{})
	go func() { d.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("controller shutdown left image download running")
	}
}
