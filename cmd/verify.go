package cmd

import (
	"fmt"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/joaolaureano/profadvisor/internal/verify"
	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	var (
		opts            verify.Options
		profile         measurement.Kind
		unit            string
		baseline, after string
	)
	c := &cobra.Command{
		Use:   "verify --baseline <bench.txt> --after <bench.txt>",
		Short: "Decide whether a change actually improved the objective",
		Long: "Compares two `go test -bench -benchmem` outputs and returns MELHOROU, " +
			"SEM DIFERENÇA, or PIOROU per benchmark and metric, with the percentage " +
			"delta and a p-value.\n\nThe verdict is decided by the objective chosen " +
			"with --unit, and by its guards: a memory objective is guarded by ns/op, " +
			"so a patch that saves bytes and costs time is PIOROU. Every other metric " +
			"is reported and votes on nothing.\n\nThe inputs are benchmark output, " +
			"not profiles: a p-value needs N samples of the metric, and a profile says " +
			"where the cost went, not how much there was. Capture both sides with " +
			"--count 10 or higher.\n\nExit code is 0 for MELHOROU and SEM DIFERENÇA, " +
			"2 for PIOROU, so a CI job can gate on it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := measurement.Resolve(profile, unit)
			if err != nil {
				return err
			}
			opts.Measurement = cfg
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
	measurementFlags(c, &profile, &unit)
	_ = c.MarkFlagRequired("baseline")
	_ = c.MarkFlagRequired("after")
	return c
}

// errRegressed is returned so `verify` exits non-zero on a regression while
// still having written its JSON. It is recognised by Execute, which maps it to
// exit code 2 rather than treating it as a tool failure — the tool worked, the
// change did not.
var errRegressed = fmt.Errorf("benchmark regressed")
