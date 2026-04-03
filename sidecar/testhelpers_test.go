package main

import (
	"bytes"
	"context"
	"net/http"
	"sync"
)

// testResponseRecorder is a simple http.ResponseWriter + http.Flusher for tests.
type testResponseRecorder struct {
	mu      sync.Mutex
	code    int
	headers http.Header
	body    bytes.Buffer
}

func newTestResponseRecorder() *testResponseRecorder {
	return &testResponseRecorder{
		code:    200,
		headers: make(http.Header),
	}
}

func (r *testResponseRecorder) Header() http.Header {
	return r.headers
}

func (r *testResponseRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(b)
}

func (r *testResponseRecorder) WriteHeader(code int) {
	r.code = code
}

func (r *testResponseRecorder) Flush() {
	// no-op for tests
}

func newTestRequest(method, path, auth string) *http.Request {
	req, _ := http.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return req
}

func withCancelContext(r *http.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(r.Context())
	return ctx, cancel
}
