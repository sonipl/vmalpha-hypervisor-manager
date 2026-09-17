package nativehost

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestResponseValidationAndRedaction(t *testing.T) {
	for _, raw := range []string{`garbage`, `{"ok":true}`, `{"ok":false,"error":"sensitive-secret"}`, `{"ok":true,"result":1} trailing`} {
		_, err := decode([]byte(raw))
		if err == nil || bytes.Contains([]byte(err.Error()), []byte("sensitive-secret")) {
			t.Fatal("invalid or sensitive response accepted", err)
		}
	}
	value, err := decode([]byte(`{"ok":true,"result":{"hostname":"test"}}`))
	if err != nil || string(value) != `{"hostname":"test"}` {
		t.Fatal("valid response rejected", err)
	}
}
func TestBoundedResponse(t *testing.T) {
	var output limitedBuffer
	// A source exposing only Reader exercises io.Copy's destination fast-path.
	source := struct{ io.Reader }{bytes.NewReader(make([]byte, maxResponse+1))}
	if _, err := io.Copy(&output, source); err == nil || !output.exceeded || output.data.Len() > maxResponse {
		t.Fatal("unbounded host response")
	}
}
func TestEnrollmentPinsHostKey(t *testing.T) {
	dir := t.TempDir()
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(private)
	key := filepath.Join(dir, "identity")
	if err := os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	trusted, _ := ssh.NewPublicKey(pub)
	hosts := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(hosts, []byte(knownhosts.Line([]string{"[127.0.0.1]:2222"}, trusted)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Address: "127.0.0.1:2222", User: "manager", PrivateKeyFile: key, KnownHostsFile: hosts}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ssh.HostKeyAlgorithms) != 1 || c.ssh.HostKeyAlgorithms[0] != ssh.KeyAlgoED25519 {
		t.Fatal("host algorithm not pinned to enrollment")
	}
	remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 2222}
	if err := c.ssh.HostKeyCallback(cfg.Address, remote, trusted); err != nil {
		t.Fatal(err)
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	changed, _ := ssh.NewPublicKey(other)
	if err := c.ssh.HostKeyCallback(cfg.Address, remote, changed); err == nil {
		t.Fatal("changed host key accepted")
	}
	if err := os.Chmod(key, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("publicly readable private key accepted")
	}
}
