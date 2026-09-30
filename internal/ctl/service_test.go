package ctl

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TTstack/tt/internal/api"
	"github.com/TTstack/tt/internal/capability"
	"github.com/TTstack/tt/internal/model"
	"github.com/TTstack/tt/internal/runcmd"
)

func TestServiceShutdownDrainsAcceptedCreationAndRetainsStateLock(t *testing.T) {
	if os.Getenv("TT_TEST_INTEGRATION") != "1" {
		t.Skip("service integration is opt-in")
	}
	dir := t.TempDir()
	ctx := context.Background()
	db, err := OpenDB(ctx, filepath.Join(dir, "ctl.db"))
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	mock := &mockAgent{vms: []model.Vm{}, caps: append([]string{}, capability.LegacyAll...), resource: model.Resource{CPUTotal: 4, MemTotal: 4096, DiskTotal: 1024}}
	mock.beforeCreate = func(request api.CreateVmReq) { enterOnce.Do(func() { close(entered) }); <-release }
	peer := httptest.NewServer(mock.handler())
	defer peer.Close()
	host := hostRecord("host")
	host.Addr = strings.TrimPrefix(peer.URL, "http://")
	if err := db.PutHost(ctx, &host); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reserved.Addr().String()
	reserved.Close()
	running, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- serveController(running, Config{Listen: address, DataDir: dir}, io.Discard) }()
	defer releaseOnce.Do(func() { close(release) })
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get("http://" + address + "/api/status")
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("controller did not start", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	body, err := json.Marshal(environmentRequest("drained"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Post("http://"+address+"/api/envs", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 202 {
		t.Fatal(response.StatusCode)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("agent create not entered")
	}
	cancel()
	select {
	case code := <-done:
		t.Fatal("shutdown abandoned accepted creation", code)
	case <-time.After(100 * time.Millisecond):
	}
	if lock, err := runcmd.LockState(filepath.Join(dir, "service.lock")); err == nil {
		lock.Close()
		t.Fatal("state lock released while creation is active")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case code := <-done:
		if code != 0 {
			t.Fatal(code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not drain")
	}
	db, err = OpenDB(ctx, filepath.Join(dir, "ctl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	env, err := db.GetEnv(ctx, "drained")
	if err != nil || env == nil || env.State != model.EnvActive {
		t.Fatal(env, err)
	}
	lock, err := runcmd.LockState(filepath.Join(dir, "service.lock"))
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
}
