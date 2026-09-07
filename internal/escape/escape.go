package escape

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/joaolaureano/profadvisor/internal/schema"
	"github.com/joaolaureano/profadvisor/internal/toolchain"
)

// Options describes how to analyze escape behavior in a target project.
type Options struct {
	// Dir is the target project root; empty means "." (current directory).
	Dir string
	// Patterns are package patterns passed to `go build`. Empty means ["./..."].
	Patterns []string
	// Timeout bounds the build; zero means 5 minutes.
	Timeout time.Duration
}

// Run analyzes escape behavior in a target project and returns a stable report.
//
// It detects the toolchain, runs the build with -gcflags=-m=2 to get escape
// analysis diagnostics, parses them into findings normalized behind a stable
// vocabulary, and assembles an EscapeReport for emission as JSON.
func Run(ctx context.Context, o Options) (*schema.EscapeReport, error) {
	// Resolve Dir to an absolute path.
	dir := o.Dir
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("escape: resolving dir: %w", err)
	}
	// Checked here rather than left to the go tool, whose error for a missing
	// directory is about package loading and does not name the flag at fault.
	stat, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("escape: cannot read %s: %w", absDir, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("escape: %s is not a directory", absDir)
	}

	// Detect the toolchain.
	info, err := toolchain.Detect(ctx, absDir)
	if err != nil {
		return nil, fmt.Errorf("escape: detecting toolchain: %w", err)
	}

	// Run the build with -gcflags=-m=2.
	patterns := o.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	buildRes, err := toolchain.Build(ctx, toolchain.BuildOptions{
		Dir:      absDir,
		Patterns: patterns,
		Gcflags:  "-m=2",
		Timeout:  o.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("escape: building: %w", err)
	}

	// Parse the build output.
	parseRes, err := Parse(buildRes.Stderr, info.Version)
	if err != nil {
		return nil, fmt.Errorf("escape: parsing: %w", err)
	}

	// Build the report.
	report := &schema.EscapeReport{
		SchemaVersion: schema.EscapeVersion,
		Toolchain: schema.Toolchain{
			Path:    info.Path,
			Version: info.Version,
			Raw:     info.Raw,
			GOOS:    info.GOOS,
			GOARCH:  info.GOARCH,
		},
		Analysis: schema.EscapeAnalysis{
			Dir:               absDir,
			Patterns:          patterns,
			Command:           buildRes.Command,
			ParserProfile:     parseRes.Profile,
			TotalLines:        parseRes.Total,
			RecognizedLines:   parseRes.Recognized,
			IgnoredLines:      parseRes.Ignored,
			UnrecognizedLines: len(parseRes.Unrecognized),
			DurationNanos:     buildRes.Duration.Nanoseconds(),
		},
		Findings:     parseRes.Findings,
		Unrecognized: parseRes.Unrecognized,
		Warnings:     parseRes.Warnings,
	}

	// Sorted by position so the document is stable across runs. The compiler's
	// own order is an artifact of how it schedules packages and would make two
	// reports of unchanged code differ.
	slices.SortFunc(report.Findings, func(a, b schema.EscapeFinding) int {
		if c := cmp.Compare(a.Package, b.Package); c != 0 {
			return c
		}
		if c := cmp.Compare(a.File, b.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Line, b.Line); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Column, b.Column); c != 0 {
			return c
		}
		return cmp.Compare(string(a.Kind), string(b.Kind))
	})

	// Build the summary.
	summary := schema.EscapeSummary{
		Packages: len(parseRes.Packages),
		Files:    len(parseRes.Files),
		Findings: len(report.Findings),
		ByKind:   make(map[string]int),
	}
	for _, finding := range report.Findings {
		summary.ByKind[string(finding.Kind)]++
	}
	report.Summary = summary

	// Append a warning if there are unrecognized lines.
	if len(parseRes.Unrecognized) > 0 {
		msg := fmt.Sprintf("escape: %d lines were not recognized; the parser may be behind the toolchain", len(parseRes.Unrecognized))
		report.Warnings = append(report.Warnings, msg)
	}

	return report, nil
}
