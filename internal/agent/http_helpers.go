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

func authorizeLogs(logToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		provided := r.URL.Query().Get("token")
		if provided == "" {
			auth := r.Header.Get("Authorization")
			provided = strings.TrimPrefix(auth, "Bearer ")
		}
		if provided == "" || provided != logToken {
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
	ConfigURL  *string `json:"config_url"`
	ConfigText *string `json:"config_text"`
}

func applyRestartConfig(c Config, req restartRequest) error {
	if req.ConfigText != nil {
		content := strings.ReplaceAll(*req.ConfigText, "\r\n", "\n")
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if err := os.WriteFile(c.ConfigPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	if req.ConfigURL != nil && strings.TrimSpace(*req.ConfigURL) != "" {
		if err := downloadData(*req.ConfigURL, c.DefaultConfig, c.ConfigPath, c.UserAgent); err != nil {
			return fmt.Errorf("download config: %w", err)
		}
	}

	if req.ConfigText != nil || (req.ConfigURL != nil && strings.TrimSpace(*req.ConfigURL) != "") {
		if err := configureRcon(c.Protocol, c.ConfigPath, c.RconPassword); err != nil {
			return fmt.Errorf("configure rcon: %w", err)
		}
	}

	return nil
}
