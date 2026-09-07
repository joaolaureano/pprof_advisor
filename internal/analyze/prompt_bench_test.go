package analyze

import (
	"testing"

	"github.com/joaolaureano/profadvisor/internal/extract"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/prompt"
)

// BenchmarkBuildUserPrompt measures the performance of rendering the user-facing
// prompt from an extract result.
//
// This benchmark exists as a control. The function calls Render dozens of times
// per prompt (once per hotspot and summary field), and Render compiles a
// regular expression on every call. Therefore the profile is expected to show
// regexp compilation as the dominant cost. This benchmark confirms the tool
// correctly identifies that bottleneck. The fix (caching compiled regexes) is
// reserved for a later round; this round measures only.
//
// Setup mirrors prompt_golden_test.go: load the catalog, fixture profile, and
// extract result once outside the loop.
func BenchmarkBuildUserPrompt(b *testing.B) {
	// Load catalog, profile, and extract result once.
	cat, err := prompt.Load()
	if err != nil {
		b.Fatalf("prompt catalog: %v", err)
	}
	p, err := fixture.Load("all.prof")
	if err != nil {
		b.Fatalf("fixture profile: %v", err)
	}
	r, err := extract.FromProfile(p, "all.prof", extract.Options{TopN: 3})
	if err != nil {
		b.Fatalf("extract: %v", err)
	}

	// Set measurement to CPU ns/op for consistency.
	cfg, err := measurement.Resolve(measurement.CPU, "ns/op")
	if err != nil {
		b.Fatalf("resolve measurement: %v", err)
	}
	r.Profile.Measurement = cfg

	b.ReportAllocs()

	// Benchmark buildUserPrompt.
	for b.Loop() {
		_, err := buildUserPrompt(cat, r, "example.com/profadvisor/fixture")
		if err != nil {
			b.Fatalf("build user prompt: %v", err)
		}
	}
}

// BenchmarkSystemPrompt measures the performance of rendering the system prompt
// for a given measurement configuration.
//
// This is a cheaper operation than buildUserPrompt: it calls Render a small
// constant number of times rather than dozens. It serves as a baseline to
// verify that the regexp compilation cost is specific to buildUserPrompt.
func BenchmarkSystemPrompt(b *testing.B) {
	// Load catalog once.
	cat, err := prompt.Load()
	if err != nil {
		b.Fatalf("prompt catalog: %v", err)
	}

	// Resolve CPU measurement configuration.
	cfg, err := measurement.Resolve(measurement.CPU, "ns/op")
	if err != nil {
		b.Fatalf("resolve measurement: %v", err)
	}

	b.ReportAllocs()

	// Benchmark systemPrompt.
	for b.Loop() {
		_, err := systemPrompt(cat, cfg)
		if err != nil {
			b.Fatalf("system prompt: %v", err)
		}
	}
}
