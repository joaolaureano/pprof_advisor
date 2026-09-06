package analyze

import (
	"fmt"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// systemPrompt frames the task. It is deliberately narrow: the model is not
// asked to be a general code reviewer, because a general code reviewer will
// reliably return style advice that no benchmark can confirm. The pipeline can
// only accept a suggestion that moves the objective, so the prompt asks for
// exactly that and says out loud that a statistical check will follow.
//
// The objective is stated rather than assumed. Told only "make this faster"
// while shown an allocation profile, a model reliably proposes a change that
// trades memory for time, which is the opposite of what a memory run asked
// for — and the guard metric then rejects it, a whole capture-apply-verify
// cycle spent discovering that the prompt was wrong.
func systemPrompt(cfg measurement.Config) string {
	profile, objective, mechanisms := "CPU", "reduce wall-clock time", `allocations in a hot loop,
   bounds checks, an O(n) scan that could be O(1), repeated work that is
   loop-invariant, an interface call that prevents inlining, a conversion that
   copies`
	if cfg.Profile == measurement.Memory {
		profile = "memory allocation"
		objective = "reduce " + cfg.Unit
		mechanisms = `a slice or map grown without a capacity hint, a
   []byte/string conversion that copies, a value escaping to the heap because
   it is stored in an interface or captured by a closure, a buffer allocated
   per call that could be reused or stack-allocated, fmt.Sprintf where append
   would do`
	}
	return `You are a Go performance engineer reading a ` + profile + ` profile.

You will be given the hottest functions from a pprof ` + profile + ` profile of
a Go benchmark, each with its source and per-line self cost. Your job is to pick
the ONE change most likely to ` + objective + `, explain why the code is hot,
and write the patch.

The objective is ` + cfg.Unit + `. That is the number the benchmark will be
judged on. ` + guardRule(cfg) + `

Rules you must follow:

1. Optimize what the profile shows, not what you assume. Cite the line numbers
   and the cost figures that justify your choice. If the profile does not
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
6. Reason about cost concretely: ` + mechanisms + `. Name the mechanism.
7. Your diff must be a unified diff that applies with 'git apply' from the
   repository root, with paths relative to that root and at least 3 lines of
   context. Do not include the profile or commentary inside the diff.

Be honest about confidence. A low-confidence answer that names the real
uncertainty is more useful than a confident guess, because the next step in this
pipeline measures you.`
}

// guardRule tells the model what else is being watched, in the same sentence
// as the objective, so a trade it cannot make is not one it has to discover.
func guardRule(cfg measurement.Config) string {
	if cfg.Profile == measurement.Memory {
		return "Wall-clock time (ns/op) is measured as a guard: a patch that " +
			"allocates less but runs significantly slower will be rejected, so do " +
			"not buy memory with time. Allocation count and byte volume move " +
			"independently and only " + cfg.Unit + " decides."
	}
	return "Bytes and allocations per operation are reported alongside it but " +
		"do not decide the verdict."
}

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
	cfg := r.Profile.Measurement
	cost := coster(cfg)

	kind := "CPU"
	if cfg.Profile == measurement.Memory {
		kind = "Memory"
	}
	fmt.Fprintf(&b, "# %s profile\n\n", kind)
	fmt.Fprintf(&b, "Profile: %s\n", r.Profile.Path)
	fmt.Fprintf(&b, "Objective: %s (%s)\n", cfg.Unit, cfg.SampleType)
	if r.Profile.DurationNanos > 0 {
		fmt.Fprintf(&b, "Benchmark wall time: %s\n", nanos(r.Profile.DurationNanos))
	}
	fmt.Fprintf(&b, "Total %s: %s\n", cfg.SampleUnit, cost(r.Profile.Total))
	fmt.Fprintf(&b, "Attributed to %s: %s (%.1f%% of total; the rest is runtime and\n"+
		"standard-library cost that has been filtered out of the ranking below)\n",
		strings.Join(r.Profile.FocusPrefixes, ", "),
		cost(r.Profile.Analyzed),
		pct(r.Profile.Analyzed, r.Profile.Total))
	if cfg.Attribution == "first_focus_frame" {
		// Without this the model reads "self" as "allocated by its own
		// statements" and goes looking for a make() that is one frame down.
		fmt.Fprintf(&b, "Attribution: every allocation is charged to the innermost frame in\n"+
			"the code under test, not to the runtime allocator underneath it.\n")
	}
	if module != "" {
		fmt.Fprintf(&b, "Repository root module: %s\n", module)
	}

	if len(r.Profile.Excluded) > 0 {
		// The filtered time is reported, not just its size. Filtering removes
		// the runtime frames that would swamp the ranking, but those frames
		// are often the explanation: a loop that boxes a value shows up as one
		// hot user function plus a wall of runtime.convTnoptr, and only the
		// second half says what to change.
		fmt.Fprintf(&b, "\nWhere the filtered cost went (these are consequences of the code "+
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
			cost(h.Flat), h.FlatPct, cost(h.Cum), h.CumPct)
		if h.Source == nil {
			fmt.Fprintf(&b, "%s:%d — source not available on this machine\n", h.File, h.Line)
			continue
		}
		fmt.Fprintf(&b, "%s:%d\n\n```go\n", h.Source.File, h.Source.StartLine)
		for off, line := range h.Source.Lines {
			ln := h.Source.StartLine + off
			if value, hot := h.Source.LineCosts[ln]; hot && value > 0 {
				fmt.Fprintf(&b, "%6d | %-70s // %s self\n", ln, line, cost(value))
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

// coster returns the renderer for the profile's sample unit. A profile reader
// expects nanoseconds as "1.20ms" and bytes as "4.0 MB"; printing either as a
// bare integer wastes the model's attention on arithmetic.
func coster(cfg measurement.Config) func(int64) string {
	switch cfg.SampleUnit {
	case "bytes":
		return bytesCost
	case "count":
		return func(n int64) string { return fmt.Sprintf("%d allocs", n) }
	default:
		return nanos
	}
}

func bytesCost(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
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
