package extract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/joaolaureano/profadvisor/internal/fixture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
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
	if result.Profile.Analyzed >= result.Profile.Total/2 {
		t.Fatalf("analyzed=%d total=%d: runtime noise was not substantially filtered", result.Profile.Analyzed, result.Profile.Total)
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
		// CumPct is normalized by Total while FlatPct is normalized by
		// Analyzed, so the percentages are not directly comparable after
		// filtering. The corresponding raw relationship is always required.
		if hotspot.Cum < hotspot.Flat {
			t.Errorf("cum %d < flat %d for %q", hotspot.Cum, hotspot.Flat, hotspot.Function)
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

// TestWrongProfileKindIsRejected covers the mistake this tool makes easy: a CPU
// objective pointed at a memory profile. Silently finding no "cpu" samples and
// reporting zero hotspots would look like a benchmark with nothing to optimize.
func TestWrongProfileKindIsRejected(t *testing.T) {
	p := &profile.Profile{SampleType: []*profile.ValueType{{Type: "alloc_space", Unit: "bytes"}}}
	_, err := FromProfile(p, "mem.prof", Options{})
	if err == nil || !strings.Contains(err.Error(), "no cpu sample type") {
		t.Fatalf("error = %v, want no cpu sample type", err)
	}
	cfg, err := measurement.Resolve(measurement.Memory, "B/op")
	if err != nil {
		t.Fatal(err)
	}
	cpu := &profile.Profile{SampleType: []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}}}
	if _, err := FromProfile(cpu, "cpu.prof", Options{Measurement: cfg}); err == nil ||
		!strings.Contains(err.Error(), "no alloc_space sample type") {
		t.Fatalf("error = %v, want no alloc_space sample type", err)
	}
}

