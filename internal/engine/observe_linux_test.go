//go:build linux

package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadOnlyPIDObservationDoesNotRepairFilesAndRetainsUnreadableIdentity(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(t.TempDir(), "guest.pid")
	marker := "/owned/runtime/guest.pid"
	if err := os.Mkdir(filepath.Join(root, "41"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "41", "cmdline"), []byte("qemu\x00"+marker+"\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	processes := processes{root: root}
	pid, err := processes.observePID(context.Background(), pidPath, []string{marker})
	if err != nil || pid != 41 {
		t.Fatal(pid, err)
	}
	if _, err = os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatal("read-only recovery repaired a PID file", err)
	}
	if err = os.WriteFile(filepath.Join(root, "41", "cmdline"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "41", "stat"), []byte("41 (qemu) S 0 0 0 0 0 0"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = processes.observePID(context.Background(), pidPath, []string{marker}); err == nil {
		t.Fatal("unreadable userspace process became absence")
	}
	if err = os.WriteFile(filepath.Join(root, "41", "stat"), []byte("41 (kthread) S 0 0 0 0 0 2097152"), 0600); err != nil {
		t.Fatal(err)
	}
	pid, err = processes.observePID(context.Background(), pidPath, []string{marker})
	if err != nil || pid != 0 {
		t.Fatal("kernel thread was mistaken for owned guest uncertainty", pid, err)
	}
}
