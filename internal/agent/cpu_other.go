//go:build !linux

package agent

import "runtime"

func availableCPU() uint32 {
	count := runtime.NumCPU()
	if count < 1 {
		return 4
	}
	return uint32(count)
}
