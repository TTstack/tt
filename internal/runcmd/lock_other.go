//go:build !linux && !freebsd

package runcmd

import (
	"fmt"
	"os"
)

func LockState(path string) (*os.File, error) {
	return nil, fmt.Errorf("state locks are not supported on this platform")
}
