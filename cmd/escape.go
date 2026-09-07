package cmd

import (
	"time"

	"github.com/joaolaureano/profadvisor/internal/escape"
	"github.com/spf13/cobra"
)

func newEscapeCmd() *cobra.Command {
	var opts escape.Options
	c := &cobra.Command{
		Use:   "escape [--dir <project>]",
		Short: "Report the Go compiler's escape analysis for a project",
		Long: "Runs the Go compiler's escape analysis over a target project and " +
			"writes a stable report documenting which values escape to the heap.\n\n" +
			"The report names the compiler version and path so diagnostics can be " +
			"reproduced by hand: escape analysis improves between releases, and the " +
			"same source can legitimately reach different conclusions on different " +
			"versions or architectures.\n\n" +
			"The compiler's diagnostic phrases are translated to a stable vocabulary " +
			"— kind values like 'escapes_to_heap' instead of wording that may change " +
			"in the next release — but the original text is always kept in the `evidence` " +
			"field. A consumer that switches on `kind` will work across compiler versions; " +
			"one that greps for 'escapes to heap' will break on the next upgrade.\n\n" +
			"An escape is not automatically a problem. The compiler answers where a " +
			"value lives; nothing here ranks findings by importance or suggests code " +
			"should change. Whether an allocation matters requires measurement, which " +
			"is what the rest of the tool is for.\n\n" +
			"Unrecognized diagnostics are reported verbatim rather than guessed at. " +
			"If the toolchain's wording has changed since the parser was last validated, " +
			"those lines appear in the report and a warning is emitted.\n\n" +
			"The analysis needs no benchmark, no profile, no git repository, and no " +
			"API key.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := escape.Run(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Dir, "dir", ".", "target project root")
	f.StringSliceVar(&opts.Patterns, "pkg", []string{"./..."}, "package patterns (repeatable)")
	f.DurationVar(&opts.Timeout, "timeout", 5*time.Minute, "abort the build after this long")
	return c
}
