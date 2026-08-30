package extract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

const (
	allProfile    = "all.prof"
	paramProfile  = "param.prof"
	fixtureModule = "example.com/profadvisor/fixture"
)

// load reads a recorded profile from testdata and ranks it. The profiles come
// from the benchmarks in testdata/fixture, so every expectation below is about
// code that ships with this repository.
func load(t *testing.T, name string, opts Options) *schema.ExtractResult {
	t.Helper()
	p, err := fixture.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	result, err := FromProfile(p, name, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAllProfRanksFixtureCode(t *testing.T) {
	result := load(t, allProfile, Options{})
	if len(result.Hotspots) == 0 {
		t.Fatal("no hotspots")
	}
	for _, hotspot := range result.Hotspots {
		if !strings.HasPrefix(hotspot.Function, fixtureModule) {
			t.Errorf("hotspot outside inferred focus: %q", hotspot.Function)
		}
		if strings.HasPrefix(hotspot.Function, "runtime.") {
			t.Errorf("runtime hotspot was not filtered: %q", hotspot.Function)
		}
	}
	if !strings.Contains(result.Hotspots[0].Function, "matcher.") {
		t.Errorf("top hotspot %q is not the code under test", result.Hotspots[0].Function)
	}
}

func TestFilteringChangesTheAnswer(t *testing.T) {
	result := load(t, paramProfile, Options{})
	if result.Profile.AnalyzedNanos >= result.Profile.TotalNanos/2 {
		t.Fatalf("analyzed=%d total=%d: runtime noise was not substantially filtered", result.Profile.AnalyzedNanos, result.Profile.TotalNanos)
	}
	for _, hotspot := range result.Hotspots {
		if strings.HasPrefix(hotspot.Function, "runtime.") {
			t.Errorf("runtime hotspot was not filtered: %q", hotspot.Function)
		}
	}
}

func TestTopNAndDeterminism(t *testing.T) {
	first := load(t, allProfile, Options{TopN: 3})
	second := load(t, allProfile, Options{TopN: 3})
	if len(first.Hotspots) != 3 {
		t.Fatalf("got %d hotspots, want 3", len(first.Hotspots))
	}
	if !reflect.DeepEqual(first.Hotspots, second.Hotspots) {
		t.Fatal("two identical calls returned different hotspots")
	}
}

func TestSourceExcerptAbsentFileIsNotAnError(t *testing.T) {
	p, err := fixture.Load(allProfile)
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range p.Function {
		fn.Filename = "/does/not/exist/source.go"
	}
	result, err := FromProfile(p, allProfile, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hotspot := range result.Hotspots {
		if hotspot.Source != nil {
			t.Errorf("source for %q = %#v, want nil", hotspot.Function, hotspot.Source)
		}
	}
}

func TestPercentagesAreSane(t *testing.T) {
	result := load(t, allProfile, Options{TopN: 1000})
	var flatTotal float64
	for _, hotspot := range result.Hotspots {
		if hotspot.FlatPct < 0 || hotspot.FlatPct > 100 {
			t.Errorf("flat percentage for %q = %v", hotspot.Function, hotspot.FlatPct)
		}
		// CumPct is normalized by TotalNanos while FlatPct is normalized by
		// AnalyzedNanos, so the percentages are not directly comparable after
		// filtering. The corresponding raw relationship is always required.
		if hotspot.CumNanos < hotspot.FlatNanos {
			t.Errorf("cum nanos %d < flat nanos %d for %q", hotspot.CumNanos, hotspot.FlatNanos, hotspot.Function)
		}
		flatTotal += hotspot.FlatPct
	}
	if flatTotal > 100.0001 {
		t.Errorf("flat percentage total = %v", flatTotal)
	}
}

func TestSourceIsAttachedFromTheFixtureTree(t *testing.T) {
	result := load(t, allProfile, Options{TopN: 1})
	src := result.Hotspots[0].Source
	if src == nil || len(src.Lines) == 0 {
		t.Fatalf("no source excerpt for %q (file %q)", result.Hotspots[0].Function, result.Hotspots[0].File)
	}
}

func TestNoCPUSampleType(t *testing.T) {
	p := &profile.Profile{SampleType: []*profile.ValueType{{Type: "alloc_space", Unit: "bytes"}}}
	_, err := FromProfile(p, "memory.prof", Options{})
	if err == nil || !strings.Contains(err.Error(), "no cpu sample type") {
		t.Fatalf("error = %v, want no cpu sample type", err)
	}
}
