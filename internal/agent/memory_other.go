//go:build !linux && !freebsd

package agent

import "context"

func totalMemory(context.Context) (uint32, bool) { return 0, false }
