// Package schema defines the JSON contract between profadvisor subcommands.
//
// Every subcommand reads one of these types from a file (or stdin) and writes
// another to stdout. Keeping the shapes in one package is what makes
// `extract | analyze | apply` compose by construction rather than by
// convention: if a producer and a consumer disagree, they fail to compile.
//
// Nothing in this package formats, prints, or exits. Field names are the wire
// format and are not free to rename.
package schema

import "github.com/joaolaureano/profadvisor/internal/measurement"

// Version is stamped into every document so a consumer can reject input
// produced by an incompatible build instead of misreading it.
//
// Version 2 made the measurement explicit. Version 1 spoke only nanoseconds:
// every cost field was named *_nanos and every consumer was free to assume CPU
// time. Once a run can optimize bytes per operation those names are lies, so
// the costs became unit-less numbers and each document now carries the
// measurement.Config that says how to read them.
const Version = 2

// ---------------------------------------------------------------------------
// extract
// ---------------------------------------------------------------------------

// ExtractResult is what `profadvisor extract <profile>` writes to stdout.
type ExtractResult struct {
	SchemaVersion int         `json:"schema_version"`
	Profile       ProfileMeta `json:"profile"`
	Hotspots      []Hotspot   `json:"hotspots"`
}

// ProfileMeta describes the profile the hotspots were drawn from.
//
// Total and Analyzed differ because runtime and standard-library frames are
// filtered out before ranking: a short benchmark can spend the majority of its
// samples in runtime.kevent on an idle netpoller thread, and reporting that as
// the top hotspot would be worse than useless. Percentages in Hotspot are
// renormalized over Analyzed, and both totals are carried here so a reader can
// see how much was discarded.
//
// Both are in Measurement.SampleUnit — nanoseconds for a CPU profile, bytes or
// object counts for a memory one. Reading them without consulting Measurement
// is what version 2 exists to prevent.
type ProfileMeta struct {
	Path        string             `json:"path"`
	Measurement measurement.Config `json:"measurement"`
	Total       int64              `json:"total"`
	Analyzed    int64              `json:"analyzed"`
	// DurationNanos is wall-clock and therefore genuinely nanoseconds
	// whatever the profile measures. Heap profiles do not carry it; zero
	// means unknown, not instantaneous.
	DurationNanos int64 `json:"duration_nanos"`
	// FocusPrefixes are the package path prefixes that survived filtering.
	FocusPrefixes []string `json:"focus_prefixes,omitempty"`
	// Excluded is where the filtered cost actually went, largest first.
	//
	// It matters because filtering can remove the explanation along with the
	// noise. A per-pixel loop that boxes a value on every iteration shows up
	// as one hot user function and a pile of runtime.mallocgc; without this
	// list the reader is told which function is hot but not that the cost is
	// allocation, which is the only part that suggests what to change.
	Excluded []ExcludedCost `json:"excluded,omitempty"`
}

// ExcludedCost is one function kept out of the ranking, with what it cost.
type ExcludedCost struct {
	Function string  `json:"function"`
	Flat     int64   `json:"flat"`
	FlatPct  float64 `json:"flat_pct"`
	Cum      int64   `json:"cum"`
	CumPct   float64 `json:"cum_pct"`
}

// Hotspot is one function ranked by its self cost.
//
// "Self" is not the same question for every profile, and Measurement.Attribution
// records which one was asked. For CPU it is the leaf frame: the code that was
// executing. For memory the leaf is almost always runtime.mallocgc, so the cost
// is attributed to the first frame belonging to the code under test — the
// function that asked for the memory, which is the one a patch can change.
type Hotspot struct {
	Function string  `json:"function"`
	File     string  `json:"file"`
	Line     int     `json:"line"`
	Flat     int64   `json:"flat"`
	Cum      int64   `json:"cum"`
	FlatPct  float64 `json:"flat_pct"`
	CumPct   float64 `json:"cum_pct"`
	// Source is best-effort: nil when the file named by the profile is not
	// readable on this machine, which is normal for a profile captured
	// elsewhere and must never be an error.
	Source *SourceExcerpt `json:"source,omitempty"`
}

