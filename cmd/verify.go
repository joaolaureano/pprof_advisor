package cmd

import (
	"fmt"

	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/joaolaureano/profadvisor/internal/verify"
	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	var (
		opts            verify.Options
		baseline, after string
	)
	c := &cobra.Command{
		Use:   "verify --baseline <bench.txt> --after <bench.txt>",
		Short: "Decide whether a change actually made things faster",
		Long: "Compares two `go test -bench` outputs and returns MELHOROU, " +
			"SEM DIFERENÇA, or PIOROU per benchmark, with the percentage delta and " +
			"a p-value.\n\nThe inputs are benchmark output, not profiles: a p-value " +
			"needs N samples of ns/op, and a profile says where time went, not how " +
			"long the operation took. Capture both sides with --count 10 or higher.\n\n" +
			"Exit code is 0 for MELHOROU and SEM DIFERENÇA, 2 for PIOROU, so a CI " +
			"job can gate on it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := verify.FromFiles(baseline, after, opts)
			if err != nil {
				return err
			}
			if err := emit(cmd, res); err != nil {
				return err
			}
			if res.Verdict == schema.VerdictRegressed {
				return errRegressed
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&baseline, "baseline", "", "benchmark output from before the change (required)")
	f.StringVar(&after, "after", "", "benchmark output from after the change (required)")
	f.Float64Var(&opts.Alpha, "alpha", 0.05, "significance level")
	f.StringVar(&opts.Unit, "unit", "ns/op", "metric to compare")
	_ = c.MarkFlagRequired("baseline")
	_ = c.MarkFlagRequired("after")
	return c
}

// errRegressed is returned so `verify` exits non-zero on a regression while
// still having written its JSON. It is recognised by Execute, which maps it to
// exit code 2 rather than treating it as a tool failure — the tool worked, the
// change did not.
var errRegressed = fmt.Errorf("benchmark regressed")
