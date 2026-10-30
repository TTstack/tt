//go:build linux

package engine

import (
	"testing"
	"time"
)

// SetStopGrace(0) must reach newly constructed engines: zero means no
// graceful wait at all, the engine-specific default must survive a -1,
// and out-of-range values clamp to the default.
func TestStopGracePlumbing(t *testing.T) {
	defer SetStopGrace(-1)

	if got := newFirecracker().stopGrace; got != fcDefaultStopGrace {
		t.Fatalf("default grace: got %v", got)
	}
	SetStopGrace(0)
	if got := newFirecracker().stopGrace; got != 0 {
		t.Fatalf("grace 0: got %v", got)
	}
	if got := newQemu().stopGrace; got != 0 {
		t.Fatalf("grace 0 qemu: got %v", got)
	}
	SetStopGrace(7)
	if got := newQemu().stopGrace; got != 7*time.Second {
		t.Fatalf("grace 7 qemu: got %v", got)
	}
	SetStopGrace(-42)
	if got := newFirecracker().stopGrace; got != fcDefaultStopGrace {
		t.Fatalf("clamped grace: got %v", got)
	}
}
