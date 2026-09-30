// Package runcmd provides bounded host command execution and process locks.
package runcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const DefaultTimeout = 300 * time.Second
const CaptureLimit = 4 * 1024 * 1024

type Result struct {
	Stdout, Stderr []byte
	ExitCode       int
}

func (r Result) Success() bool { return r.ExitCode == 0 }

type timeoutError struct {
	program  string
	duration time.Duration
}

func (e timeoutError) Error() string {
	return fmt.Sprintf("%s timed out after %ds", e.program, int64(e.duration/time.Second))
}

func (e timeoutError) Unwrap() error { return os.ErrDeadlineExceeded }

// Output preserves nonzero process status as data. Only spawn, capture, timeout
// or cancellation errors are returned as errors. Temporary files avoid readers
// hanging when a descendant inherits stdout/stderr. ExtraFiles, when supplied by
// the caller, preserve a backup writer lock in surviving storage children.
func Output(ctx context.Context, cmd *exec.Cmd, timeout time.Duration) (Result, error) {
	var result Result
	if err := ctx.Err(); err != nil {
		return result, err
	}
	stdout, err := os.CreateTemp("", "tt-stdout-*")
	if err != nil {
		return result, err
	}
	defer os.Remove(stdout.Name())
	defer stdout.Close()
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
		if err := os.Remove(stdout.Name()); err != nil {
			return result, err
		}
	}
	stderr, err := os.CreateTemp("", "tt-stderr-*")
	if err != nil {
		return result, err
	}
	defer os.Remove(stderr.Name())
	defer stderr.Close()
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
		if err := os.Remove(stderr.Name()); err != nil {
			return result, err
		}
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return result, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			return result, err
		}
	case <-timer.C:
		_ = cmd.Process.Kill()
		<-done
		return result, timeoutError{cmd.Args[0], timeout}
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return result, ctx.Err()
	}
	result.ExitCode = cmd.ProcessState.ExitCode()
	if _, err := stdout.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	if _, err := stderr.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	result.Stdout, err = io.ReadAll(io.LimitReader(stdout, CaptureLimit))
	if err != nil {
		return result, err
	}
	result.Stderr, err = io.ReadAll(io.LimitReader(stderr, CaptureLimit))
	return result, err
}
func Run(ctx context.Context, name string, args ...string) (Result, error) {
	return Output(ctx, exec.Command(name, args...), DefaultTimeout)
}
