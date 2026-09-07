package escape

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestParseRecordedOutput parses the recorded compiler output and compares it
// to a golden JSON file.
func TestParseRecordedOutput(t *testing.T) {
	data, err := readFixture("escape/raw-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := Parse(data, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Normalize for comparison.
	normalizeResult(result)

	// Marshal to JSON.
	got, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	compareGolden(t, "escape/parsed-go1.26.1.json", got)
}

// TestParseMutationsOutput parses the mutations-variant compiler output.
func TestParseMutationsOutput(t *testing.T) {
	data, err := readFixture("escape/raw-mutations-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := Parse(data, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	normalizeResult(result)

	got, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	compareGolden(t, "escape/parsed-mutations-go1.26.1.json", got)
}

// TestParseLeavesNothingUnrecognized asserts that both recorded fixtures parse
// completely with no unrecognized diagnostics. This is the canary: if the
// compiler rewords an output line, this test fails loudly.
func TestParseLeavesNothingUnrecognized(t *testing.T) {
	fixtures := []string{
		"escape/raw-go1.26.1.txt",
		"escape/raw-mutations-go1.26.1.txt",
	}

	for _, fixtureName := range fixtures {
		t.Run(fixtureName, func(t *testing.T) {
			data, err := readFixture(fixtureName)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			result, err := Parse(data, "go1.26.1")
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(result.Unrecognized) > 0 {
				t.Errorf("expected zero unrecognized lines, got %d:", len(result.Unrecognized))
				for i, u := range result.Unrecognized {
					t.Logf("  [%d] %s:%d:%d %s", i, u.File, u.Line, u.Column, u.Text)
				}
			}
		})
	}
}

// TestParseKindCoverage asserts that the parser produces findings for each
// expected EscapeKind across the recorded fixtures.
func TestParseKindCoverage(t *testing.T) {
	data1, err := readFixture("escape/raw-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture 1: %v", err)
	}
	result1, err := Parse(data1, "go1.26.1")
	if err != nil {
		t.Fatalf("parse fixture 1: %v", err)
	}

	data2, err := readFixture("escape/raw-mutations-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture 2: %v", err)
	}
	result2, err := Parse(data2, "go1.26.1")
	if err != nil {
		t.Fatalf("parse fixture 2: %v", err)
	}

	// Combine findings.
	allFindings := append(result1.Findings, result2.Findings...)

	// Track which kinds we've seen.
	seenKinds := make(map[schema.EscapeKind]bool)
	for _, f := range allFindings {
		seenKinds[f.Kind] = true
	}

	// Assert each required kind is present.
	requiredKinds := []schema.EscapeKind{
		schema.EscapeMovedToHeap,
		schema.EscapeEscapesToHeap,
		schema.EscapeDoesNotEscape,
		schema.EscapeLeakingParam,
		schema.EscapeLeakingParamContent,
		schema.EscapeLeakingParamResult,
		schema.EscapeParamInert,
		schema.EscapeMutatesParam,
		schema.EscapeClosureCapture,
	}

	for _, kind := range requiredKinds {
		if !seenKinds[kind] {
			t.Errorf("expected kind %q to appear in fixtures, but it did not", kind)
		}
	}
}

