// Package benchmark runs a single go test benchmark invocation. It owns
// process execution and output collection; callers decide which flags and
// metrics are appropriate for their measurement.
package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/joaolaureano/profadvisor/internal/proc"
	"golang.org/x/perf/benchfmt"
)

// Options describes one invocation. ProfileFlags are passed verbatim to go
// test, allowing capture to select CPU or memory profiling without making this
// package aware of profiling policy.
type Options struct {
	Dir          string
	Pkg          string
	Bench        string
	Count        int
	Benchtime    string
	ProfileFlags []string
	BinaryPath   string
	Benchmem     bool
}

// Result contains the command and combined output from one invocation.
type Result struct {
	Command    []string
	Output     []byte
	Duration   time.Duration
	Benchmarks int
}

// Run executes go test once. The caller owns timeout policy by passing a
// context; cancellation errors are returned with the captured output.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Pkg == "" {
		return nil, fmt.Errorf("benchmark: package is required")
	}
	bench := opts.Bench
	if bench == "" {
		bench = "."
	}
	count := opts.Count
	if count <= 0 {
		count = 1
	}
	args := []string{"test", opts.Pkg, "-run", "^$", "-bench", bench, "-count", strconv.Itoa(count)}
	if opts.Benchmem {
		args = append(args, "-benchmem")
	}
	if opts.Benchtime != "" {
		args = append(args, "-benchtime", opts.Benchtime)
	}
	args = append(args, opts.ProfileFlags...)
	if opts.BinaryPath != "" {
		args = append(args, "-o", opts.BinaryPath)
	}
	res := &Result{Command: append([]string{"go"}, args...)}
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	proc.Configure(cmd)
	// go test launches a separate test binary. WaitDelay bounds how long the
	// parent waits if cancellation races with that child shutting down.
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = opts.Dir
	cmd.Stdout = &output
	cmd.Stderr = &output
	start := time.Now()
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.Output = append([]byte(nil), output.Bytes()...)
	if err != nil {
		return res, fmt.Errorf("benchmark: go test failed: %w\n%s", err, output.String())
	}
	reader := benchfmt.NewReader(bytes.NewReader(res.Output), "go test")
	for reader.Scan() {
		switch record := reader.Result().(type) {
		case *benchfmt.Result:
			res.Benchmarks++
		case *benchfmt.SyntaxError:
			return res, fmt.Errorf("benchmark: parsing benchmark output: %w\n%s", record, output.String())
		}
	}
	if err := reader.Err(); err != nil {
		return res, fmt.Errorf("benchmark: parsing benchmark output: %w\n%s", err, output.String())
	}
	return res, nil
}
