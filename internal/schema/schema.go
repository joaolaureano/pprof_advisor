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
const Version = 3

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
//
// Read FromFocus, not Cum. Cum is this function's cost across the whole
// profile, which for a filtered frame usually answers the wrong question: on a
// short benchmark, idle netpoller threads give runtime.kevent 40% of cum while
// having nothing to do with the code under test. FromFocus counts only samples
// in which the code under test called this function, directly or through
// others, and it is what this list is ranked by.
type ExcludedCost struct {
	Function string  `json:"function"`
	Flat     int64   `json:"flat"`
	FlatPct  float64 `json:"flat_pct"`
	Cum      int64   `json:"cum"`
	CumPct   float64 `json:"cum_pct"`
	// FromFocus is the cost on paths that pass through the focus package.
	FromFocus    int64   `json:"from_focus"`
	FromFocusPct float64 `json:"from_focus_pct"`
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
	SchemaVersion int `json:"schema_version"`
	// Provider and Model together identify what produced this diff. Once the
	// provider is pluggable a bare model id is ambiguous, and reproducing a
	// result is the only reason to record either.
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model"`
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

// ---------------------------------------------------------------------------
// escape
// ---------------------------------------------------------------------------

// EscapeVersion is the schema version of the escape report.
//
// It is deliberately a separate constant from Version. The capture → extract →
// analyze → apply → verify documents are links in one chain: they are produced
// by one run, passed hand to hand, and they move together. The escape report is
// not in that chain. It has its own reason to change — the Go compiler's
// diagnostic vocabulary — and tying the two would force a version bump on five
// documents every time the toolchain rewords one line.
const EscapeVersion = 1

// EscapeKind is the stable name for what the compiler concluded.
//
// These are profadvisor's words, not the compiler's. Compiler diagnostic text is
// prose, not an API: it has been reworded across releases and will be again, and
// a consumer that greps for a phrase breaks on a toolchain upgrade. Every kind
// here maps one-to-one onto a single compiler message template, so the mapping is
// a translation and never an interpretation — which is why two conclusions the
// compiler states separately stay separate here even when they mean the same
// thing to a reader.
//
// Nothing outside internal/escape sees the original wording except through
// EscapeFinding.Evidence.
type EscapeKind string

const (
	// EscapeMovedToHeap is a named local variable the compiler had to heap
	// allocate. EscapeEscapesToHeap is the same conclusion about an expression
	// with no name. The compiler distinguishes them and so does this.
	EscapeMovedToHeap   EscapeKind = "moved_to_heap"
	EscapeEscapesToHeap EscapeKind = "escapes_to_heap"
	// EscapeDoesNotEscape covers both an expression that stayed on the stack and
	// a parameter that does not outlive its call. The compiler prints the same
	// sentence for both and the text does not say which, so neither does this.
	EscapeDoesNotEscape EscapeKind = "does_not_escape"
	// The leaking-param family: the parameter itself outlives the call, its
	// pointed-to content does, or it reaches a named result.
	EscapeLeakingParam        EscapeKind = "leaking_param"
	EscapeLeakingParamContent EscapeKind = "leaking_param_content"
	EscapeLeakingParamResult  EscapeKind = "leaking_param_result"
	// EscapeParamInert is a parameter the callee neither retains, writes
	// through, nor calls.
	EscapeParamInert   EscapeKind = "param_inert"
	EscapeMutatesParam EscapeKind = "mutates_param"
	EscapeCallsParam   EscapeKind = "calls_param"
	// EscapeUnsafeUintptr is the compiler declining to reason about a pointer
	// laundered through uintptr. It is evidence that the analysis stopped, not
	// evidence about the value.
	EscapeUnsafeUintptr      EscapeKind = "unsafe_uintptr"
	EscapeZeroCopyConversion EscapeKind = "zero_copy_conversion"
	EscapeClosureCapture     EscapeKind = "closure_capture"
	EscapeSelfAssignment     EscapeKind = "self_assignment"
)

// EscapeReport is what `profadvisor escape` writes to stdout.
//
// It reports what the compiler concluded and nothing more. An escape is not
// automatically a cost: a heap allocation on a path that runs once is invisible,
// and the compiler is answering "where does this value live", not "is this code
// too slow". There is deliberately no severity, no ranking, and no suggestion
// anywhere in this document — deciding that code should change requires a
// measurement, which is what the rest of this tool is for.
type EscapeReport struct {
	SchemaVersion int             `json:"schema_version"`
	Toolchain     Toolchain       `json:"toolchain"`
	Analysis      EscapeAnalysis  `json:"analysis"`
	Summary       EscapeSummary   `json:"summary"`
	Findings      []EscapeFinding `json:"findings"`
	// Unrecognized holds every diagnostic line the parser could not map to a
	// kind, verbatim. A compiler that has learned a new sentence must show up
	// here rather than be silently dropped or forced into the nearest kind: a
	// wrong classification is worse than an admitted gap, because only one of
	// them is visible.
	Unrecognized []RawDiagnostic `json:"unrecognized,omitempty"`
	Warnings     []string        `json:"warnings,omitempty"`
}

// Toolchain records the compiler that produced the diagnostics.
//
// A finding is only meaningful beside the toolchain that reached it. Escape
// analysis improves between releases, and the same source can legitimately give
// a different answer on a different version or architecture; a report that does
// not say which compiler spoke cannot be reproduced or trusted later.
type Toolchain struct {
	// Path is the go binary actually used, resolved absolutely. Which `go` is on
	// PATH is not always the one a reader assumes.
	Path string `json:"path"`
	// Version is GOVERSION as reported from inside the target directory, so a
	// go.mod toolchain directive is reflected rather than overlooked.
	Version string `json:"version"`
	Raw     string `json:"version_raw"`
	GOOS    string `json:"goos"`
	GOARCH  string `json:"goarch"`
}

// EscapeAnalysis records how the diagnostics were obtained, so the run can be
// reproduced by hand, and how much of the output the parser actually understood,
// so a reader can tell a quiet report from a blind one.
type EscapeAnalysis struct {
	Dir      string   `json:"dir"`
	Patterns []string `json:"patterns"`
	Command  []string `json:"command"`
	// ParserProfile names the ruleset that read this output. It moves when the
	// compiler's wording does, and it is what makes an old report legible after
	// the parser has changed underneath it.
	ParserProfile string `json:"parser_profile"`
	TotalLines    int    `json:"total_lines"`
	// RecognizedLines became findings. IgnoredLines were understood and
	// deliberately not reported — package headers, inlining decisions, and the
	// explanation headings already folded into a finding. UnrecognizedLines is
	// the gap, and it is the number worth watching after a toolchain upgrade.
	RecognizedLines   int   `json:"recognized_lines"`
	IgnoredLines      int   `json:"ignored_lines"`
	UnrecognizedLines int   `json:"unrecognized_lines"`
	DurationNanos     int64 `json:"duration_nanos"`
}

// EscapeSummary counts what was found. It is a tally, not a judgement: the
// counts say how often the compiler reached each conclusion, and say nothing
// about whether any of it matters.
type EscapeSummary struct {
	Packages int `json:"packages"`
	Files    int `json:"files"`
	Findings int `json:"findings"`
	// ByKind is keyed by EscapeKind.
	ByKind map[string]int `json:"by_kind"`
}

// EscapeFinding is one conclusion the compiler reached, at one position.
type EscapeFinding struct {
	Kind    EscapeKind `json:"kind"`
	Package string     `json:"package"`
	File    string     `json:"file"`
	Line    int        `json:"line"`
	Column  int        `json:"column"`
	// Subject is the expression or identifier the compiler named, verbatim and
	// unparsed — "&T{...}", "ts", "p". These are Go source fragments as the
	// compiler chose to print them, not identifiers this tool can resolve, and
	// treating them as anything more structured than a label is a mistake.
	Subject string `json:"subject"`
	// Function is the enclosing function, known only when the compiler printed
	// an explanation block for this finding, which it does only at -m=2 and only
	// for some kinds.
	Function string `json:"function,omitempty"`
	// Target is where the value went, for the leaking-to-result kind: a result
	// name such as "~r0".
	Target string `json:"target,omitempty"`
	Level  *int   `json:"level,omitempty"`
	Derefs *int   `json:"derefs,omitempty"`
	// ByRef distinguishes the two closure-capture messages.
	ByRef *bool `json:"by_ref,omitempty"`
	// Flow is the compiler's own account of how the value reached its
	// destination.
	Flow []EscapeFlowStep `json:"flow,omitempty"`
	// Evidence is the compiler's line, exactly as printed. It is the record that
	// survives any future change to how this report is modelled: if the kinds
	// above turn out to be the wrong carving, this field still says what
	// actually happened.
	Evidence string `json:"evidence"`
}

// EscapeFlowStep is one line of a flow explanation.
//
// The compiler prints these in two shapes and both land here. A step with an
// empty Reason is the header stating an edge — "flow: ~r0 ← &x" — and carries no
// position. A step with a Reason is one hop along that edge, and carries the
// position where it happened. Text is kept verbatim in both cases because the
// compiler's rendering of a value is not reconstructible from its parts.
type EscapeFlowStep struct {
	Text   string `json:"text"`
	Reason string `json:"reason,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// RawDiagnostic is a line kept exactly as the compiler printed it, with whatever
// position could be read off the front of it.
type RawDiagnostic struct {
	Package string `json:"package,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Text    string `json:"text"`
}