// TestParseAttachesFlowAndFunction finds the "moved to heap: n" finding in
// closure.go and asserts that its Function field is "closureByRef" and that
// it has flow steps.
func TestParseAttachesFlowAndFunction(t *testing.T) {
	data, err := readFixture("escape/raw-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := Parse(data, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Find the "moved to heap: n" finding in closure.go.
	var found *schema.EscapeFinding
	for i := range result.Findings {
		f := &result.Findings[i]
		if f.File == "closure.go" && f.Kind == schema.EscapeMovedToHeap && f.Subject == "n" {
			found = f
			break
		}
	}

	if found == nil {
		t.Fatal("expected to find a 'moved to heap: n' finding in closure.go")
	}

	if found.Function != "closureByRef" {
		t.Errorf("expected Function == %q, got %q", "closureByRef", found.Function)
	}

	if len(found.Flow) == 0 {
		t.Error("expected Flow to have steps, but it was empty")
	}

	// A "flow:" header states the edge and carries no reason; only the "from"
	// hops beneath it do. At least one hop has to have parsed, or the
	// explanation is a list of opaque strings and the parser has added nothing.
	var hops int
	for _, step := range found.Flow {
		if step.Reason == "" {
			continue
		}
		hops++
		if step.File == "" || step.Line == 0 {
			t.Errorf("flow hop %q has a reason but no position", step.Text)
		}
	}
	if hops == 0 {
		t.Errorf("no flow hop carried a reason; got %+v", found.Flow)
	}
	// This particular escape is a closure capture, and the compiler says so in
	// the flow rather than in the summary line. Losing that is losing the answer
	// to "why".
	var reasons []string
	for _, step := range found.Flow {
		reasons = append(reasons, step.Reason)
	}
	if !slices.Contains(reasons, "captured by a closure") {
		t.Errorf("flow reasons = %q, want one of them to name the closure capture", reasons)
	}
}

// TestParsePreservesEvidence asserts that every finding's Evidence field is
// non-empty and appears verbatim in the raw input.
func TestParsePreservesEvidence(t *testing.T) {
	data, err := readFixture("escape/raw-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := Parse(data, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rawText := string(data)
	lines := strings.Split(rawText, "\n")
	lineSet := make(map[string]bool)
	for _, line := range lines {
		if line != "" {
			lineSet[line] = true
		}
	}

	for i, f := range result.Findings {
		if f.Evidence == "" {
			t.Errorf("finding %d has empty Evidence", i)
			continue
		}
		if !lineSet[f.Evidence] {
			t.Errorf("finding %d Evidence not found in raw input: %q", i, f.Evidence)
		}
	}
}

// TestParseRefusesToGuess asserts that an unrecognized line is preserved
// verbatim in Unrecognized rather than being forced into a Kind.
func TestParseRefusesToGuess(t *testing.T) {
	input := []byte("x.go:1:1: gremlins ate the stack\n")
	result, err := Parse(input, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Errorf("expected zero findings, got %d", len(result.Findings))
	}

	if len(result.Unrecognized) != 1 {
		t.Fatalf("expected 1 unrecognized line, got %d", len(result.Unrecognized))
	}

	u := result.Unrecognized[0]
	if u.File != "x.go" {
		t.Errorf("expected File == %q, got %q", "x.go", u.File)
	}
	if u.Line != 1 {
		t.Errorf("expected Line == 1, got %d", u.Line)
	}
	if u.Column != 1 {
		t.Errorf("expected Column == 1, got %d", u.Column)
	}
	// The whole line is kept, position and all. An unrecognized diagnostic is the
	// one case where this package has nothing to offer but the compiler's own
	// words, so it must not paraphrase them or trim anything off.
	if want := "x.go:1:1: gremlins ate the stack"; u.Text != want {
		t.Errorf("Text = %q, want the line verbatim: %q", u.Text, want)
	}
}

// TestParseWarnsOnAnUnvalidatedToolchain asserts that version warnings are
// added when the toolchain is outside the validated range.
func TestParseWarnsOnAnUnvalidatedToolchain(t *testing.T) {
	data, err := readFixture("escape/raw-go1.26.1.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Test with newer version.
	result, err := Parse(data, "go1.99.0")
	if err != nil {
		t.Fatalf("parse with newer version: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for newer toolchain version")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "go1.99.0") && strings.Contains(w, "go1.26.1") {
			found = true
			break
		}
	}
	if !found {
		t.Error("warning does not mention both versions")
	}

	// Test with validated version (go1.26.1) should have no version warning.
	result, err = Parse(data, "go1.26.1")
	if err != nil {
		t.Fatalf("parse with validated version: %v", err)
	}
	for _, w := range result.Warnings {
		if strings.Contains(w, "version") {
			t.Errorf("expected no version warning for go1.26.1, got: %s", w)
		}
	}

	// Both should still parse and produce findings.
	if len(result.Findings) == 0 {
		t.Error("expected findings to be parsed even with warning")
	}
}

// TestParseHandlesLinesWithNoPosition asserts that lines without a position
// prefix are added to Unrecognized with no position fields set.
func TestParseHandlesLinesWithNoPosition(t *testing.T) {
	input := []byte("this line has no position\n")
	result, err := Parse(input, "go1.26.1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Errorf("expected zero findings, got %d", len(result.Findings))
	}

	if len(result.Unrecognized) != 1 {
		t.Fatalf("expected 1 unrecognized line, got %d", len(result.Unrecognized))
	}

	u := result.Unrecognized[0]
	if u.File != "" {
		t.Errorf("expected File to be empty, got %q", u.File)
	}
	if u.Line != 0 {
		t.Errorf("expected Line to be 0, got %d", u.Line)
	}
	if u.Column != 0 {
		t.Errorf("expected Column to be 0, got %d", u.Column)
	}
	if u.Text != "this line has no position" {
		t.Errorf("expected Text to be %q, got %q", "this line has no position", u.Text)
	}
}

// Helper functions below.

func readFixture(name string) ([]byte, error) {
	path, err := fixture.Path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func normalizeResult(r *Result) {
	// Zero out machine-specific fields and order-dependent ones for comparison.
	// Keep findings in their original order (no sorting by parse.go).
	// Keep packages and files in order they appeared.

	// Sort packages and files for deterministic comparison.
	sort.Strings(r.Packages)
	sort.Strings(r.Files)
}

func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	dir, err := fixture.Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update to create it)", name, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s differs from the golden file; rerun with -update and review the diff", name)
	}
}

