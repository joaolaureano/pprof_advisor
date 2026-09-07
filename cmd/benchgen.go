package cmd

import (
	"github.com/joaolaureano/profadvisor/internal/benchgen"
	"github.com/spf13/cobra"
)

func newBenchgenCmd() *cobra.Command {
	var opts benchgen.Options
	c := &cobra.Command{
		Use:   "benchgen --pkg <package> --func <name> --corpus <directory> --out <directory>",
		Short: "Generate offline fuzz tests and benchmarks from a frozen Go corpus",
		Long: "Generate a same-package fuzz test and benchmark without a model or API key. " +
			"The function must be deterministic, have no external state, and neither modify nor retain its argument. " +
			"Only non-generic, non-variadic functions taking exactly string or []byte are supported. " +
			"Returns are discarded; fuzzing detects panics and does not infer correctness properties. " +
			"Requires Go 1.24+ in the target. Generation does not execute the target: replay seeds before measuring. " +
			"Artifacts go to --out; --write also installs the test file in the target package. Existing files are refused.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := benchgen.Generate(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return emit(cmd, result)
		},
	}
	f := c.Flags()
	f.StringVar(&opts.Dir, "dir", ".", "target repository root")
	f.StringVar(&opts.Package, "pkg", "", "exactly one target package")
	f.StringVar(&opts.Function, "func", "", "package-level function name (may be unexported)")
	f.StringVar(&opts.Corpus, "corpus", "", "directory of Go fuzz v1 seed files")
	f.StringVar(&opts.Out, "out", "", "artifact directory")
	f.BoolVar(&opts.Write, "write", false, "also install the generated test in the target package")
	for _, name := range []string{"pkg", "func", "corpus", "out"} {
		_ = c.MarkFlagRequired(name)
	}
	return c
}
