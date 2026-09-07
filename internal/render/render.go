// Package render is where values and documents become readable text — for a human
// reading --format text and for a model reading a prompt, which is the same
// operation. This package returns strings and never prints, like every other
// package under internal/.
package render

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/joaolaureano/profadvisor/internal/benchgen"
	"github.com/joaolaureano/profadvisor/internal/capture"
	"github.com/joaolaureano/profadvisor/internal/measurement"
	"github.com/joaolaureano/profadvisor/internal/schema"
)

// Text renders a result document for a human reading --format text.
// It accepts the seven document types, as both pointers and values.
// Unknown types return an error naming the type.
func Text(v any) (string, error) {
	switch v := v.(type) {
	case *benchgen.Result:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *benchgen.Result")
		}
		return textBenchgen(v), nil
	case benchgen.Result:
		return textBenchgen(&v), nil
	case *capture.Result:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *capture.Result")
		}
		return textCapture(v)
	case capture.Result:
		return textCapture(&v)
	case *schema.ExtractResult:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *schema.ExtractResult")
		}
		return textExtract(v)
	case schema.ExtractResult:
		return textExtract(&v)
	case *schema.ApplyResult:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *schema.ApplyResult")
		}
		return textApply(v)
	case schema.ApplyResult:
		return textApply(&v)
	case *schema.VerifyResult:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *schema.VerifyResult")
		}
		return textVerify(v)
	case schema.VerifyResult:
		return textVerify(&v)
	case *schema.EscapeReport:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *schema.EscapeReport")
		}
		return textEscape(v)
	case schema.EscapeReport:
		return textEscape(&v)
	case *schema.PromptResult:
		if v == nil {
			return "", fmt.Errorf("Text: cannot render nil *schema.PromptResult")
		}
		return textPrompt(v), nil
	case schema.PromptResult:
		return textPrompt(&v), nil
	default:
		return "", fmt.Errorf("Text: unsupported type %T", v)
	}
}

