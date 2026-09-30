//go:build linux || freebsd

package runcmd

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// LockState holds a nonblocking exclusive lock until its returned file is closed.
// A backup worker may pass that file in exec.Cmd.ExtraFiles to retain the lock in
// children that survive an uncertain command outcome.
func LockState(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("open service lock: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("state directory is already in use or cannot be locked: %w", err)
	}
	return f, nil
}
