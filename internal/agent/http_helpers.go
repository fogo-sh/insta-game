package agent

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func authorize(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type restartRequest struct {
	ConfigText *string `json:"config_text"`
}

func applyRestartConfig(c Config, req restartRequest) error {
	if req.ConfigText != nil {
		content := strings.ReplaceAll(*req.ConfigText, "\r\n", "\n")
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if err := os.WriteFile(c.ConfigPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		if err := configureRcon(c.Protocol, c.ConfigPath, c.RconPassword); err != nil {
			return fmt.Errorf("configure rcon: %w", err)
		}
	}

	return nil
}
