package command

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fogo-sh/insta-game/internal/securehttp"
)

func TestHTTPSClientPinsControllerToken(t *testing.T) {
	const token = "controller-secret"
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Path != "/api/catalog" {
			t.Errorf("unexpected authenticated request: %s", r.URL.Path)
		}
		w.Write([]byte(`["quake"]`))
	}))
	config, err := securehttp.ServerTLS(token)
	if err != nil {
		t.Fatal(err)
	}
	server.TLS = config
	server.StartTLS()
	defer server.Close()
	var output bytes.Buffer
	t.Setenv("INSTA_GAME_TOKEN", token)
	if err := client(context.Background(), "games", []string{"--url", server.URL}, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("quake")) {
		t.Fatal("missing controller response")
	}
	t.Setenv("INSTA_GAME_TOKEN", "different-controller")
	if err := client(context.Background(), "games", []string{"--url", server.URL}, &output); err == nil {
		t.Fatal("trusted a controller with a different token identity")
	}
}
