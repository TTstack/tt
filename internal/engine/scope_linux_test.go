//go:build linux

package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestKernelPayloadDomainSurvivesNamespaceRelativeProcessRoot(t *testing.T) {
	proc := t.TempDir()
	root := t.TempDir()
	for index, domain := range []string{"ttscope-owned", "ttscope-foreign"} {
		pid := 100 + index
		directory := filepath.Join(proc, strconv.Itoa(pid))
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "cmdline"), []byte("/firecracker\x00--api-sock\x00/fc-guest.sock\x00"), 0600); err != nil {
			t.Fatal(err)
		}
		fields := make([]string, 20)
		for i := range fields {
			fields[i] = "0"
		}
		fields[0] = "S"
		fields[19] = "1000"
		if err := os.WriteFile(filepath.Join(directory, "stat"), []byte(strconv.Itoa(pid)+" (firecracker) "+strings.Join(fields, " ")), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "cgroup"), []byte("0::/"+domain+"/guest\n"), 0600); err != nil {
			t.Fatal(err)
		}
		// pivot_root exposes '/' rather than a host jail path. Path-based
		// guesses cannot establish which receiver owns this process.
		if err := os.Symlink("/", filepath.Join(directory, "root")); err != nil {
			t.Fatal(err)
		}
	}
	found, err := scopedProcesses(context.Background(), proc, ProcessScope{Roots: []string{root}, Cgroup: "ttscope-owned"})
	if err != nil || len(found) != 1 || found[0].PID != 100 {
		t.Fatal("kernel domain lost ownership or included another receiver", found, err)
	}
}
func TestUnknownKernelDomainReadIsNotAnEmptyInventory(t *testing.T) {
	proc := t.TempDir()
	directory := filepath.Join(proc, "100")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "cmdline"), []byte("/firecracker\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[19] = "1000"
	if err := os.WriteFile(filepath.Join(directory, "stat"), []byte("100 (firecracker) "+strings.Join(fields, " ")), 0600); err != nil {
		t.Fatal(err)
	}
	// A directory is an unreadable identity value, rather than an absent PID.
	if err := os.Mkdir(filepath.Join(directory, "cgroup"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := scopedProcesses(context.Background(), proc, ProcessScope{Roots: []string{t.TempDir()}, Cgroup: "ttscope-owned"}); err == nil {
		t.Fatal("failed kernel identity read became absence")
	}
}
