package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

func TestSaveUsesLoadedPathAndRestrictsSecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom", "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Dirty() {
		t.Fatal("saved config is still dirty")
	}
	reloaded, err := Load(path)
	if err != nil || reloaded.RPCSecret != cfg.RPCSecret {
		t.Fatalf("config was not persisted at loaded path: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("RPC secret file is not private")
	}
}

func TestFutureConfigCannotBeOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":2,"future":"keep"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("accepted unknown schema")
	}
	data, _ := os.ReadFile(path)
	if string(data) != `{"schemaVersion":2,"future":"keep"}` {
		t.Fatal("future config changed")
	}
}
