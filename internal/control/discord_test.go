package control

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestDiscordSignaturesGuildScopeAndIdempotentCreation(t *testing.T) {
	c, _, _ := setup(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c.DiscordKey = public
	c.DiscordGuild = "123"
	handler := c.Handler("controller-token")
	send := func(body []byte, timestamp string, signed bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/discord", bytes.NewReader(body))
		req.Header.Set("X-Signature-Timestamp", timestamp)
		if signed {
			req.Header.Set("X-Signature-Ed25519", hex.EncodeToString(ed25519.Sign(private, append([]byte(timestamp), body...))))
		}
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		return result
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"id":"456","type":2,"guild_id":"123","data":{"name":"create","options":[{"name":"game","value":"xonotic"},{"name":"name","value":"Friends"}]}}`)
	if result := send(body, timestamp, false); result.Code != 401 {
		t.Fatal("accepted unsigned command")
	}
	if result := send(body, strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10), true); result.Code != 401 {
		t.Fatal("accepted stale signature")
	}
	otherGuild := bytes.Replace(body, []byte(`"guild_id":"123"`), []byte(`"guild_id":"999"`), 1)
	if result := send(otherGuild, timestamp, true); result.Code != 403 {
		t.Fatal("accepted another guild")
	}
	for range 2 {
		result := send(body, timestamp, true)
		if result.Code != 200 {
			t.Fatalf("signed command: %d %s", result.Code, result.Body)
		}
		var response struct {
			Type int `json:"type"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Type != 4 {
			t.Fatal("missing immediate interaction response")
		}
	}
	instances, err := c.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 || instances[0].ID != "discord-456" {
		t.Fatal("retried interaction created duplicate instances")
	}
	if _, err := c.discordCommand("789", "stop", map[string]string{"server": "discord-456"}); err != nil {
		t.Fatal(err)
	}
	stored, err := c.Store.Get("discord-456")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Desired != "stopped" {
		t.Fatal("Discord bypassed desired-state flow")
	}
}
