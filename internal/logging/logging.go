package logging

import (
	"log"
	"os"
	"path/filepath"
)

func Setup() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "Bang")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, "bang.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		log.SetOutput(file)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		return path
	}
	return ""
}
