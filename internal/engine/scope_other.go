//go:build !linux

package engine

import (
	"context"
	"fmt"
)

type ProcessScope struct {
	KernelBootID string `json:",omitempty"`
	Roots        []string
	Dataset      string
	Cgroup       string
}

func KernelBootIdentity() (string, error) {
	return "", fmt.Errorf("kernel boot fencing unsupported on this platform")
}

type ProcessEvidence struct {
	PID    int    `json:"pid"`
	Start  uint64 `json:"start"`
	Digest string `json:"digest"`
	Class  string `json:"class"`
}

func ScopedProcesses(context.Context, ProcessScope) ([]ProcessEvidence, error) {
	return nil, fmt.Errorf("physical process fencing unsupported on this platform")
}
func StopScopedProcess(context.Context, ProcessEvidence, ProcessScope) error {
	return fmt.Errorf("physical process fencing unsupported on this platform")
}