// TestParseCoversEveryCompilerTemplate is an inventory, not a sample.
//
// Every other test here runs on recorded output, which only proves the parser
// handles what the corpus happened to provoke. That is a weak guarantee: three
// templates are gated behind a debug flag, several need a construct nobody
// wrote, and one — the truncated-explanation warning — needs an assignment
// cycle. A real nine-package service produced 5868 diagnostic lines without
// triggering it.
//
// So this table is transcribed from the format strings in
// cmd/compile/internal/escape (escape.go, solve.go, graph.go, assign.go,
// alias.go) as of go1.26.1, with the %v placeholders filled in. It is the answer
// to "does the parser cover everything the compiler can say about escape", and
// it fails when a Go upgrade adds a sentence that nobody has taught it.
//
// Deliberately absent: the two base.ErrorfAt messages ("escapes to heap, not
// allowed in runtime" and "is incomplete (or unallocatable)"), which fail the
// build rather than reaching a report, and the stmt.go tracing, which needs
// -m=3 while this tool fixes -m=2.
func TestParseCoversEveryCompilerTemplate(t *testing.T) {
	const pos = "x.go:1:1: "
	for _, tc := range []struct {
		line string
		want schema.EscapeKind // "" means recognized but deliberately not a finding
	}{
		{"moved to heap: x", schema.EscapeMovedToHeap},
		{"&T{...} escapes to heap", schema.EscapeEscapesToHeap},
		{"append escapes to heap", schema.EscapeEscapesToHeap},
		{"x does not escape", schema.EscapeDoesNotEscape},
		{"append does not escape", schema.EscapeDoesNotEscape},
		{"zero-copy string->[]byte conversion", schema.EscapeZeroCopyConversion},
		{"assuming x is unsafe uintptr", schema.EscapeUnsafeUintptr},
		{"marking x as escaping uintptr", schema.EscapeUnsafeUintptr},
		{"marking x as escaping ...uintptr", schema.EscapeUnsafeUintptr},
		{"leaking param: p", schema.EscapeLeakingParam},
		{"leaking param content: p", schema.EscapeLeakingParamContent},
		{"leaking param: p to result ~r0 level=0", schema.EscapeLeakingParamResult},
		{"mutates param: p derefs=0", schema.EscapeMutatesParam},
		{"calls param: fn derefs=1", schema.EscapeCallsParam},
		{"p does not escape, mutate, or call", schema.EscapeParamInert},
		{"F capturing by ref: n (addr=false assign=true width=8)", schema.EscapeClosureCapture},
		{"F capturing by value: n (addr=false assign=false width=8)", schema.EscapeClosureCapture},
		{"F ignoring self-assignment in x = x", schema.EscapeSelfAssignment},

		// Recognized, and deliberately not reported: these are the compiler
		// commenting on work that is not escape analysis.
		{"rewriting OCONVIFACE value from x (int) to y (any)", ""},
		{"alias analysis: append using non-aliased slice: s in func F", ""},
		{"alias analysis: mismatch for *ir.Name: x: processed 1 times, observed 2 times", ""},
		{"can inline F with cost 5 as: func() {}", ""},
		{"cannot inline F: function too complex", ""},
		{"inlining call to F", ""},
		{"devirtualizing h.Write to *sha256.Digest", ""},
		{"index bounds check elided", ""},
		{"generated nil check", ""},
		{"expr will be kept alive", ""},
	} {
		t.Run(tc.line, func(t *testing.T) {
			r, err := Parse([]byte(pos+tc.line+"\n"), "go1.26.1")
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(r.Unrecognized) != 0 {
				t.Fatalf("template was not recognized: %q", r.Unrecognized[0].Text)
			}
			if tc.want == "" {
				if len(r.Findings) != 0 {
					t.Fatalf("expected no finding, got kind %q", r.Findings[0].Kind)
				}
				return
			}
			if len(r.Findings) != 1 {
				t.Fatalf("expected exactly one finding, got %d", len(r.Findings))
			}
			if got := r.Findings[0].Kind; got != tc.want {
				t.Errorf("kind = %q, want %q", got, tc.want)
			}
			if r.Findings[0].Evidence != pos+tc.line {
				t.Errorf("evidence = %q, want the line verbatim", r.Findings[0].Evidence)
			}
		})
	}
}

