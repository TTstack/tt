package ctl

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TTstack/tt/internal/auth"
	"github.com/TTstack/tt/internal/runcmd"
)

func serveController(ctx context.Context, cfg Config, stderr io.Writer) int {
	var key *string
	if cfg.keyPresent {
		key = &cfg.APIKey
		if _, err := auth.ParseAPIKey(*key); err != nil {
			fmt.Fprintf(stderr, "Invalid API key: %v\n", err)
			return 1
		}
	}
	if err := os.MkdirAll(cfg.DataDir, 0777); err != nil {
		fmt.Fprintf(stderr, "Failed to create data dir %s: %v\n", cfg.DataDir, err)
		return 1
	}
	lock, err := runcmd.LockState(filepath.Join(cfg.DataDir, "service.lock"))
	if err != nil {
		fmt.Fprintf(stderr, "Cannot lock state: %v\n", err)
		return 1
	}
	defer lock.Close()
	db, err := OpenDB(ctx, filepath.Join(cfg.DataDir, "ctl.db"))
	if err != nil {
		fmt.Fprintf(stderr, "Failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()
	db.stderr = stderr
	state := newServer(db, key)
	state.stderr = stderr
	reader, err := state.clientFactory(5 * time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "cannot initialize agent client: %v\n", err)
		return 1
	}
	if key != nil {
		fmt.Fprintln(stderr, "API key authentication enabled")
	} else {
		fmt.Fprintln(stderr, "WARNING: no --api-key set, all API endpoints are unauthenticated!")
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to bind %s: %v\n", cfg.Listen, err)
		return 1
	}
	fmt.Fprintf(stderr, "tt-ctl listening on %s\n", cfg.Listen)
	background, stop := context.WithCancel(context.Background())
	var loops sync.WaitGroup
	loops.Add(2)
	go func() {
		defer loops.Done()
		for {
			timer := time.NewTimer(60 * time.Second)
			select {
			case <-background.Done():
				timer.Stop()
				return
			case <-timer.C:
				state.ExpireOnce(background)
			}
		}
	}()
	go func() {
		defer loops.Done()
		for {
			if background.Err() != nil {
				return
			}
			state.RefreshAllHosts(background, reader)
			timer := time.NewTimer(15 * time.Second)
			select {
			case <-background.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	server := &http.Server{Handler: state.Handler()}
	serving := make(chan error, 1)
	go func() { serving <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		fmt.Fprintln(stderr, "received shutdown signal")
		stop()
		_ = server.Shutdown(context.Background())
		<-serving
	case err := <-serving:
		stop()
		_ = server.Shutdown(context.Background())
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "Server error: %v\n", err)
		}
	}
	loops.Wait()
	state.workers.Wait()
	fmt.Fprintln(stderr, "tt-ctl shutting down")
	return 0
}
