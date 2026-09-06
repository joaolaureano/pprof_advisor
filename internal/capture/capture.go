// Package capture runs a Go benchmark and preserves the two artifacts the rest
// of the pipeline needs.
//
// It produces both from a single invocation on purpose. The profile answers
// "where does the cost go", which drives `analyze`; the benchmark output answers
// "how much does it cost", which drives `verify`. They must describe the same
// run, or the diagnosis and the verdict are about different programs.
//
// Which profile is written follows from the measurement: -cpuprofile for a CPU
// run, -memprofile for a memory one. -benchmem is always on, whatever the
// objective, because verify needs ns/op, B/op and allocs/op from the same
// output in order to guard one against the other.
package capture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joaolaureano/profadvisor/internal/benchmark"
	"github.com/joaolaureano/profadvisor/internal/measurement"
)

// Options configures one capture.
type Options struct {
	// Profile and Unit name the objective. They are resolved through
	// measurement.Resolve, so the zero value means CPU nanoseconds.
	Profile measurement.Kind
	Unit    string
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
	Dir string `json:"dir"`
	// Measurement is what this capture was configured to measure. It travels
	// with the artifacts so a later stage cannot misread them.
	Measurement measurement.Config `json:"measurement"`
	// ProfilePath is the pprof profile, input to `extract`.
	ProfilePath string `json:"profile_path"`
	// BenchPath is the raw `go test -bench` output, input to `verify`.
	BenchPath string `json:"bench_path"`
	// Command is the go invocation, recorded so a run can be reproduced by hand.
	Command  []string      `json:"command"`
	Duration time.Duration `json:"duration_ns"`
}

// profileFile is the name written under the capture directory, per profile kind.
func profileFile(kind measurement.Kind) string {
	if kind == measurement.Memory {
		return "mem.prof"
	}
	return "cpu.prof"
}

// Run executes the benchmark and writes the artifacts.
//
// A non-zero exit from `go test` is an error, but the artifacts written so far
// are still reported in Result: a compile failure in the target is something the
// caller needs to see, and the captured output is where the reason is.
func Run(ctx context.Context, opts Options) (*Result, error) {
	cfg, err := measurement.Resolve(opts.Profile, opts.Unit)
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	if opts.Pkg == "" {
		return nil, fmt.Errorf("capture: Pkg is required")
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

	dir := filepath.Join(outRoot, time.Now().UTC().Format("20060102T150405.000Z"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("capture: creating %s: %w", dir, err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("capture: resolving %s: %w", dir, err)
	}

	name := profileFile(cfg.Profile)
	res := &Result{
		Dir:         dir,
		Measurement: cfg,
		ProfilePath: filepath.Join(dir, name),
		BenchPath:   filepath.Join(dir, "bench.txt"),
	}

	flag := "-cpuprofile"
	if cfg.Profile == measurement.Memory {
		flag = "-memprofile"
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// -o keeps the compiled test binary out of the target repository. Without
	// it `go test -cpuprofile` drops pkg.test in the working directory, which
	// leaves the tree dirty — and `apply` then refuses to run, so the pipeline
	// stops itself on its own litter. It also pins the run to a single
	// package, which profiling requires anyway.
	out, runErr := benchmark.Run(runCtx, benchmark.Options{
		Dir:          opts.Dir,
		Pkg:          opts.Pkg,
		Bench:        opts.Bench,
		Count:        count,
		Benchtime:    opts.Benchtime,
		Benchmem:     true,
		ProfileFlags: []string{flag, filepath.Join(absDir, name)},
		BinaryPath:   filepath.Join(absDir, "pkg.test"),
	})
	if out != nil {
		res.Command, res.Duration = out.Command, out.Duration
		// Written before the error is returned: when the build fails, this
		// file is the only record of why.
		if err := os.WriteFile(res.BenchPath, out.Output, 0o644); err != nil {
			return res, fmt.Errorf("capture: writing %s: %w", res.BenchPath, err)
		}
	}
	if runErr != nil {
		return res, fmt.Errorf("capture: %w", runErr)
	}
	if _, err := os.Stat(res.ProfilePath); err != nil {
		bench := opts.Bench
		if bench == "" {
			bench = "."
		}
		return res, fmt.Errorf("capture: no profile at %s — did any benchmark match %q?",
			res.ProfilePath, bench)
	}
	return res, nil
}
