package storage

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestCapacityEnvelopeChargesSparseVirtualSizeWithoutFollowingBacking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sparse.qcow2")
	header := make([]byte, 104)
	copy(header, []byte{'Q', 'F', 'I', 0xfb})
	binary.BigEndian.PutUint32(header[4:8], 3)
	binary.BigEndian.PutUint32(header[20:24], 16)
	virtual := uint64(32 << 30)
	binary.BigEndian.PutUint64(header[24:32], virtual)
	binary.BigEndian.PutUint32(header[100:104], 104)
	binary.BigEndian.PutUint64(header[8:16], 104)
	binary.BigEndian.PutUint32(header[16:20], 32)
	if err := os.WriteFile(path, header, 0600); err != nil {
		t.Fatal(err)
	}
	bytes, known, err := CapacityEnvelope(context.Background(), path)
	if err != nil || !known || bytes < virtual+72<<20 {
		t.Fatal("sparse container length replaced virtual capacity", bytes, known, err)
	}
	binary.BigEndian.PutUint64(header[72:80], 1<<2)
	if err = os.WriteFile(path, header, 0600); err != nil {
		t.Fatal(err)
	}
	_, known, err = CapacityEnvelope(context.Background(), path)
	if err != nil || known {
		t.Fatal("external data dependency was treated as a closed capacity envelope", known, err)
	}
}
func TestFirecrackerCapacityDoesNotInterpretRawGuestBytesAsQcow(t *testing.T) {
	root := t.TempDir()
	payload := make([]byte, 4096)
	copy(payload, []byte{'Q', 'F', 'I', 0xfb})
	if err := os.WriteFile(filepath.Join(root, "rootfs.ext4"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vmlinux"), []byte("kernel"), 0600); err != nil {
		t.Fatal(err)
	}
	bytes, known, err := CapacityEnvelope(context.Background(), root)
	if err != nil || !known || bytes != 4102 {
		t.Fatal(bytes, known, err)
	}
}
func TestStageCapacityDoesNotTraverseUnknownLinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(outside, []byte("private bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "payload")); err != nil {
		t.Fatal(err)
	}
	_, known, err := StagingFootprint(context.Background(), root)
	if err != nil || known {
		t.Fatal("stage link yielded an authoritative size", known, err)
	}
}
