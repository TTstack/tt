package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZvolCloneAndResizeCommandPlan(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	t.Setenv("TT_STORAGE_LOG", log)
	fakeTool(t, "zfs", `printf '%s\n' "$*" >> "$TT_STORAGE_LOG"
if [ "$1" = get ]; then
 case "$5" in type) printf 'volume\n';; volsize) printf '1048576\n';; esac
fi
if [ "$1" = list ] && [ "$2" = -t ]; then exit 1; fi
`)
	store := ZvolStore{waitDevice: func(ctx context.Context, dataset string) (string, error) {
		if dataset != "tank/runtime/clone-vm" {
			t.Fatal(dataset)
		}
		return "/dev/zvol/" + dataset, nil
	}}
	ctx := context.Background()
	if err := store.CloneImage(ctx, "tank/base", "tank/runtime/clone-vm"); err != nil {
		t.Fatal(err)
	}
	if err := store.ResizeDisk(ctx, "tank/runtime/clone-vm", 2); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	want := "get -Hp -o value type tank/base\nlist -t snapshot tank/base@ttsnap\nsnapshot tank/base@ttsnap\nclone -o readonly=off -o volmode=dev tank/base@ttsnap tank/runtime/clone-vm\nget -Hp -o value volsize tank/runtime/clone-vm\nset volsize=2097152 tank/runtime/clone-vm\n"
	if string(data) != want {
		t.Fatalf("%s", data)
	}
	if err := store.ResizeDisk(ctx, "tank/runtime/clone-vm", 0); err == nil {
		t.Fatal("shrink accepted")
	}
}
func TestIncompleteFirecrackerSnapshotDoesNotClone(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "vmlinux"), []byte("kernel"))
	log := filepath.Join(dir, "calls")
	t.Setenv("TT_STORAGE_LOG", log)
	t.Setenv("TT_STORAGE_MOUNT", dir)
	fakeTool(t, "zfs", `printf '%s\n' "$*" >> "$TT_STORAGE_LOG"
if [ "$1" = get ]; then
 case "$5" in type) printf 'filesystem\n';; mountpoint) printf '%s\n' "$TT_STORAGE_MOUNT";; mounted) printf 'yes\n';; volsize) printf '1048576\n';; esac
fi
if [ "$1" = list ] && [ "$4" = 'tank/base/rootfs@ttsnap' ]; then exit 1; fi
`)
	err := (ZvolStore{}).CloneImage(context.Background(), "tank/base", "tank/runtime/clone-vm")
	if err == nil || !strings.Contains(err.Error(), "incomplete Firecracker base snapshot") {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "clone -") {
		t.Fatal("cloned incomplete snapshot")
	}
}
