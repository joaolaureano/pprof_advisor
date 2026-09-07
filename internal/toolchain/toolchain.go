package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joaolaureano/profadvisor/internal/proc"
)

// Info identifies the toolchain that produced a set of diagnostics.
//
// It is recorded rather than assumed because a compiler diagnostic only means
// something beside the compiler that reached it: escape analysis improves
// between releases, and the same source can legitimately get a different answer
// on a different version or architecture.
type Info struct {
	// Path is the go binary actually used, made absolute. Which `go` is on PATH
	// is not always the one the reader of a report would assume.
	Path string
	// Version is GOVERSION, e.g. "go1.26.1".
	Version string
	// Raw is the full `go version` line.
	Raw  string
	GOOS string
	// GOARCH matters here: escape analysis is architecture-independent in
	// principle, but the diagnostics quote type widths, which are not.
	GOARCH string
}

// Detect finds the Go toolchain that will compile a target in dir.
//
// Both queries run with cwd=dir on purpose. A target's go.mod can select a
// different toolchain with a `toolchain` directive, and the version that matters
// is the one that will actually compile the target — not the launcher that
// happens to be first on PATH.
func Detect(ctx context.Context, dir string) (Info, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: no `go` on PATH: %w", err)
	}
	// LookPath may return a relative path when PATH holds a relative entry, and
	// a relative path in a report is worse than none.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	env, err := run(ctx, path, dir, "env", "GOVERSION", "GOOS", "GOARCH")
	if err != nil {
		return Info{}, err
	}
	// One line per variable, in the order asked. None of the three can be empty
	// from a working toolchain, so a short answer means something is wrong and
	// guessing at which field is missing would be worse than stopping.
	fields := strings.Split(strings.ReplaceAll(env, "\r\n", "\n"), "\n")
	if len(fields) < 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
		return Info{}, fmt.Errorf("toolchain: `go env` in %s returned %q, want GOVERSION, GOOS and GOARCH", dir, env)
	}

	raw, err := run(ctx, path, dir, "version")
	if err != nil {
		return Info{}, err
	}

	return Info{
		Path:    path,
		Version: fields[0],
		Raw:     raw,
		GOOS:    fields[1],
		GOARCH:  fields[2],
	}, nil
}

// run executes one short, non-interactive go command and returns its trimmed
// stdout. Its stderr is folded into the error because `go env` failing with the
// reason discarded is a support ticket.
func run(ctx context.Context, goBin, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("toolchain: go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// BuildOptions describes one compilation of a target purely for its diagnostics.
type BuildOptions struct {
	// Dir is the target project root. Required; package patterns are resolved
	// inside it, not inside this tool's checkout.
	Dir string
	// Patterns are go package patterns. Empty means ["./..."].
	Patterns []string
	// Gcflags is passed through to -gcflags, e.g. "-m=2". Required: this package
	// exists to collect compiler diagnostics, and a build with no diagnostic
	// flag collects nothing.
	Gcflags string
	// Timeout bounds the whole build. Zero means five minutes.
	Timeout time.Duration
}

// BuildResult is the captured output of one build.
type BuildResult struct {
	// Command is the argv, recorded so the run can be reproduced by hand.
	Command  []string
	Stderr   []byte
	Duration time.Duration
}

// Build compiles the target for its diagnostics and returns them.
//
// -o os.DevNull is passed unconditionally. The go tool special-cases it and
// accepts it for any number of packages, main and library alike, so no binary is
// ever written into the target repository — this tool reads other people's
// projects and must not leave anything behind. stdout is discarded because the
// toolchain writes every diagnostic to stderr.
//
// A non-zero exit is an error wrapping the captured output. A target that does
// not compile has to fail loudly: the alternative is a report that says a
// package has no escapes when in truth it was never analyzed.
func Build(ctx context.Context, o BuildOptions) (*BuildResult, error) {
	if o.Dir == "" {
		return nil, fmt.Errorf("toolchain: dir is required")
	}
	if o.Gcflags == "" {
		return nil, fmt.Errorf("toolchain: gcflags is required")
	}
	patterns := o.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{"build", "-o", os.DevNull, "-gcflags=" + o.Gcflags}, patterns...)
	res := &BuildResult{Command: append([]string{"go"}, args...)}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", args...)
	proc.Configure(cmd)
	// The go tool spawns a compiler per package; WaitDelay bounds how long the
	// parent waits if cancellation races with those shutting down.
	cmd.WaitDelay = 2 * time.Second
	cmd.Dir = o.Dir
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.Stderr = append([]byte(nil), stderr.Bytes()...)
	if err != nil {
		return res, fmt.Errorf("toolchain: go build failed in %s: %w\n%s", o.Dir, err, stderr.String())
	}
	return res, nil
}
