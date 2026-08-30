// Package capture runs a Go benchmark and preserves the two artifacts the rest
// of the pipeline needs.
//
// It produces both from a single invocation on purpose. The CPU profile answers
// "where does the time go", which drives `analyze`; the benchmark output answers
// "how long does it take", which drives `verify`. They must describe the same
// run, or the diagnosis and the verdict are about different programs.
package capture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Options configures one capture.
type Options struct {
	// Pkg is the package pattern passed to `go test`, e.g. "./internal/parser/"
	// or a full import path. Required.
	Pkg string
	// Bench is the -bench regexp. Empty means ".", every benchmark.
	Bench string
	// Dir is the working directory the go command runs in — the target
	// repository root. Empty means the current directory.
	Dir string
	// Count is -count. Zero means 10: `verify` needs several samples per
	// benchmark to say anything statistical, and a single sample has more than
	// once suggested a regression that turned out to be a gain.
	Count int
	// Benchtime is -benchtime, e.g. "1s" or "2000x". Empty leaves it to the
	// go tool's default.
	Benchtime string
	// OutDir is the root under which a timestamped directory is created.
	// Empty means "./profadvisor-out".
	OutDir string
	// Timeout bounds the whole run. Zero means 20 minutes; -count=10 over a
	// package of benchmarks is not fast.
	Timeout time.Duration
}

// Result names what was written.
type Result struct {
	// Dir is the timestamped directory holding both artifacts.
	Dir string
	// ProfilePath is the CPU profile, input to `extract`.
	ProfilePath string
	// BenchPath is the raw `go test -bench` output, input to `verify`.
	BenchPath string
	// Command is the go invocation, recorded so a run can be reproduced by hand.
	Command  []string
	Duration time.Duration
}

// Run executes the benchmark and writes the artifacts.
//
// A non-zero exit from `go test` is an error, but the artifacts written so far
// are still reported in Result: a compile failure in the target is something the
// caller needs to see, and the captured output is where the reason is.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Pkg == "" {
		return nil, fmt.Errorf("capture: Pkg is required")
	}
	bench := opts.Bench
	if bench == "" {
		bench = "."
	}
	count := opts.Count
	if count == 0 {
		count = 10
	}
	outRoot := opts.OutDir
	if outRoot == "" {
		outRoot = "profadvisor-out"
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}

	dir := filepath.Join(outRoot, time.Now().UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("capture: creating %s: %w", dir, err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("capture: resolving %s: %w", dir, err)
	}

	res := &Result{
		Dir:         dir,
		ProfilePath: filepath.Join(dir, "cpu.prof"),
		BenchPath:   filepath.Join(dir, "bench.txt"),
	}

	// -run=^$ matters: without it `go test` runs the unit tests too, and their
	// time lands in the same CPU profile as the benchmarks.
	args := []string{
		"test", opts.Pkg,
		"-run", "^$",
		"-bench", bench,
		"-count", strconv.Itoa(count),
		"-cpuprofile", filepath.Join(absDir, "cpu.prof"),
		// -o keeps the compiled test binary out of the target repository.
		// Without it `go test -cpuprofile` drops pkg.test in the working
		// directory, which leaves the tree dirty — and `apply` then refuses to
		// run, so the pipeline stops itself on its own litter. It also pins the
		// run to a single package, which profiling requires anyway.
		"-o", filepath.Join(absDir, "pkg.test"),
	}
	if opts.Benchtime != "" {
		args = append(args, "-benchtime", opts.Benchtime)
	}
	res.Command = append([]string{"go"}, args...)

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(runCtx, "go", args...)
	cmd.Dir = opts.Dir
	cmd.Stdout = &out
	cmd.Stderr = &out

	start := time.Now()
	runErr := cmd.Run()
	res.Duration = time.Since(start)

	// Written before the error is returned: when the build fails, this file is
	// the only record of why.
	if err := os.WriteFile(res.BenchPath, out.Bytes(), 0o644); err != nil {
		return res, fmt.Errorf("capture: writing %s: %w", res.BenchPath, err)
	}
	if runErr != nil {
		return res, fmt.Errorf("capture: %s failed: %w\n%s",
			"go test", runErr, out.String())
	}
	if _, err := os.Stat(res.ProfilePath); err != nil {
		return res, fmt.Errorf("capture: no profile at %s — did any benchmark match %q?",
			res.ProfilePath, bench)
	}
	return res, nil
}
