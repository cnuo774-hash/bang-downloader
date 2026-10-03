//go:build windows

package engine

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockSession(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{}); err != nil {
		file.Close()
		return nil, fmt.Errorf("另一个 Bang 实例正在使用下载会话: %w", err)
	}
	return file, nil
}
