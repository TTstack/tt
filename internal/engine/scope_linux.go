//go:build linux

package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// ProcessScope binds physical fencing to private namespace roots. Process
// arguments are never returned: inventory exposes only identity digests.
type ProcessScope struct {
	KernelBootID string `json:",omitempty"`
	Roots        []string
	Dataset      string
	Cgroup       string
}

func KernelBootIdentity() (string, error) {
	file, err := os.Open("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 129))
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if len(raw) > 128 || len(value) != 36 {
		return "", fmt.Errorf("kernel boot identity unavailable")
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", fmt.Errorf("invalid kernel boot identity")
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", fmt.Errorf("invalid kernel boot identity")
		}
	}
	return value, nil
}

type ProcessEvidence struct {
	PID    int    `json:"pid"`
	Start  uint64 `json:"start"`
	Digest string `json:"digest"`
	Class  string `json:"class"`
}

const maxProcessEntries = 131072
const maxOwnedProcesses = 4096

func boundedProcessFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return nil, err
	}
	if len(data) > 65536 {
		return nil, fmt.Errorf("process identity exceeds its budget")
	}
	return data, nil
}
func readProcessEvidence(root string, pid int) (ProcessEvidence, []string, error) {
	evidence := ProcessEvidence{PID: pid}
	directory := filepath.Join(root, strconv.Itoa(pid))
	raw, err := boundedProcessFile(filepath.Join(directory, "cmdline"))
	if err != nil {
		return evidence, nil, err
	}
	stat, err := boundedProcessFile(filepath.Join(directory, "stat"))
	if err != nil {
		return evidence, nil, err
	}
	index := bytes.LastIndex(stat, []byte(") "))
	if index < 0 {
		return evidence, nil, fmt.Errorf("malformed process stat")
	}
	fields := strings.Fields(string(stat[index+2:]))
	if len(fields) < 20 {
		return evidence, nil, fmt.Errorf("malformed process stat")
	}
	if fields[0] == "Z" || fields[0] == "X" {
		return evidence, nil, nil
	}
	evidence.Start, err = strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return evidence, nil, err
	}
	if len(raw) == 0 {
		return evidence, nil, nil
	}
	parts := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		args = append(args, string(part))
	}
	evidence.Digest = fmt.Sprintf("%x", sha256.Sum256(raw))
	return evidence, args, nil
}
func processClass(args []string) string {
	if len(args) == 0 {
		return ""
	}
	binary := filepath.Base(args[0])
	if strings.HasPrefix(binary, "qemu-system-") {
		return "qemu"
	}
	if strings.HasPrefix(binary, "firecracker") || strings.HasPrefix(binary, "jailer") {
		return "firecracker"
	}
	switch binary {
	case "qemu-img", "cp", "resize2fs", "e2fsck", "zfs", "dd":
		return "storage"
	}
	return ""
}
func scopeArgument(argument, root string) bool {
	return argument == root || strings.HasPrefix(argument, root+"/") || strings.Contains(argument, "="+root+"/") || strings.Contains(argument, ","+root+"/")
}
func seesScopeRoot(procRoot string, pid int, root string) (bool, error) {
	own, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	observed, err := os.Stat(filepath.Join(procRoot, strconv.Itoa(pid), "root", strings.TrimLeft(root, "/")))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(own, observed), nil
}
func ownedProcess(procRoot string, pid int, args []string, scope ProcessScope) (bool, error) {
	if scope.Cgroup != "" {
		raw, err := boundedProcessFile(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup"))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			_, path, ok := strings.Cut(line, "::")
			if ok && (path == "/"+scope.Cgroup || strings.HasPrefix(path, "/"+scope.Cgroup+"/")) {
				return true, nil
			}
		}
	}
	class := processClass(args)
	if class == "" {
		return false, nil
	}
	if len(scope.Roots) == 0 {
		return false, fmt.Errorf("physical process scope is empty")
	}
	if class == "firecracker" {
		rootPath, err := os.Readlink(filepath.Join(procRoot, strconv.Itoa(pid), "root"))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		for _, root := range scope.Roots {
			relative := strings.TrimPrefix(rootPath, root+"/")
			if relative == rootPath || (!strings.HasPrefix(relative, ".jailer/") && !strings.Contains(relative, "/.jailer/")) {
				continue
			}
			observed, err := os.Stat(filepath.Join(procRoot, strconv.Itoa(pid), "root"))
			if err != nil {
				return false, err
			}
			own, err := os.Stat(strings.TrimSuffix(rootPath, " (deleted)"))
			if err != nil {
				return false, fmt.Errorf("jail scope cannot be established")
			}
			return os.SameFile(observed, own), nil
		}
	}
	candidates := args[1:]
	if class == "storage" {
		if len(args) < 2 {
			return false, nil
		}
		if filepath.Base(args[0]) == "zfs" {
			switch args[1] {
			case "create", "clone", "destroy", "rename", "set", "snapshot", "rollback", "receive":
			default:
				return false, nil
			}
			for _, arg := range args[2:] {
				if scope.Dataset != "" && (arg == scope.Dataset || strings.HasPrefix(arg, scope.Dataset+"/") || strings.HasPrefix(arg, scope.Dataset+"@")) {
					return true, nil
				}
			}
			return false, nil
		}
		candidates = args[len(args)-1:]
	}
	for _, root := range scope.Roots {
		if !filepath.IsAbs(root) {
			continue
		}
		for _, arg := range candidates {
			if !scopeArgument(arg, root) {
				continue
			}
			same, err := seesScopeRoot(procRoot, pid, root)
			if err != nil {
				return false, err
			}
			if same {
				return true, nil
			}
		}
	}
	return false, nil
}

