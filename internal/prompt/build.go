package prompt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// promptBuilder accumulates a prompt from catalog entries, holding the first error
// instead of returning one at every call site. A render failure means the
// catalog and this code disagree about a placeholder, which is a bug rather
// than a runtime condition, so the sticky error is checked once at the end.
type promptBuilder struct {
	cat *Catalog
	b   strings.Builder
	err error
}

// put appends one catalog entry. vars must supply exactly the placeholders the
// entry declares; the catalog rejects anything else.
func (r *promptBuilder) put(key string, vars map[string]string) {
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
func (r *promptBuilder) raw(s string) {
	if r.err == nil {
		r.b.WriteString(s)
	}
}

func (r *promptBuilder) done() (string, error) {
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
// For a contention profile, the objective remains ns/op (reducing wall-clock time)
// because that is what the benchmark measures and what the patch will be tested on.
// The profile and mechanisms change to describe the contention pattern rather than
// the typical path.
//
// The wording itself lives in the catalog, so what the tool says can be read
// and changed in one place without recompiling.
func systemPrompt(cat *Catalog, cfg measurement.Config) (string, error) {
	// Three objectives select parallel sets of fragments. Determine the suffix
	// and build vars maps for each fragment separately, since not all fragments
	// need the same variables.
	var suffix string
	switch cfg.Profile {
	case measurement.CPU:
		suffix = "cpu"
	case measurement.Memory:
		suffix = "memory"
	case measurement.Block, measurement.Mutex:
		suffix = "contention"
	}

	// Render each fragment with its own appropriate vars.
	profileVars := map[string]string{}
	if suffix == "contention" {
		profileVars["kind"] = string(cfg.Profile)
	}

	profileStr, err := cat.Render("system.profile."+suffix, profileVars)
	if err != nil {
		return "", err
	}

	objectiveVars := map[string]string{}
	if suffix == "memory" {
		objectiveVars["unit"] = cfg.Unit
	}

	objectiveStr, err := cat.Render("system.objective."+suffix, objectiveVars)
	if err != nil {
		return "", err
	}

	mechanismsStr, err := cat.Render("system.mechanisms."+suffix, noVars)
	if err != nil {
		return "", err
	}

	guardVars := map[string]string{}
	if suffix == "memory" {
		guardVars["unit"] = cfg.Unit
	}

	guardStr, err := cat.Render("system.guard."+suffix, guardVars)
	if err != nil {
		return "", err
	}

	scopeStr, err := cat.Render("system.scope."+suffix, noVars)
	if err != nil {
		return "", err
	}

	return cat.Render("system.base", map[string]string{
		"profile":    profileStr,
		"objective":  objectiveStr,
		"unit":       cfg.Unit,
		"guard":      guardStr,
		"mechanisms": mechanismsStr,
		"scope":      scopeStr,
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
func buildUserPrompt(cat *Catalog, r *schema.ExtractResult, module string) (string, error) {
	out := &promptBuilder{cat: cat}
	cfg := r.Profile.Measurement
	cost := measurement.Coster(cfg)

	var kind string
	switch cfg.Profile {
	case measurement.CPU:
		kind = "CPU"
	case measurement.Memory:
		kind = "Memory"
	case measurement.Block:
		kind = "Block contention"
	case measurement.Mutex:
		kind = "Mutex contention"
	}
	out.put("user.header", map[string]string{"kind": kind})
	out.put("user.profile_line", map[string]string{"path": r.Profile.Path})
	out.put("user.objective_line", map[string]string{"unit": cfg.Unit, "sample_type": cfg.SampleType})
	if r.Profile.DurationNanos > 0 {
		out.put("user.walltime_line", map[string]string{"walltime": measurement.Nanos(r.Profile.DurationNanos)})
	}
	out.put("user.total_line", map[string]string{"sample_unit": cfg.SampleUnit, "total": cost(r.Profile.Total)})
	out.put("user.attributed_line", map[string]string{
		"focus":      strings.Join(r.Profile.FocusPrefixes, ", "),
		"attributed": cost(r.Profile.Analyzed),
		"pct":        measurement.Dec1(measurement.Pct(r.Profile.Analyzed, r.Profile.Total)),
	})
	if cfg.Attribution == "first_focus_frame" {
		// Without this the model reads "self" as "allocated by its own
		// statements" and goes looking for a make() that is one frame down.
		// For memory profiles, note that allocations are charged to user code.
		// For contention profiles, note that blocking events are charged to user code.
		if cfg.Profile == measurement.Memory {
			out.put("user.attribution_note.memory", noVars)
		} else {
			out.put("user.attribution_note.contention", noVars)
		}
		// For contention profiles, also emit a note about summed wall-clock time.
		if cfg.Profile == measurement.Block || cfg.Profile == measurement.Mutex {
			out.put("user.contention_note", noVars)
		}
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
			"n": strconv.Itoa(i + 1), "function": measurement.TrimShape(h.Function),
		})
		out.put("user.hotspot_cost", map[string]string{
			"flat": cost(h.Flat), "flat_pct": measurement.Dec1(h.FlatPct),
			"cum": cost(h.Cum), "cum_pct": measurement.Dec1(h.CumPct),
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

// ResponseSchema is the JSON schema of the answer the prompt asks for. It is
// emitted alongside the prompt so a caller wiring its own structured-output
// request sends the same contract this catalog describes.
func (c *Catalog) ResponseSchema() (map[string]any, error) {
	return responseSchema(c)
}

func responseSchema(cat *Catalog) (map[string]any, error) {
	str := func(key string) (map[string]any, error) {
		d, err := cat.Render(key, map[string]string{})
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "string", "description": d}, nil
	}
	props := map[string]any{}
	for field, key := range map[string]string{
		"target": "schema.target", "cause": "schema.cause",
		"change": "schema.change", "diff": "schema.diff",
	} {
		p, err := str(key)
		if err != nil {
			return nil, err
		}
		props[field] = p
	}
	conf, err := str("schema.confidence")
	if err != nil {
		return nil, err
	}
	conf["enum"] = []string{"high", "medium", "low"}
	props["confidence"] = conf

	risks, err := cat.Render("schema.risks", map[string]string{})
	if err != nil {
		return nil, err
	}
	props["risks"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": risks,
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"target", "cause", "change", "diff", "confidence", "risks"},
		"properties":           props,
	}, nil
}

// Build renders the two halves of the request for one extract document: the
// system prompt, which frames the objective, and the user prompt, which carries
// the hotspots and their source. module names the target module, used to label
// paths; an empty string leaves them as the profile recorded them.
func (c *Catalog) Build(r *schema.ExtractResult, module string) (system, user string, err error) {
	if r == nil || len(r.Hotspots) == 0 {
		return "", "", fmt.Errorf("prompt: extract result has no hotspots")
	}
	cfg := r.Profile.Measurement
	if (cfg == measurement.Config{}) {
		resolved, rerr := measurement.Resolve("", "")
		if rerr != nil {
			return "", "", rerr
		}
		cfg = resolved
	}
	if verr := cfg.Validate(); verr != nil {
		return "", "", fmt.Errorf("prompt: %w", verr)
	}
	system, err = systemPrompt(c, cfg)
	if err != nil {
		return "", "", fmt.Errorf("prompt: system: %w", err)
	}
	user, err = buildUserPrompt(c, r, module)
	if err != nil {
		return "", "", fmt.Errorf("prompt: user: %w", err)
	}
	return system, user, nil
}
