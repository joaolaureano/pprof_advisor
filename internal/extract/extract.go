// Package extract turns CPU profiles into a compact, source-oriented hotspot list.
package extract

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Options controls hotspot selection and source context.
type Options struct {
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
	function  *profile.Function
	flat      int64
	cum       int64
	minLine   int
	lineNanos map[int]int64
}

// FromFile reads a pprof CPU profile from path and ranks its hotspots.
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
	cpuIndex, unit := cpuSampleIndex(p)
	if cpuIndex < 0 {
		return nil, fmt.Errorf("no cpu sample type")
	}

	stats := make(map[string]*functionStats)
	getStats := func(fn *profile.Function) *functionStats {
		if fn == nil {
			return nil
		}
		if s := stats[fn.Name]; s != nil {
			return s
		}
		s := &functionStats{function: fn, lineNanos: make(map[int]int64)}
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

	var total int64
	for _, sample := range p.Sample {
		if sample == nil || cpuIndex >= len(sample.Value) {
			continue
		}
		value := sample.Value[cpuIndex]
		total += value

		// pprof stores a sample's stack from leaf to root. The first function
		// in the leaf location is consequently the flat attribution target.
		if len(sample.Location) > 0 && sample.Location[0] != nil {
			var leaf profile.Line
			for _, line := range sample.Location[0].Line {
				if line.Function != nil {
					leaf = line
					break
				}
			}
			if leaf.Function != nil {
				s := getStats(leaf.Function)
				s.flat += value
				if leaf.Line > 0 {
					s.lineNanos[int(leaf.Line)] += value
				}
			}
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
	if len(focus) > 0 {
		filtered := survivors[:0]
		for _, s := range survivors {
			if matchesAnyPrefix(s.function.Name, focus) {
				filtered = append(filtered, s)
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
			Path: path, SampleUnit: unit, TotalNanos: total, AnalyzedNanos: analyzed,
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
			FlatNanos: s.flat, CumNanos: s.cum,
		}
		if analyzed != 0 {
			h.FlatPct = float64(s.flat) / float64(analyzed) * 100
		}
		if total != 0 {
			h.CumPct = float64(s.cum) / float64(total) * 100
		}
		h.Source = sourceExcerpt(h.File, h.Line, context, s.lineNanos)
		result.Hotspots = append(result.Hotspots, h)
	}
	return result, nil
}

func cpuSampleIndex(p *profile.Profile) (int, string) {
	for i, sampleType := range p.SampleType {
		if sampleType != nil && sampleType.Type == "cpu" {
			return i, sampleType.Unit
		}
	}
	return -1, ""
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
			flatByModule[module] += s.flat
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

func sourceExcerpt(filename string, fallbackLine, context int, lineNanos map[int]int64) *schema.SourceExcerpt {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(contents), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	hottest, hottestValue := fallbackLine, int64(0)
	for line, value := range lineNanos {
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
	excerptNanos := make(map[int]int64)
	for line, value := range lineNanos {
		if line >= start && line <= end && value != 0 {
			excerptNanos[line] = value
		}
	}
	return &schema.SourceExcerpt{File: filename, StartLine: start, EndLine: end, Lines: excerptLines, LineNanos: excerptNanos}
}

// topExcluded reports the hottest filtered-out functions, so the caller can see
// where the discarded time went instead of only how much was discarded.
func topExcluded(excluded []*functionStats, total int64, n int) []schema.ExcludedCost {
	// Sorted by cumulative, not self, time. Self time finds the busiest leaf,
	// which for runtime frames is usually the allocator or the scheduler;
	// cumulative time finds the one that explains the shape of the cost, such
	// as runtime.convTnoptr sitting under every iteration of a loop that boxes
	// a value into an interface.
	sort.Slice(excluded, func(i, j int) bool {
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
			Function: s.function.Name, FlatNanos: s.flat, CumNanos: s.cum,
		}
		if total != 0 {
			c.FlatPct = float64(s.flat) / float64(total) * 100
			c.CumPct = float64(s.cum) / float64(total) * 100
		}
		out = append(out, c)
	}
	return out
}