// textPrompt prints the two prompts ready to paste. The response schema is
// omitted: it is a machine contract for a caller building its own request, and
// that caller is reading the JSON form.
func textPrompt(r *schema.PromptResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== system ===\n\n%s", r.System)
	if !strings.HasSuffix(r.System, "\n") {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n=== user ===\n\n%s", r.User)
	if !strings.HasSuffix(r.User, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func textBenchgen(r *benchgen.Result) string {
	var b strings.Builder
	argumentTypes := r.Manifest.Target.ArgumentTypes
	if len(argumentTypes) == 0 {
		argumentTypes = r.Manifest.Target.InputTypes
	}
	fmt.Fprintf(&b, "Benchmark generation\n  Schema version: %d\n  Target: %s.%s (%s)\n  Fuzz inputs: %s\n  Seeds: %d\n  Corpus SHA256: %s\n  Code: %s\n  Manifest: %s\n", r.SchemaVersion, r.Manifest.Target.Package, r.Manifest.Target.Function, strings.Join(argumentTypes, ", "), strings.Join(r.Manifest.Target.InputTypes, ", "), len(r.Manifest.Seeds), r.Manifest.CorpusHash, r.CodePath, r.ManifestPath)
	if len(r.Manifest.Target.Implementations) > 0 {
		fmt.Fprintf(&b, "  Implementations: %s\n", strings.Join(r.Manifest.Target.Implementations, ", "))
	}
	if r.InstalledPath != "" {
		fmt.Fprintf(&b, "  Installed: %s\n", r.InstalledPath)
	}
	fmt.Fprintf(&b, "  Generated: %t\n  Validated: %t\nReplay the seeds before measuring; generation does not execute the target.\n", r.Generated, r.Validated)
	return b.String()
}

func textCapture(r *capture.Result) (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Capture Result\n")
	fmt.Fprintf(&buf, "  Dir:            %s\n", r.Dir)
	fmt.Fprintf(&buf, "  Objective:      %s (%s)\n", r.Measurement.Unit, r.Measurement.Profile)
	fmt.Fprintf(&buf, "  Profile:        %s\n", r.ProfilePath)
	fmt.Fprintf(&buf, "  Benchmark:      %s\n", r.BenchPath)
	fmt.Fprintf(&buf, "  Duration:       %v\n", r.Duration)
	fmt.Fprintf(&buf, "  Command:        %s\n", strings.Join(r.Command, " "))
	return buf.String(), nil
}

func textExtract(r *schema.ExtractResult) (string, error) {
	var buf bytes.Buffer
	cfg := r.Profile.Measurement
	cost := measurement.Coster(cfg)

	// Header: profile stats
	fmt.Fprintf(&buf, "Profile Analysis\n")
	fmt.Fprintf(&buf, "  Path:           %s\n", r.Profile.Path)
	fmt.Fprintf(&buf, "  Objective:      %s (%s)\n", cfg.Unit, cfg.Profile)
	totalCost := cost(r.Profile.Total)
	analyzedCost := cost(r.Profile.Analyzed)
	pct := measurement.Dec1(measurement.Pct(r.Profile.Analyzed, r.Profile.Total))
	fmt.Fprintf(&buf, "  Total:          %s\n", totalCost)
	fmt.Fprintf(&buf, "  Analyzed:       %s (%s%%)\n", analyzedCost, pct)
	if r.Profile.DurationNanos > 0 {
		fmt.Fprintf(&buf, "  Duration:       %s\n", measurement.Nanos(r.Profile.DurationNanos))
	}
	if len(r.Profile.FocusPrefixes) > 0 {
		fmt.Fprintf(&buf, "  Focus:          %s\n", strings.Join(r.Profile.FocusPrefixes, ", "))
	}

	// Excluded costs (if any)
	if len(r.Profile.Excluded) > 0 {
		// Ordered by what the code under test reached, which is the only
		// column that explains the cost filtering removed. The global
		// cumulative is shown beside it because the gap between the two is
		// itself informative: a large cum with a small from-focus is a frame
		// busy on some other goroutine.
		fmt.Fprintf(&buf, "\nFiltered out (not ranked), by cost reached from the focus package:\n")
		for _, e := range r.Profile.Excluded {
			fmt.Fprintf(&buf, "  %-34s %5.1f%% from focus, %5.1f%% of profile\n",
				e.Function, e.FromFocusPct, e.CumPct)
		}
	}

	// Hotspots
	fmt.Fprintf(&buf, "\nHotspots:\n")
	for i, h := range r.Hotspots {
		fmt.Fprintf(&buf, "\n%d. %s\n", i+1, measurement.TrimShape(h.Function))
		fmt.Fprintf(&buf, "   File:     %s:%d\n", h.File, h.Line)
		fmt.Fprintf(&buf, "   Self:     %s (%.1f%%)\n", cost(h.Flat), h.FlatPct)
		fmt.Fprintf(&buf, "   Cumul:    %s (%.1f%%)\n", cost(h.Cum), h.CumPct)

		// Source with per-line costs
		if h.Source != nil {
			fmt.Fprintf(&buf, "   Source:\n")
			for off, line := range h.Source.Lines {
				ln := h.Source.StartLine + off
				if value, hot := h.Source.LineCosts[ln]; hot && value > 0 {
					fmt.Fprintf(&buf, "%6d | %-70s // %s self\n", ln, line, cost(value))
					continue
				}
				fmt.Fprintf(&buf, "%6d | %s\n", ln, line)
			}
		}
	}

	return buf.String(), nil
}

func textApply(r *schema.ApplyResult) (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Apply Result\n")
	fmt.Fprintf(&buf, "  Branch:         %s\n", r.Branch)
	fmt.Fprintf(&buf, "  Base ref:       %s\n", r.BaseRef)
	fmt.Fprintf(&buf, "  Commit:         %s\n", r.Commit)
	fmt.Fprintf(&buf, "  Files changed:  %d\n", len(r.FilesChanged))
	if len(r.FilesChanged) > 0 {
		for _, file := range r.FilesChanged {
			fmt.Fprintf(&buf, "    %s\n", file)
		}
	}
	return buf.String(), nil
}

func textVerify(r *schema.VerifyResult) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Verification\n")
	fmt.Fprintf(&b, "  Verdict:        %s\n", r.Verdict)
	fmt.Fprintf(&b, "  Objective:      %s (%s)\n", r.Measurement.Unit, r.Measurement.Profile)

	if len(r.Comparisons) > 0 {
		// Widths come from the data. A benchmark name has no bound worth
		// guessing at, and a fixed column that overflows is worse than no
		// column at all: it silently destroys the alignment of every row
		// after it.
		// Counted in runes, not bytes: the verdicts are Portuguese and
		// "SEM DIFERENÇA" is 14 bytes but 13 columns wide.
		w := func(header string, get func(schema.BenchComparison) string) int {
			n := utf8.RuneCountInString(header)
			for _, c := range r.Comparisons {
				if l := utf8.RuneCountInString(get(c)); l > n {
					n = l
				}
			}
			return n
		}
		name := w("BENCHMARK", func(c schema.BenchComparison) string { return c.Name })
		role := w("ROLE", func(c schema.BenchComparison) string { return c.Role })
		unit := w("UNIT", func(c schema.BenchComparison) string { return c.Unit })

		fmt.Fprintf(&b, "\n%-*s  %-*s  %-*s  %12s  %12s  %8s  %8s  %*s\n",
			name, "BENCHMARK", role, "ROLE", unit, "UNIT",
			"BASELINE", "AFTER", "DELTA%", "P", 0, "VERDICT")
		for _, c := range r.Comparisons {
			// A p-value only means something once the test says it is
			// significant; printing a bare number invites reading 0.06 as
			// almost-significant.
			p := fmt.Sprintf("%.4f", c.PValue)
			if !c.Significant {
				p = "(" + p + ")"
			}
			fmt.Fprintf(&b, "%-*s  %-*s  %-*s  %12.2f  %12.2f  %8s  %8s  %*s\n",
				name, c.Name, role, c.Role, unit, c.Unit,
				c.BaselineCenter, c.AfterCenter,
				measurement.Dec1(c.DeltaPct), p, 0, c.Verdict)
		}
		fmt.Fprintf(&b, "\nA p-value in parentheses was not significant at the chosen alpha.\n")
		fmt.Fprintf(&b, "Only the objective and its guards decide the verdict; guards can veto, never accept.\n")
	}

	if len(r.Warnings) > 0 {
		fmt.Fprintf(&b, "\nWarnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&b, "  %s\n", w)
		}
	}
	return b.String(), nil
}

func textEscape(r *schema.EscapeReport) (string, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Escape Analysis\n")
	fmt.Fprintf(&buf, "  Toolchain:      %s\n", r.Toolchain.Version)
	fmt.Fprintf(&buf, "  Path:           %s\n", r.Toolchain.Path)
	fmt.Fprintf(&buf, "  GOOS/GOARCH:    %s/%s\n", r.Toolchain.GOOS, r.Toolchain.GOARCH)
	fmt.Fprintf(&buf, "\nAnalysis:\n")
	fmt.Fprintf(&buf, "  Dir:            %s\n", r.Analysis.Dir)
	fmt.Fprintf(&buf, "  Patterns:       %s\n", strings.Join(r.Analysis.Patterns, ", "))
	fmt.Fprintf(&buf, "  Parser:         %s\n", r.Analysis.ParserProfile)
	fmt.Fprintf(&buf, "  Total lines:    %d\n", r.Analysis.TotalLines)
	fmt.Fprintf(&buf, "  Recognized:     %d\n", r.Analysis.RecognizedLines)
	fmt.Fprintf(&buf, "  Ignored:        %d\n", r.Analysis.IgnoredLines)
	fmt.Fprintf(&buf, "  Unrecognized:   %d\n", r.Analysis.UnrecognizedLines)
	fmt.Fprintf(&buf, "  Duration:       %s\n", measurement.Nanos(r.Analysis.DurationNanos))

	fmt.Fprintf(&buf, "\nSummary:\n")
	fmt.Fprintf(&buf, "  Packages:       %d\n", r.Summary.Packages)
	fmt.Fprintf(&buf, "  Files:          %d\n", r.Summary.Files)
	fmt.Fprintf(&buf, "  Findings:       %d\n", r.Summary.Findings)

	if len(r.Summary.ByKind) > 0 {
		fmt.Fprintf(&buf, "  By kind:\n")
		// Sort keys for deterministic output
		kinds := make([]string, 0, len(r.Summary.ByKind))
		for k := range r.Summary.ByKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			fmt.Fprintf(&buf, "    %-30s %d\n", kind, r.Summary.ByKind[kind])
		}
	}

	// Findings grouped by file
	if len(r.Findings) > 0 {
		fmt.Fprintf(&buf, "\nFindings:\n")
		fileMap := make(map[string][]schema.EscapeFinding)
		for _, f := range r.Findings {
			fileMap[f.File] = append(fileMap[f.File], f)
		}
		for _, f := range r.Findings {
			if _, seen := fileMap[f.File]; seen {
				fmt.Fprintf(&buf, "\n%s\n", f.File)
				for _, finding := range fileMap[f.File] {
					fmt.Fprintf(&buf, "  %d:%d %s %s\n", finding.Line, finding.Column, finding.Kind, finding.Subject)
					if finding.Function != "" {
						fmt.Fprintf(&buf, "    function: %s\n", finding.Function)
					}
					if finding.Target != "" {
						fmt.Fprintf(&buf, "    target: %s\n", finding.Target)
					}
					fmt.Fprintf(&buf, "    evidence: %s\n", finding.Evidence)
					if len(finding.Flow) > 0 {
						fmt.Fprintf(&buf, "    flow:\n")
						for _, step := range finding.Flow {
							if step.Reason == "" {
								fmt.Fprintf(&buf, "      %s\n", step.Text)
							} else {
								fmt.Fprintf(&buf, "      %s (%s at %s:%d:%d)\n",
									step.Text, step.Reason, step.File, step.Line, step.Column)
							}
						}
					}
				}
				delete(fileMap, f.File)
			}
		}
	}

	// Unrecognized lines
	if len(r.Unrecognized) > 0 {
		fmt.Fprintf(&buf, "\nUnrecognized:\n")
		for _, u := range r.Unrecognized {
			if u.File != "" {
				fmt.Fprintf(&buf, "  %s:%d: %s\n", u.File, u.Line, u.Text)
			} else if u.Package != "" {
				fmt.Fprintf(&buf, "  %s: %s\n", u.Package, u.Text)
			} else {
				fmt.Fprintf(&buf, "  %s\n", u.Text)
			}
		}
	}

	// Warnings
	if len(r.Warnings) > 0 {
		fmt.Fprintf(&buf, "\nWarnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&buf, "  %s\n", w)
		}
	}

	return buf.String(), nil
}
