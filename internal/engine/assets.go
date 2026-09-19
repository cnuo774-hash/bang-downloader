package engine

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const aria2Version = "1.37.0"

func Aria2Version() string { return aria2Version }

// CI replaces the selected placeholder with aria2c 1.37.0 before the release build.
//
//go:embed binaries/*
var aria2Assets embed.FS

func resolveAria2() (string, error) {
	name := "aria2c"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	assetName := fmt.Sprintf("binaries/%s-%s-%s", runtime.GOOS, runtime.GOARCH, name)
	data, err := aria2Assets.ReadFile(assetName)
	if err == nil && !strings.HasPrefix(string(data), "BANG_ARIA2_PLACEHOLDER") {
		return extractAria2(data, name)
	}
	// Source builds remain convenient for contributors; release builds fail in CI if
	// the placeholder was not replaced and always contain the pinned executable.
	path, lookupErr := exec.LookPath(name)
	if lookupErr == nil {
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("当前平台未包含 aria2c 资源 %s: %w", assetName, err)
	}
	return "", errors.New("此开发构建未嵌入 aria2c；请安装 aria2 供开发使用，或使用正式发布版本")
}

func extractAria2(data []byte, name string) (string, error) {
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "Bang", "engine", aria2Version, digest[:16])
	if err := os.MkdirAll(dir, 0o700); err != nil {
		base = os.TempDir()
		dir = filepath.Join(base, "Bang", "engine", aria2Version, digest[:16])
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("创建引擎缓存目录: %w", err)
		}
	}
	path := filepath.Join(dir, name)
	if current, err := os.ReadFile(path); err == nil {
		currentSum := sha256.Sum256(current)
		if currentSum == sum {
			if err := os.Chmod(path, 0o700); err != nil {
				return "", err
			}
			return path, nil
		}
	}
	tmp, err := os.CreateTemp(dir, ".aria2c-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o700); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", err
	}
	return path, nil
}
