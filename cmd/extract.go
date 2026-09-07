package cmd

import (
	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/spf13/cobra"
)

func newExtractCmd() *cobra.Command {
	var (
		opts    extract.Options
		profile measurement.Kind
		unit    string
	)
	c := &cobra.Command{
		Use:   "extract <profile>",
		Short: "Rank the hot functions in a profile, with their source",
		Long: "Reads a pprof profile and emits the top functions by self cost, each " +
			"with the surrounding source and per-line cost.\n\n" +
			"Runtime and standard-library frames are filtered out of the ranking. " +
			"They are not noise in a general sense, but in a short benchmark they " +
			"dominate — idle netpoller threads alone can be half the samples — and " +
			"ranking them would bury the code under test.\n\n" +
			"For a memory profile, an allocation is charged to the innermost frame " +
			"in the code under test rather than to the leaf. The leaf of every " +
			"allocation sample is runtime.mallocgc, and ranking that produces one " +
			"enormous hotspot nobody can edit. The same attribution applies to block " +
			"and mutex contention profiles: the leaf of a contention sample is " +
			"runtime.chanrecv or sync.(*Mutex).Lock, never the code that caused the wait. " +
			"Cost is charged to the call that caused the wait, which is the line a patch " +
			"can change.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := measurement.Resolve(profile, unit)
			if err != nil {
				return err
			}
			opts.Measurement = cfg
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
	measurementFlags(c, &profile, &unit)
	return c
}
