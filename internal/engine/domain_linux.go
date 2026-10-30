//go:build linux

package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// OpenProcessDomain pins a receiver's payload cgroup outside SQLite state. Its
// identity depends on deployment/receiver identity, never restored VM rows.
func OpenProcessDomain(authority, receiver string) (string, *os.File, error) {
	digest := sha256.Sum256([]byte(authority + "/" + receiver))
	parent := fmt.Sprintf("ttscope-%x", digest)
	lock, err := os.OpenFile(filepath.Join("/run", parent+".lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return "", nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return "", nil, err
	}
	if err = os.MkdirAll(filepath.Join("/sys/fs/cgroup", parent), 0755); err != nil {
		lock.Close()
		return "", nil, err
	}
	controllers, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", parent, "cgroup.controllers"))
	if err != nil {
		lock.Close()
		return "", nil, err
	}
	available := " " + strings.TrimSpace(string(controllers)) + " "
	for _, required := range []string{"cpu", "memory", "pids"} {
		if !strings.Contains(available, " "+required+" ") {
			lock.Close()
			return "", nil, fmt.Errorf("payload cgroup controllers unavailable")
		}
	}
	if err = os.WriteFile(filepath.Join("/sys/fs/cgroup", parent, "cgroup.subtree_control"), []byte("+cpu +memory +pids"), 0600); err != nil {
		lock.Close()
		return "", nil, err
	}
	return parent, lock, nil
}
func WithProcessDomain(implementation Engine, parent string) (Engine, error) {
	if strings.Contains(parent, "/") || !strings.HasPrefix(parent, "ttscope-") {
		return nil, fmt.Errorf("invalid payload cgroup identity")
	}
	switch base := implementation.(type) {
	case *Firecracker:
		copy := *base
		copy.controlParent = parent
		return &copy, nil
	case *Qemu:
		copy := *base
		copy.controlParent = parent
		return &copy, nil
	default:
		return nil, fmt.Errorf("engine has no verified payload cgroup domain")
	}
}
func bindQemuDomain(command *exec.Cmd, parent, id string) (*os.File, error) {
	leaf := filepath.Join("/sys/fs/cgroup", parent, "qemu-"+id)
	if err := os.MkdirAll(leaf, 0755); err != nil {
		return nil, err
	}
	file, err := os.Open(leaf)
	if err != nil {
		return nil, err
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.UseCgroupFD = true
	command.SysProcAttr.CgroupFD = int(file.Fd())
	return file, nil
}
