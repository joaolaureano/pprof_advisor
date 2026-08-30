package cmd

import (
	"time"

	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/spf13/cobra"
)

func newCaptureCmd() *cobra.Command {
	var opts capture.Options
	c := &cobra.Command{
		Use:   "capture",
		Short: "Run a benchmark, saving a CPU profile and the benchmark output",
		Long: "Runs `go test -bench` once, keeping both artifacts the pipeline needs:\n" +
			"  cpu.prof   the CPU profile, input to `extract`\n" +
			"  bench.txt  the raw benchmark output, input to `verify`\n\n" +
			"Both come from the same run so the diagnosis and the verdict describe " +
			"the same program.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := capture.Run(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Pkg, "pkg", "", "package pattern passed to `go test` (required)")
	f.StringVar(&opts.Bench, "bench", ".", "-bench regexp")
	f.StringVar(&opts.Dir, "dir", ".", "target repository root to run in")
	f.IntVar(&opts.Count, "count", 10, "-count; several samples are required for `verify` to say anything")
	f.StringVar(&opts.Benchtime, "benchtime", "", "-benchtime, e.g. 1s or 2000x")
	f.StringVar(&opts.OutDir, "out", "profadvisor-out", "root for the timestamped output directory")
	f.DurationVar(&opts.Timeout, "timeout", 20*time.Minute, "abort the run after this long")
	_ = c.MarkFlagRequired("pkg")
	return c
}