// TestBlockProfileChargesTheCodeUnderTest verifies that block contention profiles
// are read correctly and hotspots are ranked by the code under test, not by
// runtime primitives. This is critical: a contention sample's leaf frame is
// always in runtime or sync, never the code that caused the contention. The
// first_focus_frame attribution redirects cost to the innermost frame inside the
// focus package, making the ranking useful. Without this, all top hotspots would
// be sync.(*Mutex).Lock, which is not code the user can edit.
func TestBlockProfileChargesTheCodeUnderTest(t *testing.T) {
	cfg, err := measurement.Resolve(measurement.Block, "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	result := load(t, "block.prof", Options{Measurement: cfg, TopN: 5})
	if len(result.Hotspots) == 0 {
		t.Fatal("no hotspots returned")
	}

	// Verify that at least one hotspot is from the code under test.
	found := false
	for _, hotspot := range result.Hotspots {
		if strings.HasPrefix(hotspot.Function, "example.com/contend.") {
			found = true
			break
		}
	}
	if !found {
		var names []string
		for _, hotspot := range result.Hotspots {
			names = append(names, hotspot.Function)
		}
		t.Fatalf("no hotspots from example.com/contend in ranking: %v", names)
	}

	// Verify that no hotspot is a runtime or sync frame. These are the leaf
	// frames of contention samples and should never appear in the ranking.
	for _, hotspot := range result.Hotspots {
		if strings.HasPrefix(hotspot.Function, "runtime.") {
			t.Errorf("runtime hotspot should not be ranked: %q", hotspot.Function)
		}
		if strings.HasPrefix(hotspot.Function, "sync.") {
			t.Errorf("sync hotspot should not be ranked: %q", hotspot.Function)
		}
	}

	// Verify that no hotspot is a benchmark harness function, which the
	// harnessFunction filter should have removed.
	for _, hotspot := range result.Hotspots {
		if strings.Contains(hotspot.Function, ".Benchmark") {
			t.Errorf("benchmark harness should not be ranked: %q", hotspot.Function)
		}
	}

	// Verify the sample type and unit are correctly set to delay/nanoseconds.
	if result.Profile.Measurement.SampleType != "delay" {
		t.Errorf("sample type = %q, want delay", result.Profile.Measurement.SampleType)
	}
	if result.Profile.Measurement.SampleUnit != "nanoseconds" {
		t.Errorf("sample unit = %q, want nanoseconds", result.Profile.Measurement.SampleUnit)
	}

	// Verify that total cost is non-zero, confirming the profile was actually read.
	if result.Profile.Total <= 0 {
		t.Errorf("total cost = %d, want > 0", result.Profile.Total)
	}

	// The assertions above would all hold even under "self" attribution, since
	// noiseFunction drops the runtime and sync leaves from the ranking either
	// way. What separates the two is where the cost lands: charged to the leaf,
	// every nanosecond would be filtered out with it and the surviving frames
	// would rank at zero. A non-zero cost on the top frame is therefore the
	// assertion that actually pins first_focus_frame.
	if result.Profile.Analyzed <= 0 {
		t.Errorf("attributed cost = %d, want > 0: the blocked time was charged to frames that were then filtered out",
			result.Profile.Analyzed)
	}
	if top := result.Hotspots[0]; top.Flat <= 0 {
		t.Errorf("top hotspot %s has flat = %d, want > 0", top.Function, top.Flat)
	}
}

// TestBlockProfileReportsTheFilteredPrimitives verifies that contention profiles
// report which blocking primitives caused the cost, even though they are not
// ranked as hotspots. The excluded list lets the reader see where the cost went,
// which for a contention profile is the entire mechanism: sync.(*Mutex).Lock,
// runtime.chanrecv, runtime.chansend, etc. Without this visibility, a reader
// cannot understand what they are optimizing for.
func TestBlockProfileReportsTheFilteredPrimitives(t *testing.T) {
	cfg, err := measurement.Resolve(measurement.Block, "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	result := load(t, "block.prof", Options{Measurement: cfg, TopN: 5})
	if len(result.Profile.Excluded) == 0 {
		t.Fatal("excluded list is empty, expected blocking primitives")
	}

	// Verify that at least one excluded frame is from the blocking primitives.
	found := false
	var names []string
	for _, frame := range result.Profile.Excluded {
		names = append(names, frame.Function)
		if strings.HasPrefix(frame.Function, "sync.") || strings.HasPrefix(frame.Function, "runtime.") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no sync or runtime frames in excluded list: %v", names)
	}
}

// TestBlockProfileRejectsTheWrongMeasurement verifies that attempting to read a
// block profile with a CPU measurement config fails with a clear error. This
// catches the user mistake of pointing --profile=block at a CPU benchmark or
// vice versa. A silent read of the wrong column is worse than a loud failure.
func TestBlockProfileRejectsTheWrongMeasurement(t *testing.T) {
	// Load block.prof with CPU measurement config; should fail because the
	// profile does not have a "cpu" sample type.
	cpuCfg, err := measurement.Resolve(measurement.CPU, "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	blockProf, err := fixture.Load("block.prof")
	if err != nil {
		t.Fatal(err)
	}
	_, err = FromProfile(blockProf, "block.prof", Options{Measurement: cpuCfg})
	if err == nil {
		t.Fatal("expected error when reading block.prof with CPU config, got nil")
	}
	if !strings.Contains(err.Error(), "cpu") {
		t.Errorf("error message should name the missing sample type 'cpu': %v", err)
	}

	// Load all.prof (CPU profile) with block measurement config; should fail
	// because the profile does not have a "delay" sample type.
	blockCfg, err := measurement.Resolve(measurement.Block, "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	cpuProf, err := fixture.Load(allProfile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = FromProfile(cpuProf, allProfile, Options{Measurement: blockCfg})
	if err == nil {
		t.Fatal("expected error when reading all.prof with block config, got nil")
	}
	if !strings.Contains(err.Error(), "delay") {
		t.Errorf("error message should name the missing sample type 'delay': %v", err)
	}
}
