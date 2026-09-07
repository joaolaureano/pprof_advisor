// Package extract turns pprof profiles into a compact, source-oriented hotspot
// list.
//
// It reads CPU and memory profiles through the same code path. The only things
// that differ are which sample type is summed and which frame a sample is
// charged to, and both come from the measurement.Config it is given rather than
// from a branch in here.
package extract

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Options controls hotspot selection and source context.
type Options struct {
	// Measurement says which sample type to read and how to attribute it.
	// The zero value means CPU nanoseconds.
	Measurement measurement.Config
	// TopN is how many hotspots to return after filtering. Zero means 10.
	TopN int
	// FocusPrefixes limits hotspots to functions whose name starts with one
	// of these prefixes. Empty means "infer".
	FocusPrefixes []string
	// ContextLines is how many source lines to include on each side of the
	// hotspot's hottest line. Zero means 8.
	ContextLines int
}

type functionStats struct {
	function *profile.Function
	// flat is the cost charged to this function under the configured
	// attribution; selfFlat is always the leaf-frame cost. They are the same
	// number for CPU, and differ for memory, where the leaf is the allocator.
	// selfFlat is never reported: it exists because focus inference has to
	// run before attribution can, and needs a cost to rank modules by.
	flat     int64
	selfFlat int64
	cum      int64
	// fromFocus is cost on paths that pass through the focus package: the
	// value of every sample in which this function was called, directly or
	// not, by the code under test. It exists because cum answers the wrong
	// question for a filtered frame. On a short benchmark, idle netpoller
	// threads put runtime.kevent at 40% of cum while having nothing to do with
	// the code under test, which pushes the frames that actually explain the
	// cost off the list this package reports.
	fromFocus int64
	minLine   int
	lineCosts map[int]int64
}

// FromFile reads a pprof profile from path and ranks its hotspots.
func FromFile(path string, opts Options) (*schema.ExtractResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p, err := profile.Parse(f)
	if err != nil {
		return nil, err
	}
	return FromProfile(p, path, opts)
}

