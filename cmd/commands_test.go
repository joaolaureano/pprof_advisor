package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

func invoke(args ...string) (int, string, string) {
	root := newRootCmd()
	var out, diagnostics bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	root.SetArgs(args)
	code := execute(root)
	return code, out.String(), diagnostics.String()
}

func writeInput(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyCLIObjectiveAndExitContract(t *testing.T) {
	baseline := writeInput(t, "before.txt", strings.Repeat("BenchmarkWork-8 1 100 ns/op 100 B/op 10 allocs/op\n", 10))
	for _, tc := range []struct {
		name, after   string
		args          []string
		code          int
		verdict, unit string
	}{
		{"memory_default", "100 ns/op 50 B/op 20 allocs/op", []string{"--profile", "memory"}, 0, schema.VerdictImproved, "B/op"},
		{"cpu_protection", "200 ns/op 50 B/op 5 allocs/op", []string{"--unit", "B/op"}, 2, schema.VerdictRegressed, "B/op"},
		{"allocation_count", "100 ns/op 200 B/op 5 allocs/op", []string{"--unit", "allocs/op"}, 0, schema.VerdictImproved, "allocs/op"},
		{"cpu_default", "50 ns/op 200 B/op 20 allocs/op", nil, 0, schema.VerdictImproved, "ns/op"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := writeInput(t, "after.txt", strings.Repeat("BenchmarkWork-8 1 "+tc.after+"\n", 10))
			args := append([]string{"verify", "--baseline", baseline, "--after", after}, tc.args...)
			code, out, diagnostics := invoke(args...)
			if code != tc.code {
				t.Fatalf("code=%d diagnostics=%s", code, diagnostics)
			}
			var res schema.VerifyResult
			if err := json.Unmarshal([]byte(out), &res); err != nil {
				t.Fatalf("stdout is not one JSON document: %v\n%s", err, out)
			}
			if res.SchemaVersion != 3 || res.Verdict != tc.verdict || res.Measurement.Unit != tc.unit {
				t.Fatalf("result=%+v", res)
			}
			if code == 2 && !strings.Contains(diagnostics, "regressed") {
				t.Fatalf("missing regression diagnostic: %q", diagnostics)
			}
		})
	}
}

func TestCLIRejectsInvalidInputs(t *testing.T) {
	old := writeInput(t, "old.json", `{"schema_version":1,"diff":"nonempty"}`)
	for _, args := range [][]string{
		{"analyze", old}, {"apply", old},
		{"capture", "--pkg", ".", "--profile", "memory", "--unit", "ns/op"},
		{"run", "--pkg", ".", "--profile", "cpu", "--unit", "B/op"},
	} {
		code, _, diagnostics := invoke(args...)
		if code != 1 || diagnostics == "" {
			t.Fatalf("%v: code=%d diagnostics=%q", args, code, diagnostics)
		}
	}
}

func TestExtractCLIEmitsV3CPU(t *testing.T) {
	code, out, diagnostics := invoke("extract", filepath.Join("..", "testdata", "all.prof"))
	if code != 0 {
		t.Fatalf("exit=%d %s", code, diagnostics)
	}
	var res schema.ExtractResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if res.SchemaVersion != 3 || res.Profile.Measurement.Profile != measurement.CPU {
		t.Fatalf("result=%+v", res.Profile)
	}
	for _, legacy := range []string{"flat_nanos", "line_nanos", "total_nanos", "analyzed_nanos"} {
		if strings.Contains(out, `"`+legacy+`"`) {
			t.Fatalf("legacy field %s emitted", legacy)
		}
	}
}

func TestEscapeCLI(t *testing.T) {
	corpusPath := filepath.Join("..", "testdata", "escape", "corpus")
	code, out, diagnostics := invoke("escape", "--dir", corpusPath)
	if code != 0 {
		t.Fatalf("exit=%d %s", code, diagnostics)
	}
	var res schema.EscapeReport
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if res.SchemaVersion != 1 {
		t.Errorf("SchemaVersion: got %d, want 1", res.SchemaVersion)
	}
	if res.Toolchain.Version == "" {
		t.Error("Toolchain.Version is empty")
	}
	if len(res.Findings) == 0 {
		t.Fatal("no findings")
	}

	// Boundary assertion: compiler phrases must appear only in Evidence.
	compilerPhrases := []string{
		"escapes to heap",
		"does not escape",
		"moved to heap",
		"leaking param",
	}
	for _, finding := range res.Findings {
		for _, phrase := range compilerPhrases {
			if strings.Contains(string(finding.Kind), phrase) {
				t.Errorf("Found %q in Kind: %s", phrase, finding.Kind)
			}
			if strings.Contains(finding.Subject, phrase) {
				t.Errorf("Found %q in Subject: %s", phrase, finding.Subject)
			}
			if strings.Contains(finding.Function, phrase) {
				t.Errorf("Found %q in Function: %s", phrase, finding.Function)
			}
			if strings.Contains(finding.Target, phrase) {
				t.Errorf("Found %q in Target: %s", phrase, finding.Target)
			}
		}
	}
}
