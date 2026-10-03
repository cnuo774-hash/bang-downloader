package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	SchemaVersion int    `json:"schemaVersion"`
	Output        string `json:"output"`
	RPCSecret     string `json:"rpcSecret"`
	MaxDownload   string `json:"maxDownload,omitempty"`
	EngineVersion string `json:"engineVersion"`
	dirty         bool
	path          string
}

type PublicConfig struct {
	Output        string `json:"output"`
	MaxDownload   string `json:"maxDownload"`
	EngineVersion string `json:"engineVersion"`
}

func (c *Config) Public() PublicConfig {
	return PublicConfig{Output: c.Output, MaxDownload: c.MaxDownload, EngineVersion: c.EngineVersion}
}

func (c *Config) Dirty() bool { return c.dirty }

func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Bang", "config.json"), nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		secret, err := newSecret()
		if err != nil {
			return nil, err
		}
		return &Config{SchemaVersion: 1, Output: filepath.Join(home(), "Downloads"), RPCSecret: secret, EngineVersion: "1.37.0", dirty: true, path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	c.path = path
	if c.SchemaVersion > 1 {
		return nil, fmt.Errorf("配置版本 %d 高于当前程序支持的版本", c.SchemaVersion)
	}
	if c.RPCSecret == "" {
		secret, err := newSecret()
		if err != nil {
			return nil, err
		}
		c.RPCSecret, c.dirty = secret, true
	}
	if c.EngineVersion == "" {
		c.EngineVersion, c.dirty = "1.37.0", true
	}
	if c.Output == "" {
		c.Output, c.dirty = filepath.Join(home(), "Downloads"), true
	}
	if c.SchemaVersion < 1 {
		c.SchemaVersion, c.dirty = 1, true
	}
	return &c, nil
}

func Save(c *Config) error {
	path := c.path
	if path == "" {
		var err error
		path, err = Path()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	c.dirty = false
	return nil
}

func newSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}
