//go:build linux || freebsd

package runcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func helper(mode, dir string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestCommandHelper$")
	cmd.Env = append(os.Environ(), "TT_COMMAND_HELPER="+mode, "TT_COMMAND_DIR="+dir, "GORACE=atexit_sleep_ms=0")
	return cmd
}

// Helper processes only touch a test-owned directory, and self-expire even if
// the parent test fails. They never signal unrelated processes or create guests.
func TestCommandHelper(t *testing.T) {
	mode := os.Getenv("TT_COMMAND_HELPER")
	if mode == "" {
		return
	}
	dir := os.Getenv("TT_COMMAND_DIR")
	switch mode {
	case "output":
		fmt.Print(strings.Repeat("x", CaptureLimit+123))
		fmt.Fprint(os.Stderr, "failure")
		os.Exit(7)
	case "sleep":
		time.Sleep(10 * time.Second)
	case "descendant":
		if err := os.WriteFile(filepath.Join(dir, "ready"), []byte("ready"), 0600); err != nil {
			os.Exit(3)
		}
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		_ = os.WriteFile(filepath.Join(dir, "done"), []byte("done"), 0600)
	case "launch", "launch-lock":
		child := helper("descendant", dir)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if mode == "launch-lock" {
			child.ExtraFiles = []*os.File{os.NewFile(3, "inherited-lock")}
		}
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		if mode == "launch-lock" {
			time.Sleep(10 * time.Second)
		}
	}
	os.Exit(0)
}
func waitFile(path string, limit time.Duration) bool {
	until := time.Now().Add(limit)
	for time.Now().Before(until) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
func releaseDescendant(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
		t.Error(err)
	}
	if !waitFile(filepath.Join(dir, "done"), 5*time.Second) {
		t.Error("test descendant did not exit")
	}
}
func TestCommandTimeoutAndOutput(t *testing.T) {
	out, err := Output(context.Background(), helper("output", ""), time.Second)
	if err != nil || out.ExitCode != 7 || len(out.Stdout) != CaptureLimit || string(out.Stderr) != "failure" {
		t.Fatalf("status=%d stdout=%d stderr=%q err=%v", out.ExitCode, len(out.Stdout), out.Stderr, err)
	}
	_, err = Output(context.Background(), helper("sleep", ""), 50*time.Millisecond)
	if !errors.Is(err, os.ErrDeadlineExceeded) || !strings.HasSuffix(err.Error(), "timed out after 0s") {
		t.Fatal(err)
	}
}
func TestInheritedOutputDoesNotWaitForDescendant(t *testing.T) {
	dir := t.TempDir()
	defer releaseDescendant(t, dir)
	out, err := Output(context.Background(), helper("launch", dir), 500*time.Millisecond)
	if err != nil || !out.Success() {
		t.Fatalf("%+v %v", out, err)
	}
	if !waitFile(filepath.Join(dir, "ready"), time.Second) {
		t.Fatal("descendant not ready")
	}
}
func TestStateLockExcludesSecondWriterAndReleasesOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.lock")
	first, err := LockState(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LockState(path)
	if err == nil {
		second.Close()
		t.Fatal("second writer acquired lock")
	}
	first.Close()
	second, err = LockState(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
}
func TestBackupLockSurvivesParentTimeout(t *testing.T) {
	dir := t.TempDir()
	defer releaseDescendant(t, dir)
	path := filepath.Join(dir, "backup.lock")
	lock, err := LockState(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd := helper("launch-lock", dir)
	cmd.ExtraFiles = []*os.File{lock}
	_, err = Output(context.Background(), cmd, 500*time.Millisecond)
	lock.Close()
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal(err)
	}
	if !waitFile(filepath.Join(dir, "ready"), time.Second) {
		t.Fatal("descendant not ready")
	}
	other, err := LockState(path)
	if err == nil {
		other.Close()
		t.Fatal("released live storage child's writer lock")
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !waitFile(filepath.Join(dir, "done"), time.Second) {
		t.Fatal("descendant did not finish")
	}
	// The done marker precedes process exit; acquire with a bounded retry.
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		other, err = LockState(path)
		if err == nil {
			other.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("lock remained held after child exit", err)
}

func TestUnboundedOutputStillHonorsCancellation(t *testing.T) {
	result, err := OutputUnbounded(context.Background(), exec.Command("sh", "-c", "sleep 0.02; printf ready"))
	if err != nil || !result.Success() || string(result.Stdout) != "ready" {
		t.Fatal(result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	command := exec.Command("sh", "-c", "printf ready > \"$1\"; exec sleep 30", "fixture", ready)
	done := make(chan error, 1)
	go func() { _, err := OutputUnbounded(ctx, command); done <- err }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("uncapped command ignored cancellation")
	}
}
