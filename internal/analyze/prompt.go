package analyze

import (
	"fmt"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/schema"
)

// systemPrompt frames the task. It is deliberately narrow: the model is not
// asked to be a general code reviewer, because a general code reviewer will
// reliably return style advice that no benchmark can confirm. The pipeline can
// only accept a suggestion that moves ns/op, so the prompt asks for exactly
// that and says out loud that a statistical check will follow.
const systemPrompt = `You are a Go performance engineer reading a CPU profile.

You will be given the hottest functions from a pprof CPU profile of a Go
benchmark, each with its source and per-line self time in nanoseconds. Your job
is to pick the ONE change most likely to reduce wall-clock time, explain why the
code is hot, and write the patch.

Rules you must follow:

1. Optimize what the profile shows, not what you assume. Cite the line numbers
   and the nanosecond figures that justify your choice. If the profile does not
   support a change, say so rather than inventing a target.
2. Preserve behaviour exactly. The target's own test suite will be run against
   your patch. A faster function that changes semantics is a failure, not a
   tradeoff.
3. Prefer one focused change over several. Your patch will be measured with
   benchstat; a diff that touches five things cannot be attributed.
4. Do not propose: adding dependencies, unsafe, assembly, build tags, caching
   layers with new lifetimes, or concurrency. Those are out of scope for this
   tool and will be rejected.
5. Do not propose changes to _test.go files. The benchmark is the measuring
   instrument; changing it invalidates the comparison.
6. Reason about cost concretely: allocations in a hot loop, bounds checks, an
   O(n) scan that could be O(1), repeated work that is loop-invariant, an
   interface call that prevents inlining, a conversion that copies. Name the
   mechanism.
7. Your diff must be a unified diff that applies with 'git apply' from the
   repository root, with paths relative to that root and at least 3 lines of
   context. Do not include the profile or commentary inside the diff.

Be honest about confidence. A low-confidence answer that names the real
uncertainty is more useful than a confident guess, because the next step in this
pipeline measures you.`

// buildUserPrompt renders the extract output into the message body.
//
// Ordering matters: the profile summary comes first so the model can see how
// much of the total was actually attributed to the target code, then hotspots in
// rank order. The per-line nanosecond annotations are inlined into the source
// rather than listed separately, because a model reading a listing with the cost
// sitting next to the statement reliably picks a better target than one handed
// the same numbers in a table.
func buildUserPrompt(r *schema.ExtractResult, module string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# CPU profile\n\n")
	fmt.Fprintf(&b, "Profile: %s\n", r.Profile.Path)
	fmt.Fprintf(&b, "Benchmark wall time: %s\n", nanos(r.Profile.DurationNanos))
	fmt.Fprintf(&b, "Total samples: %s\n", nanos(r.Profile.TotalNanos))
	fmt.Fprintf(&b, "Attributed to %s: %s (%.1f%% of total; the rest is runtime and\n"+
		"standard-library time that has been filtered out of the ranking below)\n",
		strings.Join(r.Profile.FocusPrefixes, ", "),
		nanos(r.Profile.AnalyzedNanos),
		pct(r.Profile.AnalyzedNanos, r.Profile.TotalNanos))
	if module != "" {
		fmt.Fprintf(&b, "Repository root module: %s\n", module)
	}

	if len(r.Profile.Excluded) > 0 {
		// The filtered time is reported, not just its size. Filtering removes
		// the runtime frames that would swamp the ranking, but those frames
		// are often the explanation: a loop that boxes a value shows up as one
		// hot user function plus a wall of runtime.convTnoptr, and only the
		// second half says what to change.
		fmt.Fprintf(&b, "\nWhere the filtered time went (these are consequences of the code "+
			"below, not things to edit directly):\n")
		for _, e := range r.Profile.Excluded {
			fmt.Fprintf(&b, "  %-34s %5.1f%% cumulative, %5.1f%% self\n",
				e.Function, e.CumPct, e.FlatPct)
		}
	}

	fmt.Fprintf(&b, "\n# Hotspots, hottest first\n")
	for i, h := range r.Hotspots {
		fmt.Fprintf(&b, "\n## %d. %s\n", i+1, trimShape(h.Function))
		fmt.Fprintf(&b, "self %s (%.1f%% of attributed), cumulative %s (%.1f%% of total)\n",
			nanos(h.FlatNanos), h.FlatPct, nanos(h.CumNanos), h.CumPct)
		if h.Source == nil {
			fmt.Fprintf(&b, "%s:%d — source not available on this machine\n", h.File, h.Line)
			continue
		}
		fmt.Fprintf(&b, "%s:%d\n\n```go\n", h.Source.File, h.Source.StartLine)
		for off, line := range h.Source.Lines {
			ln := h.Source.StartLine + off
			if cost, hot := h.Source.LineNanos[ln]; hot && cost > 0 {
				fmt.Fprintf(&b, "%6d | %-70s // %s self\n", ln, line, nanos(cost))
				continue
			}
			fmt.Fprintf(&b, "%6d | %s\n", ln, line)
		}
		fmt.Fprintf(&b, "```\n")
	}

	fmt.Fprintf(&b, "\n# Your task\n\n"+
		"Pick the single highest-value change and answer in the required JSON schema.\n")

	return b.String()
}

// trimShape strips the generic instantiation the Go compiler bakes into a
// profile's function names. A method on a generic type comes back as
//
//	store.(*cache[go.shape.interface { Key() string }]).lookup
//
// which costs real tokens and tells the model nothing it cannot see in the
// source. The bracketed part is dropped and the name reads as written.
func trimShape(name string) string {
	open := strings.Index(name, "[")
	if open < 0 {
		return name
	}
	depth, i := 0, open
	for ; i < len(name); i++ {
		switch name[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return name[:open] + name[i+1:]
			}
		}
	}
	return name
}

// nanos renders a nanosecond count the way a profile reader expects to see it.
func nanos(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.2fs", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.0fms", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0fµs", float64(n)/1e3)
	default:
		return fmt.Sprintf("%dns", n)
	}
}

func pct(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}
