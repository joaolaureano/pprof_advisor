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
		Short: "Run a benchmark, saving a profile and the benchmark output",
		Long: "Runs `go test -bench -benchmem` once, keeping both artifacts the " +
			"pipeline needs:\n" +
			"  cpu.prof / mem.prof / block.prof / mutex.prof  the profile, input to `extract`\n" +
			"  bench.txt                                       the raw benchmark output, input to `verify`\n\n" +
			"Which profile is written follows from --profile. A contention profile (block or mutex) is judged on " +
			"ns/op like a CPU run, because `go test -bench` reports no contention metric — the profile changes " +
			"the evidence, not the verdict. -benchmem is always on, whatever the objective, because `verify` " +
			"needs ns/op, B/op and allocs/op from the same output to guard one against another.\n\nBoth artifacts " +
			"come from the same run so the diagnosis and the verdict describe the same program.",
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
	f.IntVar(&opts.Rate, "rate", 1, "sampling detail for a contention profile: -blockprofilerate (block) or -mutexprofilefraction (mutex); 1 records every event")
	f.StringVar(&opts.OutDir, "out", "profadvisor-out", "root for the timestamped output directory")
	f.DurationVar(&opts.Timeout, "timeout", 20*time.Minute, "abort the run after this long")
	measurementFlags(c, &opts.Profile, &opts.Unit)
	_ = c.MarkFlagRequired("pkg")
	return c
}
