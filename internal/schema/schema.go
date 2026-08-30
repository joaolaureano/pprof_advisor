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

// Version is stamped into every document so a consumer can reject input
// produced by an incompatible build instead of misreading it.
const Version = 1

// ---------------------------------------------------------------------------
// extract
// ---------------------------------------------------------------------------

// ExtractResult is what `profadvisor extract <cpu.prof>` writes to stdout.
type ExtractResult struct {
	SchemaVersion int         `json:"schema_version"`
	Profile       ProfileMeta `json:"profile"`
	Hotspots      []Hotspot   `json:"hotspots"`
}

// ProfileMeta describes the profile the hotspots were drawn from.
//
// TotalNanos and AnalyzedNanos differ because runtime and standard-library
// frames are filtered out before ranking: a short benchmark can spend the
// majority of its samples in runtime.kevent on an idle netpoller thread, and
// reporting that as the top hotspot would be worse than useless. Percentages in
// Hotspot are renormalized over AnalyzedNanos, and both totals are carried here
// so a reader can see how much was discarded.
type ProfileMeta struct {
	Path          string `json:"path"`
	SampleUnit    string `json:"sample_unit"`
	TotalNanos    int64  `json:"total_nanos"`
	AnalyzedNanos int64  `json:"analyzed_nanos"`
	DurationNanos int64  `json:"duration_nanos"`
	// FocusPrefixes are the package path prefixes that survived filtering.
	FocusPrefixes []string `json:"focus_prefixes,omitempty"`
	// Excluded is where the filtered time actually went, hottest first.
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
	Function  string  `json:"function"`
	FlatNanos int64   `json:"flat_nanos"`
	FlatPct   float64 `json:"flat_pct"`
	CumNanos  int64   `json:"cum_nanos"`
	CumPct    float64 `json:"cum_pct"`
}

// Hotspot is one function ranked by self time.
type Hotspot struct {
	Function  string  `json:"function"`
	File      string  `json:"file"`
	Line      int     `json:"line"`
	FlatNanos int64   `json:"flat_nanos"`
	CumNanos  int64   `json:"cum_nanos"`
	FlatPct   float64 `json:"flat_pct"`
	CumPct    float64 `json:"cum_pct"`
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
	// LineNanos maps an absolute line number to its flat cost, for the lines
	// inside this excerpt that carry any. This is the signal that tells the
	// model which statement is expensive, not just which function.
	LineNanos map[int]int64 `json:"line_nanos,omitempty"`
}

// ---------------------------------------------------------------------------
// analyze
// ---------------------------------------------------------------------------

// Diagnosis is what `profadvisor analyze` writes to stdout.
type Diagnosis struct {
	SchemaVersion int    `json:"schema_version"`
	Model         string `json:"model"`
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
// It compares two `go test -bench -count=N` outputs, not two profiles: a
// p-value requires N samples of ns/op, which a CPU profile does not contain.
type VerifyResult struct {
	SchemaVersion int `json:"schema_version"`
	// Verdict is the roll-up across benchmarks: PIOROU if any benchmark
	// regressed significantly, else MELHOROU if any improved, else SEM DIFERENÇA.
	Verdict     string            `json:"verdict"`
	Comparisons []BenchComparison `json:"comparisons"`
	Warnings    []string          `json:"warnings,omitempty"`
}

// BenchComparison is one benchmark, one unit, before vs. after.
type BenchComparison struct {
	Name string `json:"name"`
	Unit string `json:"unit"`
	// BaselineCenter and AfterCenter are the estimated centers (median or
	// mean, per the assumption used) in the metric's own unit.
	BaselineCenter float64 `json:"baseline_center"`
	AfterCenter    float64 `json:"after_center"`
	BaselineN      int     `json:"baseline_n"`
	AfterN         int     `json:"after_n"`
	// DeltaPct is (after-baseline)/baseline*100. Negative is faster for
	// lower-is-better units such as ns/op.
	DeltaPct float64 `json:"delta_pct"`
	// PValue is NaN-free; when the test could not run, Significant is false
	// and the reason lands in Warnings.
	PValue      float64 `json:"p_value"`
	Significant bool    `json:"significant"`
	Verdict     string  `json:"verdict"`
}
