package cmd

import (
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/spf13/cobra"
)

// measurementFlags installs the two flags that choose what a run optimizes.
// They are identical on every command that measures anything, and the defaults
// are empty rather than "cpu"/"ns/op" so measurement.Resolve can tell "the user
// said cpu" apart from "the user said nothing" and infer the profile from a
// unit alone: --unit B/op is unambiguous, and requiring --profile memory beside
// it would be ceremony. For block and mutex profiles, --profile must be named
// explicitly, since ns/op alone still means cpu.
func measurementFlags(c *cobra.Command, profile *measurement.Kind, unit *string) {
	f := c.Flags()
	f.StringVar((*string)(profile), "profile", "", "profile capability: cpu, memory, block, or mutex (default cpu; inferred from --unit when omitted)")
	f.StringVar(unit, "unit", "", "objective: ns/op for cpu, block and mutex; B/op (default) or allocs/op for memory")
}
