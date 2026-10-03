//go:build !windows

package engine

import (
	"fmt"
	"os"
	"syscall"
)

func lockSession(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("另一个 Bang 实例正在使用下载会话: %w", err)
	}
	return file, nil
}
