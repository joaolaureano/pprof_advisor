package cmd

import (
	"time"

	"github.com/joaolaureano/profadvisor/internal/pipeline"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var opts pipeline.Options
	var mo modelOptions
	c := &cobra.Command{
		Use:   "run --pkg <pattern> --dir <repo>",
		Short: "Capture, diagnose, apply, and re-measure",
		Long: "Runs the analysis pipeline to suggest and measure an optimization.\n\n" +
			"The objective is chosen with --profile and --unit and is " +
			"pushed into every stage, so the profile that is read, the patch that is " +
			"asked for, and the measurements are all for the same metric.\n\n" +
			"The suggestion branch is kept for inspection and the repository is left " +
			"on the branch you started from. Progress goes to stderr; the full record — " +
			"every intermediate artifact — goes to stdout as JSON.\n\n" +
			"To decide whether the change is an improvement, pass the baseline and after " +
			"benchmark paths from the result to the verify subcommand.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cat, err := mo.client()
			if err != nil {
				return err
			}
			opts.Analyze.Provider, opts.Analyze.Model, opts.Analyze.Prompts = mo.Provider, mo.Model, cat
			opts.Progress = cmd.ErrOrStderr()
			res, runErr := pipeline.Run(cmd.Context(), client, opts)
			// Emitted even on failure: the partial record names which step
			// broke and holds everything captured up to that point.
			if err := emit(cmd, res); err != nil {
				return err
			}
			return runErr
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Capture.Pkg, "pkg", "", "package pattern passed to `go test` (required)")
	f.StringVar(&opts.Capture.Bench, "bench", ".", "-bench regexp")
	f.StringVar(&opts.Capture.Dir, "dir", ".", "target repository root")
	f.IntVar(&opts.Capture.Count, "count", 10, "-count per capture")
	f.StringVar(&opts.Capture.Benchtime, "benchtime", "", "-benchtime, e.g. 1s")
	f.StringVar(&opts.Capture.OutDir, "out", "profadvisor-out", "root for captured artifacts")
	f.DurationVar(&opts.Capture.Timeout, "timeout", 20*time.Minute, "per-capture timeout")
	f.IntVar(&opts.Extract.TopN, "top", 10, "hotspots to send to the model")
	f.StringVar(&opts.Analyze.Module, "module", "", "target module path, for diff paths")
	measurementFlags(c, &opts.Profile, &opts.Unit)
	modelFlags(c, &mo)
	_ = c.MarkFlagRequired("pkg")
	return c
}
