package analyze

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/prompt"
)

var update = flag.Bool("update", false, "rewrite the prompt golden files")

// TestPromptsMatchGolden pins the exact bytes the model is sent.
//
// The wording lives in a JSON catalog now, which makes it easy to change and
// therefore easy to change by accident: a stray space in a template is
// invisible in review and silently alters every request the tool makes. These
// goldens are the record of what was actually intended. Regenerate them with
// `go test ./internal/analyze -update` and read the diff — a change here is a
// change to the product, not a test fixture to be refreshed on autopilot.
func TestPromptsMatchGolden(t *testing.T) {
	cat, err := prompt.Load()
	if err != nil {
		t.Fatal(err)
	}
	p, err := fixture.Load("all.prof")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	r, err := extract.FromProfile(p, "all.prof", extract.Options{TopN: 3})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	cases := []struct {
		name string
		kind measurement.Kind
		unit string
	}{
		{"cpu", measurement.CPU, "ns/op"},
		{"mem_bytes", measurement.Memory, "B/op"},
		{"mem_allocs", measurement.Memory, "allocs/op"},
		{"block", measurement.Block, "ns/op"},
		{"mutex", measurement.Mutex, "ns/op"},
	}
	for _, tc := range cases {
		cfg, err := measurement.Resolve(tc.kind, tc.unit)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got, err := systemPrompt(cat, cfg)
		if err != nil {
			t.Fatalf("%s: system prompt: %v", tc.name, err)
		}
		compareGolden(t, "system_"+tc.name+".txt", got)

		// The same evidence relabelled, so the memory cases also exercise the
		// attribution note and the bytes/allocs cost renderers.
		r.Profile.Measurement = cfg
		got, err = buildUserPrompt(cat, r, "example.com/profadvisor/fixture")
		if err != nil {
			t.Fatalf("%s: user prompt: %v", tc.name, err)
		}
		compareGolden(t, "user_"+tc.name+".txt", got)
	}

	// Without a module the prompt must simply omit that line, not print an
	// empty one.
	r.Profile.Measurement, _ = measurement.Resolve(measurement.CPU, "ns/op")
	got, err := buildUserPrompt(cat, r, "")
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "user_cpu_nomodule.txt", got)
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	dir, err := fixture.Dir()
	if err != nil {
		t.Fatal(err)
	}
	// The recorded prompt names the source file by absolute path, which is
	// correct at runtime and wrong in a golden file: it would pin these tests
	// to one checkout at one path, which is the very thing internal/fixture
	// re-anchors profiles to avoid. The checkout root is replaced by a
	// placeholder so the goldens travel with the repository.
	got = strings.ReplaceAll(got, dir, "<testdata>")
	path := filepath.Join(dir, "prompts", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update to create it)", name, err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the golden file; rerun with -update and review the diff", name)
	}
}