// SourceExcerpt is the code the LLM is asked to reason about.
type SourceExcerpt struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	// Lines holds StartLine..EndLine inclusive, without trailing newlines.
	Lines []string `json:"lines"`
	// LineCosts maps an absolute line number to its flat cost, for the lines
	// inside this excerpt that carry any. This is the signal that tells the
	// model which statement is expensive, not just which function.
	LineCosts map[int]int64 `json:"line_costs,omitempty"`
}

// ---------------------------------------------------------------------------
// analyze
// ---------------------------------------------------------------------------

// Diagnosis is what `profadvisor analyze` writes to stdout.
type Diagnosis struct {
	SchemaVersion int    `json:"schema_version"`
	Model         string `json:"model"`
	// Measurement is carried through from the profile so `apply` and `verify`
	// judge the suggestion against the objective it was asked for.
	Measurement measurement.Config `json:"measurement"`
	// Target names the function the model chose to attack, which is not
	// necessarily the top hotspot.
	Target string `json:"target"`
	// Cause is the model's explanation of why the target is hot.
	Cause string `json:"cause"`
	// Change is a prose summary of the proposed rewrite.
	Change string `json:"change"`
	// Diff is a unified diff, applicable with `git apply` from the repo root.
	Diff string `json:"diff"`
	// Confidence is the model's own estimate: "high", "medium", or "low".
	Confidence string `json:"confidence"`
	// Risks are behavioural changes the model believes the diff could cause.
	Risks []string `json:"risks,omitempty"`
}

// ---------------------------------------------------------------------------
// apply
// ---------------------------------------------------------------------------

// ApplyResult is what `profadvisor apply` writes to stdout.
type ApplyResult struct {
	SchemaVersion int      `json:"schema_version"`
	Branch        string   `json:"branch"`
	BaseRef       string   `json:"base_ref"`
	Commit        string   `json:"commit"`
	FilesChanged  []string `json:"files_changed"`
}

// ---------------------------------------------------------------------------
// verify
// ---------------------------------------------------------------------------

// Verdict values. These are the strings the CLI prints and AGENTS.md documents;
// they are part of the contract.
const (
	VerdictImproved  = "MELHOROU"
	VerdictNoChange  = "SEM DIFERENÇA"
	VerdictRegressed = "PIOROU"
)

// VerifyResult is what `profadvisor verify` writes to stdout.
//
// It compares two `go test -bench -benchmem -count=N` outputs, not two
// profiles: a p-value requires N samples of the metric, which a profile does
// not contain.
type VerifyResult struct {
	SchemaVersion int `json:"schema_version"`
	// Measurement names the objective. Comparisons carries every metric that
	// was read, but only this one decides whether the change was worth making.
	Measurement measurement.Config `json:"measurement"`
	// Verdict is the roll-up. It is decided by the objective and the guard
	// metrics only: a regression in either is PIOROU, otherwise an improvement
	// in the objective is MELHOROU, otherwise SEM DIFERENÇA. Informational
	// metrics are reported and never voted, which is what lets a memory run
	// accept a patch that allocates fewer bytes in more, smaller pieces.
	Verdict     string            `json:"verdict"`
	Comparisons []BenchComparison `json:"comparisons"`
	Warnings    []string          `json:"warnings,omitempty"`
}

// BenchComparison is one benchmark, one unit, before vs. after.
type BenchComparison struct {
	Name string `json:"name"`
	Unit string `json:"unit"`
	// Role is measurement.Objective, Guard, or Informational — what this
	// metric is allowed to decide.
	Role string `json:"role"`
	// BaselineCenter and AfterCenter are the estimated centers (median or
	// mean, per the assumption used) in the metric's own unit.
	BaselineCenter float64 `json:"baseline_center"`
	AfterCenter    float64 `json:"after_center"`
	BaselineN      int     `json:"baseline_n"`
	AfterN         int     `json:"after_n"`
	// DeltaPct is (after-baseline)/baseline*100. Negative is better for
	// lower-is-better units such as ns/op and B/op.
	DeltaPct float64 `json:"delta_pct"`
	// PValue is NaN-free; when the test could not run, Significant is false
	// and the reason lands in Warnings.
	PValue      float64 `json:"p_value"`
	Significant bool    `json:"significant"`
	Verdict     string  `json:"verdict"`
}
