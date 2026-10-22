package command

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestInstallSSHNegotiatesTheAuthenticatedHostKey(t *testing.T) {
	_, trustedPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := ssh.NewSignerFromKey(trustedPrivate)
	if err != nil {
		t.Fatal(err)
	}
	otherPrivate, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ssh.NewSignerFromKey(otherPrivate)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(other)
	serverConfig.AddHostKey(trusted)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		server, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			done <- err
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		go func() {
			for ch := range channels {
				ch.Reject(ssh.Prohibited, "test only")
			}
		}()
		done <- server.Wait()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := dialHost(ctx, listener.Addr().String(), trusted, trusted.PublicKey())
	if err != nil {
		t.Fatalf("trusted ED25519 key must be negotiated even when ECDSA is also offered: %v", err)
	}
	client.Close()
	<-done
}

func TestHostTokenDoesNotReuseLocalSession(t *testing.T) {
	t.Setenv("INSTA_GAME_TOKEN", "unrelated-local-controller-token")
	path := filepath.Join(t.TempDir(), "token")
	if err := ensureHostToken(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(data))) < 32 || strings.TrimSpace(string(data)) == os.Getenv("INSTA_GAME_TOKEN") {
		t.Fatal("host requires its own persistent token")
	}
	if err := ensureHostToken(path); err != nil {
		t.Fatal(err)
	}
	next, _ := os.ReadFile(path)
	if string(next) != string(data) {
		t.Fatal("existing host identity changed")
	}
}
