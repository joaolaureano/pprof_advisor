package cmd

import (
	"time"

	"github.com/joaolaureano/profadvisor/internal/analyze"
	"github.com/joaolaureano/profadvisor/internal/pipeline"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var opts pipeline.Options
	c := &cobra.Command{
		Use:   "run --pkg <pattern> --dir <repo>",
		Short: "Capture, diagnose, apply, re-measure, and judge",
		Long: "Runs the whole loop and reports whether the change survived " +
			"measurement.\n\nThe objective is chosen with --profile and --unit and is " +
			"pushed into every stage, so the profile that is read, the patch that is " +
			"asked for, and the metric that decides are all the same one.\n\n" +
			"The suggestion branch is kept whatever the verdict, and " +
			"the repository is left on the branch you started from. Progress goes to " +
			"stderr; the full record — every intermediate artifact — goes to stdout " +
			"as JSON, so a disappointing verdict can be investigated without " +
			"repeating the work.\n\n" +
			"Exit code is 0 when the change was accepted or neutral, 2 when it made " +
			"things slower.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Progress = cmd.ErrOrStderr()
			res, runErr := pipeline.Run(cmd.Context(), analyze.NewClient(), opts)
			// Emitted even on failure: the partial record names which step
			// broke and holds everything captured up to that point.
			if err := emit(cmd, res); err != nil {
				return err
			}
			if runErr != nil {
				return runErr
			}
			if res.Verification != nil && res.Verification.Verdict == schema.VerdictRegressed {
				return errRegressed
			}
			return nil
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
	f.StringVar(&opts.Analyze.Model, "model", analyze.DefaultModel, "model id")
	f.StringVar(&opts.Analyze.Module, "module", "", "target module path, for diff paths")
	f.Float64Var(&opts.Verify.Alpha, "alpha", 0.05, "significance level")
	measurementFlags(c, &opts.Profile, &opts.Unit)
	_ = c.MarkFlagRequired("pkg")
	return c
}
