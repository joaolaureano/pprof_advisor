// Package verify compares repeated Go benchmark measurements.
//
// A run has one objective, but reading only the objective is how a tool talks
// itself into a bad change: fewer bytes per operation is not a win if the patch
// that achieved it doubled the time. So every metric the measurement declares
// is compared, and each carries the role that says what it is allowed to
// decide — the objective and its guards vote, the rest are reported.
package verify

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
)

// Options controls which metrics and significance level are used.
type Options struct {
	// Alpha is the significance level. Zero means 0.05.
	Alpha float64
	// Measurement names the objective and, through it, the guard and
	// informational metrics. The zero value means CPU nanoseconds.
	Measurement measurement.Config
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
	alpha := opts.Alpha
	if alpha == 0 {
		alpha = 0.05
	}
	metrics := cfg.Metrics()
	units := make([]string, 0, len(metrics))
	for _, m := range metrics {
		units = append(units, m.Unit)
	}

	// Both sides are read once for every unit at the same time: re-reading a
	// stream is not possible, and re-opening it per metric would allow the
	// objective and its guard to come from different parses of the same file.
	baselineValues, err := readBenchmarks(baseline, baselineName, units)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	afterValues, err := readBenchmarks(after, afterName, units)
	if err != nil {
		return nil, fmt.Errorf("read after: %w", err)
	}

	result := &schema.VerifyResult{
		SchemaVersion: schema.Version,
		Measurement:   cfg,
		Verdict:       schema.VerdictNoChange,
	}
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

	objectiveVerdict, guardRegressed := "", false
	for _, name := range names {
		oldByUnit, oldOK := baselineValues[name]
		newByUnit, newOK := afterValues[name]
		if !oldOK {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: missing from baseline %q", name, baselineName))
			continue
		}
		if !newOK {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: missing from after %q", name, afterName))
			continue
		}
		for _, metric := range metrics {
			oldValues, newValues := oldByUnit[metric.Unit], newByUnit[metric.Unit]
			if len(oldValues) == 0 || len(newValues) == 0 {
				if metric.Role != measurement.Informational {
					result.Warnings = append(result.Warnings, fmt.Sprintf(
						"%s: no %s samples; %s metric could not be checked (is -benchmem on?)",
						name, metric.Unit, metric.Role))
				}
				continue
			}
			comparison, warnings := compare(name, metric, oldValues, newValues, alpha)
			result.Comparisons = append(result.Comparisons, comparison)
			result.Warnings = append(result.Warnings, warnings...)

			switch metric.Role {
			case measurement.Objective:
				if comparison.Verdict == schema.VerdictRegressed {
					objectiveVerdict = schema.VerdictRegressed
				} else if comparison.Verdict == schema.VerdictImproved && objectiveVerdict == "" {
					objectiveVerdict = schema.VerdictImproved
				}
			case measurement.Guard:
				// A guard never earns an acceptance, it only withholds one:
				// a memory patch that happens to be faster is still judged on
				// the bytes it saved.
				if comparison.Verdict == schema.VerdictRegressed {
					guardRegressed = true
				}
			}
		}
	}

	switch {
	case objectiveVerdict == schema.VerdictRegressed || guardRegressed:
		result.Verdict = schema.VerdictRegressed
	case objectiveVerdict == schema.VerdictImproved:
		result.Verdict = schema.VerdictImproved
	}
	return result, nil
}

// readBenchmarks collects every requested unit in one pass, keyed by benchmark
// name and then by unit. Original units are preferred so callers can ask for
// the familiar units emitted by go test even though benchfmt tidies some
// internally.
func readBenchmarks(input io.Reader, name string, units []string) (map[string]map[string][]float64, error) {
	if input == nil {
		return nil, fmt.Errorf("%q: nil reader", name)
	}
	reader := benchfmt.NewReader(input, name)
	values := make(map[string]map[string][]float64)
	benchmarks := 0
	for reader.Scan() {
		switch record := reader.Result().(type) {
		case *benchfmt.Result:
			benchmarks++
			key := record.Name.String()
			byUnit := values[key]
			if byUnit == nil {
				byUnit = make(map[string][]float64, len(units))
				values[key] = byUnit
			}
			for _, unit := range units {
				for _, value := range record.Values {
					if value.OrigUnit == unit {
						byUnit[unit] = append(byUnit[unit], value.OrigValue)
						break
					}
					if value.Unit == unit {
						byUnit[unit] = append(byUnit[unit], value.Value)
						break
					}
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

func compare(name string, metric measurement.Metric, oldValues, newValues []float64, alpha float64) (schema.BenchComparison, []string) {
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
		if !metric.LowerIsBetter || strings.HasSuffix(metric.Unit, "/s") {
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
		Unit:           metric.Unit,
		Role:           metric.Role,
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
