package analyze

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/prompt"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// render accumulates a prompt from catalog entries, holding the first error
// instead of returning one at every call site. A render failure means the
// catalog and this code disagree about a placeholder, which is a bug rather
// than a runtime condition, so the sticky error is checked once at the end.
type render struct {
	cat *prompt.Catalog
	b   strings.Builder
	err error
}

// put appends one catalog entry. vars must supply exactly the placeholders the
// entry declares; the catalog rejects anything else.
func (r *render) put(key string, vars map[string]string) {
	if r.err != nil {
		return
	}
	s, err := r.cat.Render(key, vars)
	if err != nil {
		r.err = err
		return
	}
	r.b.WriteString(s)
}

// raw appends text that is layout rather than prose — column alignment and
// fenced source, which belong next to the loop that produces them.
func (r *render) raw(s string) {
	if r.err == nil {
		r.b.WriteString(s)
	}
}

func (r *render) done() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.b.String(), nil
}

var noVars = map[string]string{}

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
//
// The wording itself lives in the catalog, so what the tool says can be read
// and changed in one place without recompiling.
func systemPrompt(cat *prompt.Catalog, cfg measurement.Config) (string, error) {
	// The two objectives select parallel sets of fragments, so the suffix picks
	// the variant and the unit is only needed by the ones that name it.
	suffix, vars := "cpu", noVars
	if cfg.Profile == measurement.Memory {
		suffix, vars = "memory", map[string]string{"unit": cfg.Unit}
	}
	part := map[string]string{}
	for _, name := range []string{"profile", "objective", "mechanisms", "guard"} {
		v := noVars
		switch name {
		case "objective", "guard":
			v = vars
		}
		s, err := cat.Render("system."+name+"."+suffix, v)
		if err != nil {
			return "", err
		}
		part[name] = s
	}
	return cat.Render("system.base", map[string]string{
		"profile":    part["profile"],
		"objective":  part["objective"],
		"unit":       cfg.Unit,
		"guard":      part["guard"],
		"mechanisms": part["mechanisms"],
	})
}

// buildUserPrompt renders the extract output into the message body.
//
// Ordering matters: the profile summary comes first so the model can see how
// much of the total was actually attributed to the target code, then hotspots in
// rank order. The per-line cost annotations are inlined into the source rather
// than listed separately, because a model reading a listing with the cost
// sitting next to the statement reliably picks a better target than one handed
// the same numbers in a table.
func buildUserPrompt(cat *prompt.Catalog, r *schema.ExtractResult, module string) (string, error) {
	out := &render{cat: cat}
	cfg := r.Profile.Measurement
	cost := coster(cfg)

	kind := "CPU"
	if cfg.Profile == measurement.Memory {
		kind = "Memory"
	}
	out.put("user.header", map[string]string{"kind": kind})
	out.put("user.profile_line", map[string]string{"path": r.Profile.Path})
	out.put("user.objective_line", map[string]string{"unit": cfg.Unit, "sample_type": cfg.SampleType})
	if r.Profile.DurationNanos > 0 {
		out.put("user.walltime_line", map[string]string{"walltime": nanos(r.Profile.DurationNanos)})
	}
	out.put("user.total_line", map[string]string{"sample_unit": cfg.SampleUnit, "total": cost(r.Profile.Total)})
	out.put("user.attributed_line", map[string]string{
		"focus":      strings.Join(r.Profile.FocusPrefixes, ", "),
		"attributed": cost(r.Profile.Analyzed),
		"pct":        dec1(pct(r.Profile.Analyzed, r.Profile.Total)),
	})
	if cfg.Attribution == "first_focus_frame" {
		// Without this the model reads "self" as "allocated by its own
		// statements" and goes looking for a make() that is one frame down.
		out.put("user.attribution_note", noVars)
	}
	if module != "" {
		out.put("user.module_line", map[string]string{"module": module})
	}

	if len(r.Profile.Excluded) > 0 {
		// The filtered cost is reported, not just its size. Filtering removes
		// the runtime frames that would swamp the ranking, but those frames
		// are often the explanation: a loop that boxes a value shows up as one
		// hot user function plus a wall of runtime.convTnoptr, and only the
		// second half says what to change.
		out.put("user.excluded_intro", noVars)
		for _, e := range r.Profile.Excluded {
			out.raw(fmt.Sprintf("  %-34s %5.1f%% cumulative, %5.1f%% self\n",
				e.Function, e.CumPct, e.FlatPct))
		}
	}

	out.put("user.hotspots_heading", noVars)
	for i, h := range r.Hotspots {
		out.put("user.hotspot_heading", map[string]string{
			"n": strconv.Itoa(i + 1), "function": trimShape(h.Function),
		})
		out.put("user.hotspot_cost", map[string]string{
			"flat": cost(h.Flat), "flat_pct": dec1(h.FlatPct),
			"cum": cost(h.Cum), "cum_pct": dec1(h.CumPct),
		})
		if h.Source == nil {
			out.put("user.source_unavailable", map[string]string{
				"file": h.File, "line": strconv.Itoa(h.Line),
			})
			continue
		}
		out.put("user.source_open", map[string]string{
			"file": h.Source.File, "line": strconv.Itoa(h.Source.StartLine),
		})
		for off, line := range h.Source.Lines {
			ln := h.Source.StartLine + off
			if value, hot := h.Source.LineCosts[ln]; hot && value > 0 {
				out.raw(fmt.Sprintf("%6d | %-70s // %s self\n", ln, line, cost(value)))
				continue
			}
			out.raw(fmt.Sprintf("%6d | %s\n", ln, line))
		}
		out.put("user.source_close", noVars)
	}

	out.put("user.task", noVars)
	return out.done()
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

// dec1 matches the %.1f the prompts were written against, so a percentage
// substituted into a catalog entry reads the same as when it was a format verb.
func dec1(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
