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

func TestFailedVolumeCloneRollsBackAndAllowsRetry(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	state := filepath.Join(dir, "state")
	t.Setenv("TT_STORAGE_LOG", log)
	t.Setenv("TT_STORAGE_STATE", state)
	fakeTool(t, "zfs", `printf '%s\n' "$*" >> "$TT_STORAGE_LOG"
S="$TT_STORAGE_STATE"
if [ "$1" = get ]; then
 case "$5" in type) printf 'volume\n';; volsize) printf '1048576\n';; esac
 exit 0
fi
if [ "$1" = list ] && [ "$2" = -t ]; then
 [ -e "$S.snap" ] || exit 1
 printf 'tank/base@ttsnap\n'
 exit 0
fi
if [ "$1" = snapshot ]; then touch "$S.snap"; exit 0; fi
if [ "$1" = clone ]; then
 [ -e "$S.clone" ] && { echo 'dataset exists' >&2; exit 1; }
 touch "$S.clone"
 exit 0
fi
if [ "$1" = destroy ]; then rm -f "$S.clone"; exit 0; fi
`)
	deviceFails := true
	store := ZvolStore{waitDevice: func(ctx context.Context, dataset string) (string, error) {
		if deviceFails {
			deviceFails = false
			return "", context.DeadlineExceeded
		}
		return "/dev/zvol/" + dataset, nil
	}}
	ctx := context.Background()
	if err := store.CloneImage(ctx, "tank/base", "tank/runtime/clone-vm"); err == nil {
		t.Fatal("device wait failure hidden")
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "destroy -R tank/runtime/clone-vm") {
		t.Fatalf("partial clone not rolled back:\n%s", data)
	}
	if err := store.CloneImage(ctx, "tank/base", "tank/runtime/clone-vm"); err != nil {
		t.Fatal("retry after rollback did not recover", err)
	}
}
func TestFailedFirecrackerCloneRollsBackEveryCreatedDataset(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	state := filepath.Join(dir, "state")
	base := t.TempDir()
	writeTestFile(t, filepath.Join(base, "vmlinux"), []byte("kernel"))
	t.Setenv("TT_STORAGE_LOG", log)
	t.Setenv("TT_STORAGE_STATE", state)
	t.Setenv("TT_STORAGE_MOUNT", base)
	fakeTool(t, "zfs", `printf '%s\n' "$*" >> "$TT_STORAGE_LOG"
S="$TT_STORAGE_STATE"
if [ "$1" = get ]; then
 case "$5" in
  type) printf 'filesystem\n';;
  mountpoint) printf '%s\n' "$TT_STORAGE_MOUNT";;
  mounted) printf 'yes\n';;
  volsize) printf '1048576\n';;
 esac
 exit 0
fi
if [ "$1" = list ] && [ "$2" = -t ]; then
 case "$4" in
  tank/base@ttsnap) [ -e "$S.snap" ] || exit 1; exit 0;;
  tank/base/rootfs@ttsnap) [ -e "$S.snaproot" ] || exit 1; exit 0;;
 esac
fi
if [ "$1" = snapshot ]; then touch "$S.snap" "$S.snaproot"; exit 0; fi
if [ "$1" = clone ]; then exit 0; fi
if [ "$1" = destroy ]; then exit 0; fi
`)
	// Both clones succeed; the symlink fails because the fake clone never
	// materializes its mount directory. The rollback must destroy the child
	// volume before its parent filesystem.
	store := ZvolStore{waitDevice: func(ctx context.Context, dataset string) (string, error) {
		return "/dev/zvol/" + dataset, nil
	}}
	err := store.CloneImage(context.Background(), "tank/base", "tank/runtime/clone-vm")
	if err == nil {
		t.Fatal("symlink failure hidden")
	}
	data, _ := os.ReadFile(log)
	text := string(data)
	outer := strings.Index(text, "destroy -R tank/runtime/clone-vm\n")
	inner := strings.Index(text, "destroy -R tank/runtime/clone-vm/rootfs")
	if inner < 0 || outer < 0 || inner > outer {
		t.Fatalf("rollback did not destroy children first:\n%s", text)
	}
}

func TestRestoredFirecrackerDatasetCannotAliasSourceDevice(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "rootfs.ext4")
	if err := os.Symlink("/dev/zvol/pool/source/rootfs", link); err != nil {
		t.Fatal(err)
	}
	store := ZvolStore{waitDevice: func(context.Context, string) (string, error) { return "/dev/zvol/pool/restored/rootfs", nil }}
	if err := rewireZvolRoot(context.Background(), store, "pool/restored", dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(link)
	if err != nil || got != "/dev/zvol/pool/restored/rootfs" {
		t.Fatal("restore retained source device alias", got, err)
	}
}
