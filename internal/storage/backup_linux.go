package storage

import (
	"errors"
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
	return saturatedProduct(stat.Bavail, uint64(stat.Frsize)), nil
}
func reflink(source, target *os.File) error {
	err := unix.IoctlSetInt(int(target.Fd()), unix.FICLONE, int(source.Fd()))
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EXDEV) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTTY) {
		return fmt.Errorf("backup unsupported: filesystem cannot create a strict reflink (%s)", err)
	}
	return err
}
