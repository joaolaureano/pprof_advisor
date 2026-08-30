// Package verify compares repeated Go benchmark measurements.
package verify

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/schema"
	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
)

// Options controls which metric and significance level are used for comparison.
type Options struct {
	// Alpha is the significance level. Zero means 0.05.
	Alpha float64
	// Unit selects which metric to compare. Empty means "ns/op".
	Unit string
}

// FromFiles reads two go test -bench outputs and compares their measurements.
func FromFiles(baselinePath, afterPath string, opts Options) (*schema.VerifyResult, error) {
	baseline, err := os.Open(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("open baseline %q: %w", baselinePath, err)
	}
	defer baseline.Close()

	after, err := os.Open(afterPath)
	if err != nil {
		return nil, fmt.Errorf("open after %q: %w", afterPath, err)
	}
	defer after.Close()

	return FromReaders(baseline, baselinePath, after, afterPath, opts)
}

// FromReaders compares measurements from already-open benchmark output streams.
// Names are retained only to make malformed or empty input actionable.
func FromReaders(baseline io.Reader, baselineName string, after io.Reader, afterName string, opts Options) (*schema.VerifyResult, error) {
	unit := opts.Unit
	if unit == "" {
		unit = "ns/op"
	}
	alpha := opts.Alpha
	if alpha == 0 {
		alpha = 0.05
	}

	baselineValues, err := readBenchmarks(baseline, baselineName, unit)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	afterValues, err := readBenchmarks(after, afterName, unit)
	if err != nil {
		return nil, fmt.Errorf("read after: %w", err)
	}

	result := &schema.VerifyResult{SchemaVersion: schema.Version, Verdict: schema.VerdictNoChange}
	names := make([]string, 0, len(baselineValues)+len(afterValues))
	seen := make(map[string]bool, len(baselineValues)+len(afterValues))
	for name := range baselineValues {
		seen[name] = true
		names = append(names, name)
	}
	for name := range afterValues {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		oldValues, oldOK := baselineValues[name]
		newValues, newOK := afterValues[name]
		if !oldOK {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: missing from baseline %q", name, baselineName))
			continue
		}
		if !newOK {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: missing from after %q", name, afterName))
			continue
		}

		comparison, warnings := compare(name, unit, oldValues, newValues, alpha)
		result.Comparisons = append(result.Comparisons, comparison)
		result.Warnings = append(result.Warnings, warnings...)
	}

	for _, comparison := range result.Comparisons {
		if comparison.Verdict == schema.VerdictRegressed {
			result.Verdict = schema.VerdictRegressed
			break
		}
		if comparison.Verdict == schema.VerdictImproved {
			result.Verdict = schema.VerdictImproved
		}
	}
	return result, nil
}

// readBenchmarks keeps original units when available so callers can request the
// familiar units emitted by go test even though benchfmt tidies some internally.
func readBenchmarks(input io.Reader, name, unit string) (map[string][]float64, error) {
	if input == nil {
		return nil, fmt.Errorf("%q: nil reader", name)
	}
	reader := benchfmt.NewReader(input, name)
	values := make(map[string][]float64)
	benchmarks := 0
	for reader.Scan() {
		switch record := reader.Result().(type) {
		case *benchfmt.Result:
			benchmarks++
			for _, value := range record.Values {
				if value.OrigUnit == unit {
					values[record.Name.String()] = append(values[record.Name.String()], value.OrigValue)
					break
				}
				if value.Unit == unit {
					values[record.Name.String()] = append(values[record.Name.String()], value.Value)
					break
				}
			}
		case *benchfmt.SyntaxError:
			return nil, record
		}
	}
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("read %q: %w", name, err)
	}
	if benchmarks == 0 {
		return nil, fmt.Errorf("%q contains zero benchmarks", name)
	}
	return values, nil
}

func compare(name, unit string, oldValues, newValues []float64, alpha float64) (schema.BenchComparison, []string) {
	oldSample := benchmath.NewSample(oldValues, &benchmath.DefaultThresholds)
	newSample := benchmath.NewSample(newValues, &benchmath.DefaultThresholds)
	confidence := 1 - alpha
	oldSummary := benchmath.AssumeNothing.Summary(oldSample, confidence)
	newSummary := benchmath.AssumeNothing.Summary(newSample, confidence)
	stat := benchmath.AssumeNothing.Compare(oldSample, newSample)

	delta := 0.0
	if oldSummary.Center != 0 {
		delta = (newSummary.Center - oldSummary.Center) / oldSummary.Center * 100
	}
	significant := !math.IsNaN(stat.P) && stat.P < alpha
	verdict := schema.VerdictNoChange
	if significant && delta != 0 {
		improved := delta < 0
		if strings.HasSuffix(unit, "/s") {
			improved = delta > 0
		}
		if improved {
			verdict = schema.VerdictImproved
		} else {
			verdict = schema.VerdictRegressed
		}
	}

	warnings := warningStrings(name, oldSample.Warnings, newSample.Warnings, oldSummary.Warnings, newSummary.Warnings, stat.Warnings)
	return schema.BenchComparison{
		Name:           name,
		Unit:           unit,
		BaselineCenter: oldSummary.Center,
		AfterCenter:    newSummary.Center,
		BaselineN:      len(oldValues),
		AfterN:         len(newValues),
		DeltaPct:       delta,
		PValue:         finitePValue(stat.P),
		Significant:    significant,
		Verdict:        verdict,
	}, warnings
}

func warningStrings(name string, groups ...[]error) []string {
	var warnings []string
	for _, group := range groups {
		for _, err := range group {
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}
	return warnings
}

func finitePValue(p float64) float64 {
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return 0
	}
	return p
}
