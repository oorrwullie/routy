package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oorrwullie/routy/internal/models"
)

func TestFindSSHRoute(t *testing.T) {
	configs := []models.SshConfig{
		{Domain: "example.com", Host: "127.0.0.1", Port: 22},
		{Domain: "foo.example.com", Host: "127.0.0.2", Port: 2222},
	}

	route, ok := findSSHRoute(configs, "FOO.EXAMPLE.COM")
	if !ok {
		t.Fatal("expected route match")
	}
	if route.Host != "127.0.0.2" || route.Port != 2222 {
		t.Fatalf("route = %#v, want foo.example.com target", route)
	}

	if _, ok := findSSHRoute(configs, "missing.example.com"); ok {
		t.Fatal("expected no route match")
	}
}

func TestLoadOrCreateSSHSignerPersistsKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ROUTY_DATA_DIR", tmp)

	first, err := loadOrCreateSSHSigner()
	if err != nil {
		t.Fatalf("first signer: %v", err)
	}

	keyPath := filepath.Join(tmp, sshHostKeyFilename)
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected host key file: %v", err)
	}

	second, err := loadOrCreateSSHSigner()
	if err != nil {
		t.Fatalf("second signer: %v", err)
	}

	if string(first.PublicKey().Marshal()) != string(second.PublicKey().Marshal()) {
		t.Fatal("expected persisted host key to be reused")
	}
}
