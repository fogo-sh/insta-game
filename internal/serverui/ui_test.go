package serverui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/launchui"
)

func TestBothPagesServeSharedStatusModule(t *testing.T) {
	controller := control.NewController(nil, nil, nil, "")
	for name, handler := range map[string]http.Handler{"controller": controller.Handler("private"), "launcher": launchui.Handler()} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/status.js", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "javascript") || !strings.Contains(response.Body.String(), "export function serverState") {
				t.Fatalf("status module unavailable: %d %s", response.Code, response.Header().Get("Content-Type"))
			}
		})
	}
}
