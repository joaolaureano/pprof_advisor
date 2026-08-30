// Package pipeline runs the five steps in order and decides what the result
// means.
//
// The ordering is the whole design: the change is measured against a baseline
// captured from the same tree, on the same machine, minutes apart. Comparing
// against a number from yesterday would be cheaper and would not mean anything.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/joaolaureano/profadvisor/internal/analyze"
	"github.com/joaolaureano/profadvisor/internal/apply"
	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/joaolaureano/profadvisor/internal/verify"
)

// Options configures one end-to-end run.
type Options struct {
	Capture capture.Options
	Extract extract.Options
	Analyze analyze.Options
	Apply   apply.Options
	Verify  verify.Options
	// Progress receives one human-readable line per step. It is not the
	// result — that is the returned Result, as JSON. Nil discards.
	Progress io.Writer
}

// Result is the full record of a run: every intermediate artifact, so a
// disappointing verdict can be investigated without repeating the work.
type Result struct {
	SchemaVersion int                   `json:"schema_version"`
	Baseline      *capture.Result       `json:"baseline"`
	Hotspots      *schema.ExtractResult `json:"hotspots"`
	Diagnosis     *schema.Diagnosis     `json:"diagnosis"`
	Applied       *schema.ApplyResult   `json:"applied"`
	After         *capture.Result       `json:"after"`
	Verification  *schema.VerifyResult  `json:"verification"`
	// Accepted is true only when verification says MELHOROU. It is the one
	// field that answers "did this work".
	Accepted bool          `json:"accepted"`
	Duration time.Duration `json:"duration_ns"`
}

// Run executes capture, extract, analyze, apply, capture, verify.
//
// It always leaves the repository on the branch it started from. The suggestion
// branch is kept whatever the verdict — a rejected suggestion is still the most
// informative thing produced by the run, and deleting it would throw that away.
func Run(ctx context.Context, c analyze.Client, opts Options) (*Result, error) {
	start := time.Now()
	res := &Result{SchemaVersion: schema.Version}
	step := func(format string, args ...any) {
		if opts.Progress != nil {
			fmt.Fprintf(opts.Progress, format+"\n", args...)
		}
	}

	step("capturing baseline (%s, %s)", opts.Capture.Pkg, opts.Capture.Bench)
	baseline, err := capture.Run(ctx, opts.Capture)
	res.Baseline = baseline
	if err != nil {
		return res, err
	}

	step("extracting hotspots from %s", baseline.ProfilePath)
	hotspots, err := extract.FromFile(baseline.ProfilePath, opts.Extract)
	res.Hotspots = hotspots
	if err != nil {
		return res, err
	}
	if len(hotspots.Hotspots) == 0 {
		return res, fmt.Errorf("pipeline: no hotspots survived filtering; is %s CPU-bound?", opts.Capture.Pkg)
	}
	step("top hotspot: %s (%.1f%% of attributed time)",
		hotspots.Hotspots[0].Function, hotspots.Hotspots[0].FlatPct)

	step("asking %s for a diagnosis", firstNonEmpty(opts.Analyze.Model, analyze.DefaultModel))
	diagnosis, err := analyze.Run(ctx, c, hotspots, opts.Analyze)
	res.Diagnosis = diagnosis
	if err != nil {
		return res, err
	}
	step("proposal: %s (confidence %s)", diagnosis.Change, diagnosis.Confidence)

	step("applying on a new branch")
	applied, err := apply.Run(ctx, diagnosis, opts.Apply)
	res.Applied = applied
	if err != nil {
		return res, err
	}
	// From here the repository is on the suggestion branch, so every exit
	// path has to put it back.
	defer func() {
		if applied.BaseRef != "" {
			_ = checkout(context.WithoutCancel(ctx), opts.Apply.Dir, applied.BaseRef)
		}
	}()
	step("applied on %s (%d file(s))", applied.Branch, len(applied.FilesChanged))

	step("re-capturing on %s", applied.Branch)
	after, err := capture.Run(ctx, opts.Capture)
	res.After = after
	if err != nil {
		return res, err
	}

	step("comparing")
	verification, err := verify.FromFiles(baseline.BenchPath, after.BenchPath, opts.Verify)
	res.Verification = verification
	if err != nil {
		return res, err
	}
	res.Accepted = verification.Verdict == schema.VerdictImproved
	res.Duration = time.Since(start)
	step("verdict: %s — branch %s kept for inspection", verification.Verdict, applied.Branch)
	return res, nil
}

func checkout(ctx context.Context, dir, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", ref)
	cmd.Dir = dir
	return cmd.Run()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
