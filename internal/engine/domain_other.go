//go:build !linux

package engine

import (
	"fmt"
	"os"
)

func OpenProcessDomain(string, string) (string, *os.File, error) {
	return "", nil, fmt.Errorf("payload process domains unsupported")
}
func WithProcessDomain(Engine, string) (Engine, error) {
	return nil, fmt.Errorf("payload process domains unsupported")
}
