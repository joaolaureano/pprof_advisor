// Package pipeline runs the five steps in order: capture, extract, analyze,
// apply, and re-capture. It produces the full record of artifacts and leaves
// the verdict for the caller to decide with the verify subcommand.
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
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Options configures one end-to-end run.
//
// Profile and Unit are set once, here, and pushed down into every stage. The
// stages each accept a measurement.Config of their own so they stay usable
// alone, which means they could in principle disagree; Run rejects that before
// it captures anything rather than producing a diagnosis about one metric and a
// verdict about another.
type Options struct {
	Profile measurement.Kind
	Unit    string
	Capture capture.Options
	Extract extract.Options
	Analyze analyze.Options
	Apply   apply.Options
	// Progress receives one human-readable line per step. It is not the
	// result — that is the returned Result, as JSON. Nil discards.
	Progress io.Writer
}

// Result is the full record of a run: every intermediate artifact produced by
// the five stages, so any aspect can be investigated without repeating the work.
// To decide whether a change is an improvement, pass the baseline and after
// benchmark paths to verify.FromFiles.
type Result struct {
	SchemaVersion int                   `json:"schema_version"`
	Measurement   measurement.Config    `json:"measurement"`
	Baseline      *capture.Result       `json:"baseline"`
	Hotspots      *schema.ExtractResult `json:"hotspots"`
	Diagnosis     *schema.Diagnosis     `json:"diagnosis"`
	Applied       *schema.ApplyResult   `json:"applied"`
	After         *capture.Result       `json:"after"`
	Duration      time.Duration         `json:"duration_ns"`
}

// Run executes capture, extract, analyze, apply, and re-capture.
//
// It always leaves the repository on the branch it started from. The suggestion
// branch is kept in case it is useful for inspection. The caller must pass the
// baseline and after benchmark paths from the result to verify.FromFiles to
// decide whether the change is an improvement.
func Run(ctx context.Context, c analyze.Client, opts Options) (*Result, error) {
	start := time.Now()
	res := &Result{SchemaVersion: schema.Version}
	step := func(format string, args ...any) {
		if opts.Progress != nil {
			fmt.Fprintf(opts.Progress, format+"\n", args...)
		}
	}

	cfg, err := resolve(&opts)
	if err != nil {
		return res, err
	}
	res.Measurement = cfg

	step("capturing baseline: %s %s, objective %s", opts.Capture.Pkg, opts.Capture.Bench, cfg.Unit)
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
		return res, fmt.Errorf("pipeline: no hotspots survived filtering; does %s spend %s in code you own?",
			opts.Capture.Pkg, cfg.SampleType)
	}
	step("top hotspot: %s (%.1f%% of attributed %s)",
		hotspots.Hotspots[0].Function, hotspots.Hotspots[0].FlatPct, cfg.SampleUnit)

	step("asking %s for a diagnosis", firstNonEmpty(opts.Analyze.Model, "the model"))
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

	res.Duration = time.Since(start)
	step("branch %s kept; compare with: profadvisor verify --baseline %s --after %s",
		applied.Branch, baseline.BenchPath, after.BenchPath)
	return res, nil
}

// resolve settles the objective and pushes it into every stage's options.
//
// A stage that was given its own measurement must agree with the run's, and the
// mismatch is an error rather than a silent overwrite: a caller that set
// Capture.Profile to something else meant it, and the honest answer is that the
// two cannot both be true.
func resolve(opts *Options) (measurement.Config, error) {
	cfg, err := measurement.Resolve(opts.Profile, opts.Unit)
	if err != nil {
		return measurement.Config{}, fmt.Errorf("pipeline: %w", err)
	}
	if opts.Capture.Profile != "" && opts.Capture.Profile != cfg.Profile {
		return measurement.Config{}, fmt.Errorf(
			"pipeline: run measures %s but capture is configured for %s",
			cfg.Profile, opts.Capture.Profile)
	}
	if opts.Capture.Unit != "" && opts.Capture.Unit != cfg.Unit {
		return measurement.Config{}, fmt.Errorf(
			"pipeline: run objective is %s but capture is configured for %s",
			cfg.Unit, opts.Capture.Unit)
	}
	if (opts.Extract.Measurement != measurement.Config{}) && opts.Extract.Measurement != cfg {
		return measurement.Config{}, fmt.Errorf(
			"pipeline: run measures %s but extract is configured for %s",
			cfg.Unit, opts.Extract.Measurement.Unit)
	}
	if opts.Capture.Pkg == "" {
		return measurement.Config{}, fmt.Errorf("pipeline: --pkg is required")
	}
	// The target repository is one thing to the caller but two fields here.
	// Defaulting it in the pipeline rather than in the CLI matters more than
	// convenience: a caller that set only Capture.Dir and left Apply.Dir empty
	// would have git apply a model's patch to the current working directory,
	// which is whatever process happened to invoke this.
	if opts.Apply.Dir == "" {
		opts.Apply.Dir = opts.Capture.Dir
	}
	opts.Capture.Profile, opts.Capture.Unit = cfg.Profile, cfg.Unit
	opts.Extract.Measurement = cfg
	return cfg, nil
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
