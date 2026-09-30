//go:build linux || freebsd

package storage

import (
	"fmt"
	"math"
	"os"

	"golang.org/x/sys/unix"
)

func regular(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	meta, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !meta.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("conflict: backup requires a regular file")
	}
	return file, nil
}
func newFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
}
func fileIdentity(file *os.File) (string, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino), nil
}

// os.FileInfo.Sys is the platform's syscall.Stat_t, not x/sys/unix.Stat_t.
// Device extraction is isolated in per-platform files.
func saturatedProduct(a, b uint64) uint64 {
	if b != 0 && a > math.MaxUint64/b {
		return math.MaxUint64
	}
	return a * b
}
