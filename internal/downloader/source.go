package downloader

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type SourceKind string

const (
	SourceURI     SourceKind = "uri"
	SourceTorrent SourceKind = "torrent"
)

type Source struct {
	Kind  SourceKind
	Value string
}

func ParseSource(raw string) (Source, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Source{}, errors.New("下载地址不能为空")
	}
	if strings.ContainsRune(raw, '\x00') {
		return Source{}, errors.New("下载地址包含非法字符")
	}
	parsed, err := url.Parse(raw)
	if err == nil && (parsed.Scheme == "magnet" || parsed.Scheme == "http" || parsed.Scheme == "https") {
		if parsed.Scheme == "magnet" && parsed.Query().Get("xt") == "" {
			return Source{}, errors.New("磁力链接缺少 xt 参数")
		}
		return Source{Kind: SourceURI, Value: raw}, nil
	}
	path, err := expandPath(raw)
	if err != nil {
		return Source{}, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".torrent") {
		return Source{}, errors.New("本地来源必须是 .torrent 文件")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Source{}, fmt.Errorf("读取种子文件: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Source{}, errors.New("种子路径不是普通文件")
	}
	return Source{Kind: SourceTorrent, Value: path}, nil
}

func ResolveOutput(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("保存目录不能为空")
	}
	if strings.ContainsRune(raw, '\x00') {
		return "", errors.New("保存目录包含非法字符")
	}
	path, err := expandPath(raw)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return "", errors.New("保存路径已存在且不是目录")
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("检查保存目录: %w", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("创建保存目录: %w", err)
	}
	return path, nil
}

func expandPath(raw string) (string, error) {
	if raw == "~" || strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if raw == "~" {
			raw = home
		} else {
			raw = filepath.Join(home, raw[2:])
		}
	}
	abs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", fmt.Errorf("解析路径: %w", err)
	}
	return abs, nil
}