// TestParseHandlesATruncatedExplanation covers the one -m=2 line that neither
// the corpus nor a real nine-package service produced: the compiler gives up on
// a flow when it finds an assignment cycle. It is a continuation of the block,
// not a diagnostic, and the report has to say the flow is partial rather than
// let a consumer read it as complete.
func TestParseHandlesATruncatedExplanation(t *testing.T) {
	input := []byte(strings.Join([]string{
		"# example.com/p",
		"x.go:5:2: x escapes to heap in F:",
		"x.go:5:2:   flow: {heap} ← &x:",
		"x.go:5:2:     from &x (address-of) at x.go:6:9",
		"x.go:5:2:   warning: truncated explanation due to assignment cycle; see golang.org/issue/35518",
		"x.go:5:2: moved to heap: x",
	}, "\n") + "\n")

	r, err := Parse(input, "go1.26.1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(r.Unrecognized) != 0 {
		t.Fatalf("the truncation warning was not recognized: %q", r.Unrecognized[0].Text)
	}
	if len(r.Findings) != 1 || r.Findings[0].Kind != schema.EscapeMovedToHeap {
		t.Fatalf("findings = %+v, want one moved_to_heap", r.Findings)
	}
	// The warning is not a hop, so it must not appear as one.
	if n := len(r.Findings[0].Flow); n != 2 {
		t.Errorf("flow has %d steps, want 2 (the header and one hop)", n)
	}
	// But the fact that the flow is incomplete has to survive.
	var said bool
	for _, w := range r.Warnings {
		if strings.Contains(w, "truncated") {
			said = true
		}
	}
	if !said {
		t.Errorf("a truncated flow was reported as if it were complete; warnings = %q", r.Warnings)
	}
}