func ScopedProcesses(ctx context.Context, scope ProcessScope) ([]ProcessEvidence, error) {
	return scopedProcesses(ctx, "/proc", scope)
}
func scopedProcesses(ctx context.Context, root string, scope ProcessScope) ([]ProcessEvidence, error) {
	directory, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	result := []ProcessEvidence{}
	scanned := 0
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		entries, readErr := directory.ReadDir(256)
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil || pid <= 0 {
				continue
			}
			scanned++
			if scanned > maxProcessEntries {
				return nil, fmt.Errorf("process inventory exceeds its budget")
			}
			evidence, args, err := readProcessEvidence(root, pid)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			owned, scopeErr := ownedProcess(root, pid, args, scope)
			if scopeErr != nil {
				return nil, scopeErr
			}
			if owned {
				evidence.Class = processClass(args)
				if evidence.Class == "" {
					evidence.Class = "payload"
				}
				result = append(result, evidence)
				if len(result) > maxOwnedProcesses {
					return nil, fmt.Errorf("owned process inventory exceeds its budget")
				}
			}
		}
		if readErr == io.EOF {
			return result, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}

// StopScopedProcess pins the observed task with pidfd before revalidating start
// time and argv digest. Numeric PID reuse cannot redirect this signal.
func StopScopedProcess(ctx context.Context, evidence ProcessEvidence, scope ProcessScope) error {
	descriptor, err := unix.PidfdOpen(evidence.PID, 0)
	if errors.Is(err, unix.ESRCH) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unix.Close(descriptor)
	current, args, err := readProcessEvidence("/proc", evidence.PID)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	owned, scopeErr := ownedProcess("/proc", evidence.PID, args, scope)
	if scopeErr != nil {
		return scopeErr
	}
	if current.Start != evidence.Start || current.Digest != evidence.Digest || !owned {
		return fmt.Errorf("process identity changed; fencing retained")
	}
	if err = unix.PidfdSendSignal(descriptor, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		descriptors := []unix.PollFd{{Fd: int32(descriptor), Events: unix.POLLIN}}
		ready, err := unix.Poll(descriptors, 100)
		if err != nil && errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if ready > 0 {
			return nil
		}
	}
	return fmt.Errorf("process exit was not confirmed")
}
