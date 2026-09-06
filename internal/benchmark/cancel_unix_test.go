//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package benchmark

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestRunCancellationKillsRunningTestProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a blocking benchmark")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "pid")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/block\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`package block

import ("os"; "strconv"; "testing")

func BenchmarkBlock(b *testing.B) {
	os.WriteFile(%q, []byte(strconv.Itoa(os.Getpid())), 0644)
	select {}
}
`, marker)
	if err := os.WriteFile(filepath.Join(dir, "block_test.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var result *Result
	var runErr error
	go func() {
		result, runErr = Run(ctx, Options{Dir: dir, Pkg: ".", Bench: "^BenchmarkBlock$", Count: 1, Benchtime: "1x"})
		close(done)
	}()
	deadline := time.Now().Add(30 * time.Second)
	var pid int
	for {
		data, err := os.ReadFile(marker)
		if err == nil {
			pid, err = strconv.Atoi(string(data))
			if err == nil && pid > 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("blocking benchmark did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not return promptly")
	}
	if runErr == nil || result == nil {
		t.Fatalf("Run result=%v err=%v, want cancellation error", result, runErr)
	}
	if err := syscall.Kill(pid, 0); err == nil || !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("benchmark process %d still exists after cancellation: %v", pid, err)
	}
}
