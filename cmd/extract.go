package cmd

import (
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/spf13/cobra"
)

func newExtractCmd() *cobra.Command {
	var opts extract.Options
	c := &cobra.Command{
		Use:   "extract <cpu.prof>",
		Short: "Rank the hot functions in a CPU profile, with their source",
		Long: "Reads a pprof CPU profile and emits the top functions by self time, " +
			"each with the surrounding source and per-line cost.\n\n" +
			"Runtime and standard-library frames are filtered out of the ranking. " +
			"They are not noise in a general sense, but in a short benchmark they " +
			"dominate — idle netpoller threads alone can be half the samples — and " +
			"ranking them would bury the code under test.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := extract.FromFile(args[0], opts)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := c.Flags()
	f.IntVar(&opts.TopN, "top", 10, "how many hotspots to return")
	f.StringSliceVar(&opts.FocusPrefixes, "focus", nil, "keep only functions under these package prefixes (default: infer the busiest module)")
	f.IntVar(&opts.ContextLines, "context", 8, "source lines to include on each side of the hottest line")
	return c
}
