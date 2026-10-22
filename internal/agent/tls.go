package agent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

func agentKey(token string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("insta-game agent TLS\x00" + token))
	return ed25519.NewKeyFromSeed(seed[:])
}

func serverTLS(token string) (*tls.Config, error) {
	key := agentKey(token)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	certificate := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "insta-game agent"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}, nil
}

// ClientTLS pins the instance's key instead of trusting a public certificate
// authority or a hostname that can change when a cloud task restarts.
func ClientTLS(token string) *tls.Config {
	expected, _ := x509.MarshalPKIXPublicKey(agentKey(token).Public())
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // VerifyConnection replaces CA/hostname verification with an instance key pin.
		VerifyConnection: func(state tls.ConnectionState) error {
			if token == "" || len(state.PeerCertificates) == 0 {
				return fmt.Errorf("missing agent TLS identity")
			}
			certificate := state.PeerCertificates[0]
			if !bytes.Equal(certificate.RawSubjectPublicKeyInfo, expected) {
				return fmt.Errorf("agent TLS identity does not match this instance")
			}
			now := time.Now()
			if now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
				return fmt.Errorf("agent TLS certificate is outside its validity period")
			}
			return nil
		},
	}
}
