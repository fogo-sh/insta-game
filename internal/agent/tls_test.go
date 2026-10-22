package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentTLSPinsEachInstance(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	config, err := serverTLS("first-instance-token")
	if err != nil {
		t.Fatal(err)
	}
	server.TLS = config
	server.StartTLS()
	defer server.Close()
	for _, token := range []string{"first-instance-token", "other-instance-token", ""} {
		transport := &http.Transport{TLSClientConfig: ClientTLS(token)}
		client := &http.Client{Transport: transport}
		res, err := client.Get(server.URL)
		transport.CloseIdleConnections()
		if token == "first-instance-token" {
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusNoContent {
				t.Fatal(res.Status)
			}
		} else {
			if err == nil {
				res.Body.Close()
				t.Fatal("trusted a different instance identity")
			}
		}
	}
}