// FromProfile ranks an already-parsed profile. path is recorded for provenance.
func FromProfile(p *profile.Profile, path string, opts Options) (*schema.ExtractResult, error) {
	if p == nil {
		return nil, fmt.Errorf("nil profile")
	}
	cfg := opts.Measurement
	if (cfg == measurement.Config{}) {
		resolved, err := measurement.Resolve("", "")
		if err != nil {
			return nil, err
		}
		cfg = resolved
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	valueIndex := sampleIndex(p, cfg.SampleType)
	if valueIndex < 0 {
		return nil, fmt.Errorf("no %s sample type in %s (has %s)",
			cfg.SampleType, path, sampleTypes(p))
	}

	stats := make(map[string]*functionStats)
	getStats := func(fn *profile.Function) *functionStats {
		if fn == nil {
			return nil
		}
		if s := stats[fn.Name]; s != nil {
			return s
		}
		s := &functionStats{function: fn, lineCosts: make(map[int]int64)}
		stats[fn.Name] = s
		return s
	}
	for _, loc := range p.Location {
		if loc == nil {
			continue
		}
		for _, line := range loc.Line {
			if s := getStats(line.Function); s != nil && line.Line > 0 && (s.minLine == 0 || int(line.Line) < s.minLine) {
				s.minLine = int(line.Line)
			}
		}
	}

	// First pass: cumulative cost, and the leaf-frame cost that focus
	// inference ranks modules by. Attribution cannot run yet, because under
	// first_focus_frame it needs the focus this pass is about to produce.
	var total int64
	for _, sample := range p.Sample {
		if sample == nil || valueIndex >= len(sample.Value) {
			continue
		}
		value := sample.Value[valueIndex]
		total += value

		if leaf := leafLine(sample); leaf.Function != nil {
			getStats(leaf.Function).selfFlat += value
		}

		seen := make(map[string]*profile.Function)
		for _, loc := range sample.Location {
			if loc == nil {
				continue
			}
			for _, line := range loc.Line {
				if line.Function != nil {
					seen[line.Function.Name] = line.Function
				}
			}
		}
		for _, fn := range seen {
			getStats(fn).cum += value
		}
	}

	survivors := make([]*functionStats, 0, len(stats))
	var excluded []*functionStats
	for _, s := range stats {
		if noiseFunction(s.function.Name) {
			excluded = append(excluded, s)
			continue
		}
		survivors = append(survivors, s)
	}
	focus := append([]string(nil), opts.FocusPrefixes...)
	if len(focus) == 0 {
		focus = inferFocus(survivors)
	}

	// Now that focus is known, charge the filtered frames that the code under
	// test actually reached. See functionStats.fromFocus.
	attributeFromFocus(p, valueIndex, focus, getStats)

	// Second pass: charge each sample to a frame, now that focus is known.
	attribute(p, valueIndex, cfg, focus, getStats)
	if len(focus) > 0 {
		// Everything outside the focus package leaves the ranking, but it does
		// not leave the report: these are the frames the code under test
		// called, and they are frequently the whole explanation. A prompt
		// builder that spends half its time in regexp.Compile looks like a
		// slow prompt builder until this list says otherwise. Dropping them
		// silently, as this did, is what made the excluded list unable to
		// answer the question it exists for.
		filtered := survivors[:0]
		for _, s := range survivors {
			if matchesAnyPrefix(s.function.Name, focus) {
				filtered = append(filtered, s)
				continue
			}
			if !harnessFunction(s.function.Name) {
				excluded = append(excluded, s)
			}
		}
		survivors = filtered
	}

	// Only now, after focus inference has used the Benchmark frame to identify
	// the package under test, is the harness itself dropped.
	ranked := survivors[:0]
	for _, s := range survivors {
		if !harnessFunction(s.function.Name) {
			ranked = append(ranked, s)
		}
	}
	survivors = ranked

	var analyzed int64
	for _, s := range survivors {
		analyzed += s.flat
	}
	sort.Slice(survivors, func(i, j int) bool {
		if survivors[i].flat != survivors[j].flat {
			return survivors[i].flat > survivors[j].flat
		}
		return survivors[i].function.Name < survivors[j].function.Name
	})
	topN := opts.TopN
	if topN == 0 {
		topN = 10
	}
	if topN < 0 {
		topN = 0
	}
	if topN < len(survivors) {
		survivors = survivors[:topN]
	}

	context := opts.ContextLines
	if context == 0 {
		context = 8
	}
	result := &schema.ExtractResult{
		SchemaVersion: schema.Version,
		Profile: schema.ProfileMeta{
			Path: path, Measurement: cfg, Total: total, Analyzed: analyzed,
			DurationNanos: p.DurationNanos, FocusPrefixes: focus,
			Excluded: topExcluded(excluded, total, 5),
		},
		Hotspots: make([]schema.Hotspot, 0, len(survivors)),
	}
	for _, s := range survivors {
		line := int(s.function.StartLine)
		if line == 0 {
			line = s.minLine
		}
		h := schema.Hotspot{
			Function: s.function.Name, File: s.function.Filename, Line: line,
			Flat: s.flat, Cum: s.cum,
		}
		if analyzed != 0 {
			h.FlatPct = float64(s.flat) / float64(analyzed) * 100
		}
		if total != 0 {
			h.CumPct = float64(s.cum) / float64(total) * 100
		}
		h.Source = sourceExcerpt(h.File, h.Line, context, s.lineCosts)
		result.Hotspots = append(result.Hotspots, h)
	}
	return result, nil
}

func sampleIndex(p *profile.Profile, sampleType string) int {
	for i, t := range p.SampleType {
		if t != nil && t.Type == sampleType {
			return i
		}
	}
	return -1
}

func sampleTypes(p *profile.Profile) string {
	names := make([]string, 0, len(p.SampleType))
	for _, t := range p.SampleType {
		if t != nil {
			names = append(names, t.Type)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// leafLine returns the innermost frame of a sample. pprof stores a stack from
// leaf to root, in the locations and in the lines within a location, so the
// first function encountered is the one that was executing.
func leafLine(sample *profile.Sample) profile.Line {
	for _, loc := range sample.Location {
		if loc == nil {
			continue
		}
		for _, line := range loc.Line {
			if line.Function != nil {
				return line
			}
		}
	}
	return profile.Line{}
}

// attribute charges every sample's cost to one frame, and to one source line
// within it, according to the configured attribution rule.
//
// The rule exists because "which function is responsible for this cost" has a
// different answer per profile kind. In a CPU profile the leaf frame is the
// code that was running, and that is the answer. In a memory profile the leaf
// is runtime.mallocgc under every single sample; charging it there produces one
// enormous hotspot in the runtime and no information at all. What the reader
// needs is the innermost frame belonging to the code under test — the call that
// asked for the memory, which is the line a patch can change.
func attribute(p *profile.Profile, valueIndex int, cfg measurement.Config, focus []string, getStats func(*profile.Function) *functionStats) {
	firstFocusFrame := cfg.Attribution == "first_focus_frame" && len(focus) > 0
	for _, sample := range p.Sample {
		if sample == nil || valueIndex >= len(sample.Value) {
			continue
		}
		value := sample.Value[valueIndex]
		target := leafLine(sample)
		if firstFocusFrame {
			target = profile.Line{}
			for _, loc := range sample.Location {
				if loc == nil {
					continue
				}
				for _, line := range loc.Line {
					fn := line.Function
					if fn == nil || noiseFunction(fn.Name) || !matchesAnyPrefix(fn.Name, focus) {
						continue
					}
					target = line
					break
				}
				if target.Function != nil {
					break
				}
			}
		}
		// No in-focus frame: the cost is real and stays in Total, but there
		// is nothing in the code under test to charge it to.
		if target.Function == nil {
			continue
		}
		s := getStats(target.Function)
		s.flat += value
		if target.Line > 0 {
			s.lineCosts[int(target.Line)] += value
		}
	}
}

// harnessFunction reports whether name is the measuring instrument rather than
// the thing being measured. These are ranked out because the model is told not
// to touch _test.go: a benchmark at the top of the list is a suggestion the
// pipeline would have to refuse.
func harnessFunction(name string) bool {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return false
	}
	switch leaf := name[dot+1:]; {
	case strings.HasPrefix(leaf, "Benchmark"),
		strings.HasPrefix(leaf, "Test"),
		strings.HasPrefix(leaf, "Fuzz"):
		return true
	}
	return false
}

func noiseFunction(name string) bool {
	for _, prefix := range []string{"runtime.", "internal/", "syscall.", "sync.", "os.", "reflect."} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	if name == "cmpbody" || name == "memeqbody" || name == "indexbytebody" {
		return true
	}
	return !strings.Contains(name, ".")
}

func matchesAnyPrefix(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// stdlibRoots are the first path segments of the Go standard library. They are
// needed because "busiest module" is not a safe way to find the code under
// test: a per-pixel loop can spend more time inside image/color than in the
// caller that runs it, and focusing on image/color would hide the function the
// user can actually change.
var stdlibRoots = map[string]bool{
	"archive": true, "bufio": true, "bytes": true, "cmp": true, "compress": true,
	"container": true, "context": true, "crypto": true, "database": true,
	"debug": true, "embed": true, "encoding": true, "errors": true, "expvar": true,
	"flag": true, "fmt": true, "go": true, "hash": true, "html": true, "image": true,
	"index": true, "io": true, "iter": true, "log": true, "maps": true, "math": true,
	"mime": true, "net": true, "os": true, "path": true, "plugin": true,
	"reflect": true, "regexp": true, "runtime": true, "slices": true, "sort": true,
	"strconv": true, "strings": true, "structs": true, "sync": true, "syscall": true,
	"testing": true, "text": true, "time": true, "unicode": true, "unique": true,
	"unsafe": true, "weak": true,
}

func isStdlib(module string) bool {
	root, _, _ := strings.Cut(module, "/")
	// A domain in the first segment means a real module path, never stdlib.
	if strings.Contains(root, ".") {
		return false
	}
	return stdlibRoots[root]
}

// benchmarkModule finds the module that defines the benchmark being profiled.
//
// This is the reliable signal, and it is available because of what this tool
// is: profadvisor only ever profiles `go test -bench`, so a function named
// Benchmark* in the profile is by definition in the package under test. Picking
// the module by flat time instead gets this wrong whenever the callee is
// hotter than the caller, which for tight loops over standard-library helpers
// is the normal case rather than the exception.
func benchmarkModule(survivors []*functionStats) string {
	best := ""
	for _, s := range survivors {
		name := s.function.Name
		dot := strings.LastIndex(name, ".")
		if dot < 0 || !strings.HasPrefix(name[dot+1:], "Benchmark") {
			continue
		}
		pkg := packagePath(name)
		if pkg == "" {
			continue
		}
		if module := modulePrefix(pkg); best == "" || module < best {
			best = module
		}
	}
	return best
}

func inferFocus(survivors []*functionStats) []string {
	if module := benchmarkModule(survivors); module != "" {
		return []string{module}
	}
	// No benchmark frame in the profile — fall back to the busiest module,
	// skipping the standard library so the answer is still something the
	// caller owns and can edit.
	flatByModule := make(map[string]int64)
	for _, s := range survivors {
		pkg := packagePath(s.function.Name)
		if pkg == "" {
			continue
		}
		if module := modulePrefix(pkg); !isStdlib(module) {
			flatByModule[module] += s.selfFlat
		}
	}
	best := ""
	var bestFlat int64
	for module, flat := range flatByModule {
		if best == "" || flat > bestFlat || (flat == bestFlat && module < best) {
			best, bestFlat = module, flat
		}
	}
	if best == "" {
		return nil
	}
	return []string{best}
}

func packagePath(name string) string {
	slash := strings.LastIndex(name, "/")
	start := slash + 1
	dot := strings.Index(name[start:], ".")
	if dot < 0 {
		return ""
	}
	return name[:start+dot]
}

func modulePrefix(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, "/")
}

func sourceExcerpt(filename string, fallbackLine, context int, lineCosts map[int]int64) *schema.SourceExcerpt {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(contents), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	hottest, hottestValue := fallbackLine, int64(0)
	for line, value := range lineCosts {
		if value > hottestValue || (value == hottestValue && (hottest == 0 || line < hottest)) {
			hottest, hottestValue = line, value
		}
	}
	if hottest <= 0 || len(lines) == 0 {
		return &schema.SourceExcerpt{File: filename}
	}
	start, end := hottest-context, hottest+context
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return &schema.SourceExcerpt{File: filename}
	}
	excerptLines := make([]string, end-start+1)
	for i := start; i <= end; i++ {
		excerptLines[i-start] = strings.TrimRight(lines[i-1], "\r")
	}
	excerptCosts := make(map[int]int64)
	for line, value := range lineCosts {
		if line >= start && line <= end && value != 0 {
			excerptCosts[line] = value
		}
	}
	return &schema.SourceExcerpt{File: filename, StartLine: start, EndLine: end, Lines: excerptLines, LineCosts: excerptCosts}
}

// attributeFromFocus charges each sample to the frames the focus package called.
//
// pprof orders a sample's locations leaf first, so the frames from the leaf up
// to the first focus frame are exactly what the code under test called. A
// sample with no focus frame at all is discarded: it is work this program did
// not ask for — an idle netpoller thread, a background GC worker — and counting
// it is what made the filtered-cost report unreadable.
func attributeFromFocus(p *profile.Profile, valueIndex int, focus []string, getStats func(*profile.Function) *functionStats) {
	if len(focus) == 0 {
		return
	}
	seen := make(map[string]*profile.Function)
	for _, sample := range p.Sample {
		if sample == nil || valueIndex >= len(sample.Value) {
			continue
		}
		clear(seen)
		hitFocus := false
	frames:
		for _, loc := range sample.Location {
			if loc == nil {
				continue
			}
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				if matchesAnyPrefix(line.Function.Name, focus) {
					hitFocus = true
					break frames
				}
				seen[line.Function.Name] = line.Function
			}
		}
		if !hitFocus {
			continue
		}
		for _, fn := range seen {
			getStats(fn).fromFocus += sample.Value[valueIndex]
		}
	}
}

// topExcluded reports the hottest filtered-out functions, so the caller can see
// where the discarded time went instead of only how much was discarded.
func topExcluded(excluded []*functionStats, total int64, n int) []schema.ExcludedCost {
	// Sorted by cumulative, not self, time. Self time finds the busiest leaf,
	// which for runtime frames is usually the allocator or the scheduler;
	// cumulative time finds the one that explains the shape of the cost, such
	// as runtime.convTnoptr sitting under every iteration of a loop that boxes
	// a value into an interface.
	// Ranked by what the code under test reached, not by global cumulative:
	// the point of this list is to explain the cost that filtering removed,
	// and a frame no focus code ever called explains nothing.
	sort.Slice(excluded, func(i, j int) bool {
		if excluded[i].fromFocus != excluded[j].fromFocus {
			return excluded[i].fromFocus > excluded[j].fromFocus
		}
		if excluded[i].cum != excluded[j].cum {
			return excluded[i].cum > excluded[j].cum
		}
		return excluded[i].function.Name < excluded[j].function.Name
	})
	if n < len(excluded) {
		excluded = excluded[:n]
	}
	out := make([]schema.ExcludedCost, 0, len(excluded))
	for _, s := range excluded {
		if s.cum == 0 {
			continue
		}
		c := schema.ExcludedCost{
			Function: s.function.Name, Flat: s.flat, Cum: s.cum,
			FromFocus: s.fromFocus,
		}
		if total != 0 {
			c.FlatPct = float64(s.flat) / float64(total) * 100
			c.CumPct = float64(s.cum) / float64(total) * 100
			c.FromFocusPct = float64(s.fromFocus) / float64(total) * 100
		}
		out = append(out, c)
	}
	return out
}
