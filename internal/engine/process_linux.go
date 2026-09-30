package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type processes struct{ root string }

func hostProcesses() processes { return processes{root: "/proc"} }
func (p processes) matches(pid uint32, marker string) (bool, error) {
	dir := filepath.Join(p.root, strconv.FormatUint(uint64(pid), 10))
	data, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read process identity: %w", err)
	}
	if len(data) == 0 {
		stat, err := os.ReadFile(filepath.Join(dir, "stat"))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err == nil && utf8.Valid(stat) {
			index := strings.LastIndex(string(stat), ") ")
			if index >= 0 && strings.HasPrefix(string(stat)[index+2:], "Z ") {
				return false, nil
			}
		}
		return false, fmt.Errorf("process %d has no readable identity; resources retained", pid)
	}
	for _, arg := range bytes.Split(data, []byte{0}) {
		if string(arg) == marker {
			return true, nil
		}
	}
	return false, nil
}
func (p processes) recover(path string, markers []string) (uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read VM PID: %w", err)
	}
	if err == nil {
		if !utf8.Valid(data) {
			return 0, fmt.Errorf("read VM PID: invalid UTF-8; resources retained")
		}
		id, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(string(data)), "+"), 10, 32)
		if err == nil && id > 0 && id <= math.MaxInt32 {
			for _, marker := range markers {
				match, err := p.matches(uint32(id), marker)
				if err != nil {
					return 0, err
				}
				if match {
					return uint32(id), nil
				}
			}
		}
	}
	entries, err := os.ReadDir(p.root)
	if err != nil {
		return 0, fmt.Errorf("recover VM PID: %w", err)
	}
	var found uint32
	for _, entry := range entries {
		pid, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.root, entry.Name(), "cmdline"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("recover process identity: %w", err)
		}
		match := false
		for _, arg := range bytes.Split(data, []byte{0}) {
			for _, marker := range markers {
				match = match || string(arg) == marker
			}
		}
		if match {
			if found != 0 {
				return 0, fmt.Errorf("multiple processes match VM identity; resources retained")
			}
			found = uint32(pid)
		}
	}
	return found, nil
}
func (p processes) terminate(ctx context.Context, pid uint32, marker string) error {
	for _, signal := range []unix.Signal{unix.SIGTERM, unix.SIGKILL} {
		match, err := p.matches(pid, marker)
		if err != nil {
			return err
		}
		if !match {
			return nil
		}
		if err := unix.Kill(int(pid), signal); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("terminate VM: %w", err)
		}
		for i := 0; i < 100; i++ {
			match, err := p.matches(pid, marker)
			if err != nil {
				return err
			}
			if !match {
				return nil
			}
			if err := sleepContext(ctx, 50*time.Millisecond); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("VM process %d did not exit; resources retained", pid)
}
