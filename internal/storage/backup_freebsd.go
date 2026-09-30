package storage

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func deviceID(info os.FileInfo) uint64 { return uint64(info.Sys().(*syscall.Stat_t).Dev) }
func availableBytes(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return saturatedProduct(uint64(stat.Bavail), uint64(stat.Bsize)), nil
}
func reflink(source, target *os.File) error {
	return fmt.Errorf("backup unsupported: strict file reflinks are only implemented on Linux; use zvol storage for bhyve")
}
