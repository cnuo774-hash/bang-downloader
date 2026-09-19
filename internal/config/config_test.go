package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesAndPersistsSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RPCSecret) != 64 || !cfg.Dirty() {
		t.Fatalf("unexpected generated config: secret length=%d dirty=%v", len(cfg.RPCSecret), cfg.Dirty())
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.RPCSecret != cfg.RPCSecret {
		t.Fatal("RPC secret changed after reload")
	}
}
