// Package escape turns the Go compiler's escape-analysis diagnostics into a
// stable report.
//
// The compiler is the source of truth for whether a value escapes, and this
// package does not re-derive that. What it does is insulate the rest of the
// program from how the answer is phrased: diagnostic text is prose, it has
// been reworded across releases, and a consumer that greps for "escapes to
// heap" breaks on a toolchain upgrade. Everything past this package speaks
// schema.EscapeKind.
//
// An escape is not a defect. Nothing here ranks, scores, or recommends.
package escape

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/joaolaureano/profadvisor/internal/schema"
)

// position is a comparable key for the position of a compiler diagnostic.
// It replaces string formatting to avoid allocations per line.
type position struct {
	file string
	line int
	col  int
}

// Result is what Parse returns: the findings extracted from compiler stderr,
// along with counts and metadata about the parsing process.
type Result struct {
	Findings     []schema.EscapeFinding
	Unrecognized []schema.RawDiagnostic
	Profile      string // the ruleset name, for EscapeAnalysis.ParserProfile
	Warnings     []string
	Total        int // every line seen
	Recognized   int // lines that became findings
	Ignored      int // understood and deliberately not reported
	Packages     []string
	Files        []string
}

// Parse turns one build's captured stderr into findings. It preserves the order
// of findings as emitted by the compiler and never fails due to an unrecognized
// diagnostic — instead, unrecognized lines are collected in the result and a
// warning is appended if the toolchain version exceeds what the parser was
// validated against.
func Parse(stderr []byte, toolchainVersion string) (*Result, error) {
	// Preallocate Findings based on input size. The measured ratio on real
	// output is roughly one finding per 4-8 newlines; we reserve 1/4 of the
	// newline count to stay conservative without overallocating on every run.
	// Unrecognized is left nil: it is normally empty and reserving for it
	// would waste memory on healthy output.
	findingsCapacity := (bytes.Count(stderr, []byte("\n")) + 3) / 4
	r := &Result{
		Findings: make([]schema.EscapeFinding, 0, findingsCapacity),
		Packages: []string{},
		Files:    []string{},
		Warnings: []string{},
		Profile:  "go1.20+",
	}

	// Check version and add warnings if needed.
	checkVersion(toolchainVersion, r)

	// Track unique packages and files.
	seenPackages := make(map[string]bool)
	seenFiles := make(map[string]bool)
	truncated := 0

	// Convert input to string once and iterate with IndexByte. Substrings of a
	// string share the backing array, so per-line cost is zero copies.
	fullText := string(stderr)
	currentPackage := ""
	pendingBlocks := make(map[position]*schema.EscapeFinding)

	// Iterate through the string line by line using IndexByte to find newlines.
	for i := 0; i <= len(fullText); {
		// Find the next newline or end of string.
		nextNewline := strings.IndexByte(fullText[i:], '\n')
		var lineEnd int
		if nextNewline < 0 {
			lineEnd = len(fullText)
		} else {
			lineEnd = i + nextNewline
		}

		// Extract the line (substrings share the backing array).
		lineText := fullText[i:lineEnd]

		// Move to next line start.
		if lineEnd < len(fullText) {
			i = lineEnd + 1
		} else {
			i = len(fullText) + 1
		}

		// A trailing newline creates an empty line at the end, and the go tool
		// emits blank lines of its own. Neither is a diagnostic, so neither is
		// counted: Total is the number of lines the parser had to account for.
		if strings.TrimSpace(lineText) == "" {
			continue
		}
		r.Total++

		// Check for package header.
		if strings.HasPrefix(lineText, "# ") {
			currentPackage = strings.TrimPrefix(lineText, "# ")
			r.Ignored++
			if !seenPackages[currentPackage] {
				seenPackages[currentPackage] = true
				r.Packages = append(r.Packages, currentPackage)
			}
			continue
		}

		// Try to parse position: file:line:col: rest
		file, line, col, rest, ok := parsePosition(lineText)
		if !ok {
			// No position prefix.
			r.Unrecognized = append(r.Unrecognized, schema.RawDiagnostic{
				Package: currentPackage,
				Text:    lineText,
			})
			continue
		}

		// Remove leading "./" from file if present.
		if strings.HasPrefix(file, "./") {
			file = strings.TrimPrefix(file, "./")
		}

		posKey := position{file: file, line: line, col: col}

		// Track file.
		if !seenFiles[file] {
			seenFiles[file] = true
			r.Files = append(r.Files, file)
		}

		// Check for flow continuation.
		if isFlowContinuation(rest) {
			if pending, ok := pendingBlocks[posKey]; ok {
				if strings.HasPrefix(rest, truncatedExplanation) {
					// The flow above this line stops early. Recording the count
					// matters because the alternative is a consumer reading a
					// partial explanation as a complete one.
					truncated++
				} else if step, ok := parseFlowStep(rest, file); ok {
					pending.Flow = append(pending.Flow, step)
				}
				r.Ignored++
				continue
			}
			// Flow continuation without an open block is unrecognized.
			r.Unrecognized = append(r.Unrecognized, schema.RawDiagnostic{
				Package: currentPackage,
				File:    file,
				Line:    line,
				Column:  col,
				Text:    lineText,
			})
			continue
		}

		// Check for explanation heading.
		if isExplanationHeading(rest) {
			function, ok := parseExplanationHeading(rest)
			if ok {
				// Create a pending block for this position. Give Flow a small initial
				// capacity: a typical explanation has a handful of steps.
				pendingBlocks[posKey] = &schema.EscapeFinding{
					Package:  currentPackage,
					File:     file,
					Line:     line,
					Column:   col,
					Function: function,
					Flow:     make([]schema.EscapeFlowStep, 0, 4),
				}
			}
			r.Ignored++
			continue
		}

		// Check for inlining/alias/rewriting (these are ignored).
		if isIgnoredDiagnostic(rest) {
			r.Ignored++
			continue
		}

		// Check for escape template.
		finding, ok := parseEscapeTemplate(rest, file, line, col, currentPackage, lineText)
		if ok {
			r.Recognized++
			// If a pending block exists at this position, attach its function and flow.
			if pending, ok := pendingBlocks[posKey]; ok {
				finding.Function = pending.Function
				finding.Flow = pending.Flow
				delete(pendingBlocks, posKey)
			}
			r.Findings = append(r.Findings, finding)
			continue
		}

		// Unrecognized.
		r.Unrecognized = append(r.Unrecognized, schema.RawDiagnostic{
			Package: currentPackage,
			File:    file,
			Line:    line,
			Column:  col,
			Text:    lineText,
		})
	}

	if truncated > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"escape: the compiler truncated %d flow explanation(s) at an assignment cycle; those findings are correct but their flow is partial", truncated))
	}
	return r, nil
}

// parsePosition splits "file:line:col: rest" into its parts.
//
// The column is optional. The compiler omits it for positions it synthesised
// rather than read from source — "<autogenerated>:1:" on the wrapper methods it
// writes for embedded types and shape instantiations — and a real project
// produces hundreds of those. Requiring three parts sent every one of them to
// the unrecognized pile, which is both wrong and loud enough to bury a genuine
// gap.
//
// The file is everything before the numeric tail rather than everything before
// the first colon, so a Windows drive letter or a path containing a colon does
// not split in the wrong place. To find the numeric tail without allocating,
// we use LastIndexByte to locate the colons directly and extract the numeric
// parts, avoiding strings.Split.
func parsePosition(s string) (file string, line, col int, rest string, ok bool) {
	idx := strings.Index(s, ": ")
	if idx < 0 {
		return "", 0, 0, "", false
	}
	prefix := s[:idx]
	rest = s[idx+2:]

	// Try to find two numeric parts at the end, separated by colon.
	// Start from the right: the last colon separates potential col from line,
	// and the second-to-last separates line from file.
	lastColon := strings.LastIndexByte(prefix, ':')
	if lastColon < 0 {
		return "", 0, 0, "", false
	}

	// Try 3-part form: file:line:col
	// The second-to-last colon (if it exists) separates file from line.
	secondLastColon := strings.LastIndexByte(prefix[:lastColon], ':')
	if secondLastColon >= 0 {
		// Try parsing as file:line:col
		lineStr := prefix[secondLastColon+1 : lastColon]
		colStr := prefix[lastColon+1:]
		lineNum, errL := strconv.Atoi(lineStr)
		colNum, errC := strconv.Atoi(colStr)
		if errL == nil && errC == nil {
			return prefix[:secondLastColon], lineNum, colNum, rest, true
		}
	}

	// Try 2-part form: file:line (no column)
	lineStr := prefix[lastColon+1:]
	lineNum, err := strconv.Atoi(lineStr)
	if err == nil {
		return prefix[:lastColon], lineNum, 0, rest, true
	}

	return "", 0, 0, "", false
}

// truncatedExplanation is what the compiler prints in place of the rest of a
// flow when it finds an assignment cycle. It is a continuation line, not a
// diagnostic of its own, and it means the flow above it is incomplete.
const truncatedExplanation = "  warning: truncated explanation due to assignment cycle"

// isFlowContinuation reports whether the message continues an open explanation
// block rather than starting something new.
//
// The compiler indents these relative to the position prefix: two spaces for the
// "flow:" header that states an edge, four for each "from" hop along it.
func isFlowContinuation(s string) bool {
	return strings.HasPrefix(s, "  flow: ") ||
		strings.HasPrefix(s, "    from ") ||
		strings.HasPrefix(s, truncatedExplanation)
}

// parseFlowStep extracts a flow step from a line starting with "flow: " or "from ".
// It returns false if it cannot parse the line.
func parseFlowStep(s string, file string) (schema.EscapeFlowStep, bool) {
	if strings.HasPrefix(s, "  flow: ") {
		// Extract the text, everything after "  flow: "
		return schema.EscapeFlowStep{Text: strings.TrimPrefix(s, "  flow: ")}, true
	}

	if strings.HasPrefix(s, "    from ") {
		step := schema.EscapeFlowStep{}
		// Extract the "from ..." part and the reason in parentheses.
		s := strings.TrimPrefix(s, "    from ")

		// Look for " at " and parentheses pattern " (reason) at file:line:col"
		// Example: "x (spill) at ./heap.go:5:2"
		atIdx := strings.LastIndex(s, " at ")
		if atIdx < 0 {
			step.Text = s
			return step, true
		}

		beforeAt := s[:atIdx]
		afterAt := s[atIdx+4:]

		// Extract reason from parentheses.
		var reason string
		openParen := strings.LastIndex(beforeAt, "(")
		closeParen := strings.LastIndex(beforeAt, ")")
		if openParen >= 0 && closeParen > openParen {
			reason = beforeAt[openParen+1 : closeParen]
			step.Text = strings.TrimSpace(beforeAt[:openParen])
		} else {
			step.Text = beforeAt
		}

		step.Reason = reason

		// Parse the file:line:col after "at ".
		posFile, posLine, posCol, _, ok := parsePosition(afterAt + ": dummy")
		if ok {
			step.File = posFile
			if strings.HasPrefix(step.File, "./") {
				step.File = strings.TrimPrefix(step.File, "./")
			}
			step.Line = posLine
			step.Column = posCol
		}

		return step, true
	}

	return schema.EscapeFlowStep{}, false
}

// isExplanationHeading checks if the message is an explanation heading.
// Both templates end with `:` and contain specific keywords.
func isExplanationHeading(s string) bool {
	if !strings.HasSuffix(s, ":") {
		return false
	}
	// Both headings are matched on their full shape rather than on a keyword.
	// A loose test here is dangerous in one direction only: a summary mistaken
	// for a heading is a finding silently lost, which is precisely the failure
	// this package exists to prevent.
	if strings.Contains(s, " escapes to heap in ") {
		return true
	}
	return strings.HasPrefix(s, "parameter ") &&
		strings.Contains(s, " leaks to ") &&
		strings.Contains(s, " for ") &&
		strings.Contains(s, " with derefs=")
}

// parseExplanationHeading extracts the function name from an explanation heading.
func parseExplanationHeading(s string) (string, bool) {
	// Template 1: "%v escapes to heap in %v:"
	if _, function, ok := strings.Cut(s, "escapes to heap in "); ok {
		if !strings.Contains(function, "escapes to heap in ") {
			function = strings.TrimSuffix(function, ":")
			return function, true
		}
	}

	// Template 2: "parameter %v leaks to %s for %v with derefs=%d:"
	if strings.Contains(s, " leaks to ") && strings.Contains(s, " for ") {
		forIdx := strings.Index(s, " for ")
		withIdx := strings.Index(s, " with derefs=")
		if forIdx > 0 && withIdx > forIdx {
			return s[forIdx+5 : withIdx], true
		}
	}

	return "", false
}

// isIgnoredDiagnostic reports whether the message is a diagnostic this package
// understands and deliberately does not report.
//
// This is an allowlist rather than a fallback, and that is the whole point. The
// alternative — treating anything the escape templates do not match as
// irrelevant — would silently swallow a genuinely new escape diagnostic, which
// is the failure this package is built to prevent. Anything not on this list and
// not an escape template is reported as unrecognized, loudly.
func isIgnoredDiagnostic(s string) bool {
	if s == "" {
		return false
	}
	// "expr", "function arg", "function result" and "dereference" all get a
	// "... will be kept alive" line, which is about liveness, not escape.
	if strings.HasSuffix(s, "will be kept alive") {
		return true
	}
	switch s[0] {
	case 'a':
		return strings.HasPrefix(s, "alias analysis: ")
	case 'c':
		return hasAnyPrefix(s, "can inline ", "cannot inline ", "closure converted to global")
	case 'd':
		return hasAnyPrefix(s, "devirtualizing ", "cannot devirtualize ")
	case 'g':
		return strings.HasPrefix(s, "generated nil check")
	case 'h':
		return strings.HasPrefix(s, "heap closure")
	case 'i':
		return hasAnyPrefix(s, "inlining call to ", "index bounds check elided",
			"intrinsic substitution for ", "imprecise interface call")
	case 'l':
		return strings.HasPrefix(s, "loop variable ")
	case 'p':
		return strings.HasPrefix(s, "partially devirtualizing ")
	case 'r':
		return hasAnyPrefix(s, "reshaping ", "rewriting OCONVIFACE")
	case 's':
		return hasAnyPrefix(s, "stack object ", "stack closure", "skipping static copy of ")
	case 't':
		return hasAnyPrefix(s, "type assertion inlined", "type assertion not inlined", "tail call emitted")
	case 'w':
		return strings.HasPrefix(s, "write barrier")
	default:
		return false
	}
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// parseEscapeTemplate tries to match the message against an escape template.
// It returns false if no template matched.
func parseEscapeTemplate(msg, file string, line, col int, pkg, evidence string) (schema.EscapeFinding, bool) {
	// Try templates in order (more specific first).

	// "moved to heap: %v"
	if strings.HasPrefix(msg, "moved to heap: ") {
		subject := strings.TrimPrefix(msg, "moved to heap: ")
		return schema.EscapeFinding{
			Kind:     schema.EscapeMovedToHeap,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "%v escapes to heap", which also covers "append escapes to heap" with the
	// subject "append". Explanation headings end in ":" and were consumed above.
	if strings.HasSuffix(msg, " escapes to heap") {
		subject := strings.TrimSuffix(msg, " escapes to heap")
		return schema.EscapeFinding{
			Kind:     schema.EscapeEscapesToHeap,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "%v does not escape, mutate, or call" (must come before "%v does not escape")
	if strings.HasSuffix(msg, " does not escape, mutate, or call") {
		subject := strings.TrimSuffix(msg, " does not escape, mutate, or call")
		return schema.EscapeFinding{
			Kind:     schema.EscapeParamInert,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "%v does not escape"
	if strings.HasSuffix(msg, " does not escape") {
		subject := strings.TrimSuffix(msg, " does not escape")
		return schema.EscapeFinding{
			Kind:     schema.EscapeDoesNotEscape,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "leaking param: %v to result %v level=%d", before the plainer form below.
	// If the shape matches but the level will not parse, this returns nil rather
	// than falling through: the looser template would match the same line and
	// produce a finding whose subject is "p to result ~r0 level=x". A wrong
	// answer is worse than an admitted gap.
	if strings.HasPrefix(msg, "leaking param: ") && strings.Contains(msg, " to result ") && strings.Contains(msg, " level=") {
		subject := strings.TrimPrefix(msg, "leaking param: ")
		toIdx := strings.Index(subject, " to result ")
		levelIdx := strings.Index(subject, " level=")
		if toIdx <= 0 || levelIdx <= toIdx {
			return schema.EscapeFinding{}, false
		}
		levelInt, err := strconv.Atoi(subject[levelIdx+7:])
		if err != nil {
			return schema.EscapeFinding{}, false
		}
		target := subject[toIdx+11 : levelIdx]
		return schema.EscapeFinding{
			Kind:     schema.EscapeLeakingParamResult,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject[:toIdx],
			Target:   target,
			Level:    &levelInt,
			Evidence: evidence,
		}, true
	}

	// "leaking param: %v"
	if strings.HasPrefix(msg, "leaking param: ") {
		subject := strings.TrimPrefix(msg, "leaking param: ")
		return schema.EscapeFinding{
			Kind:     schema.EscapeLeakingParam,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "leaking param content: %v"
	if strings.HasPrefix(msg, "leaking param content: ") {
		subject := strings.TrimPrefix(msg, "leaking param content: ")
		return schema.EscapeFinding{
			Kind:     schema.EscapeLeakingParamContent,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	// "mutates param: %v derefs=%v". As above, a shape match with an unparseable
	// derefs count is reported as unrecognized rather than guessed at.
	if strings.HasPrefix(msg, "mutates param: ") && strings.Contains(msg, " derefs=") {
		subject := strings.TrimPrefix(msg, "mutates param: ")
		derefsIdx := strings.Index(subject, " derefs=")
		if derefsIdx <= 0 {
			return schema.EscapeFinding{}, false
		}
		derefsInt, err := strconv.Atoi(subject[derefsIdx+8:])
		if err != nil {
			return schema.EscapeFinding{}, false
		}
		return schema.EscapeFinding{
			Kind:     schema.EscapeMutatesParam,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject[:derefsIdx],
			Derefs:   &derefsInt,
			Evidence: evidence,
		}, true
	}

	// "calls param: %v derefs=%v". As above, a shape match with an unparseable
	// derefs count is reported as unrecognized rather than guessed at.
	if strings.HasPrefix(msg, "calls param: ") && strings.Contains(msg, " derefs=") {
		subject := strings.TrimPrefix(msg, "calls param: ")
		derefsIdx := strings.Index(subject, " derefs=")
		if derefsIdx <= 0 {
			return schema.EscapeFinding{}, false
		}
		derefsInt, err := strconv.Atoi(subject[derefsIdx+8:])
		if err != nil {
			return schema.EscapeFinding{}, false
		}
		return schema.EscapeFinding{
			Kind:     schema.EscapeCallsParam,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject[:derefsIdx],
			Derefs:   &derefsInt,
			Evidence: evidence,
		}, true
	}

	// "assuming %v is unsafe uintptr" and "marking %v as escaping uintptr" variants
	if strings.HasPrefix(msg, "assuming ") && strings.HasSuffix(msg, " is unsafe uintptr") {
		subject := strings.TrimPrefix(msg, "assuming ")
		subject = strings.TrimSuffix(subject, " is unsafe uintptr")
		return schema.EscapeFinding{
			Kind:     schema.EscapeUnsafeUintptr,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Subject:  subject,
			Evidence: evidence,
		}, true
	}

	if strings.HasPrefix(msg, "marking ") && strings.Contains(msg, " as escaping") && strings.Contains(msg, "uintptr") {
		subject := strings.TrimPrefix(msg, "marking ")
		asIdx := strings.Index(subject, " as escaping")
		if asIdx > 0 {
			subject = subject[:asIdx]
			return schema.EscapeFinding{
				Kind:     schema.EscapeUnsafeUintptr,
				Package:  pkg,
				File:     file,
				Line:     line,
				Column:   col,
				Subject:  subject,
				Evidence: evidence,
			}, true
		}
	}

	// "zero-copy string->[]byte conversion"
	if msg == "zero-copy string->[]byte conversion" {
		return schema.EscapeFinding{
			Kind:     schema.EscapeZeroCopyConversion,
			Package:  pkg,
			File:     file,
			Line:     line,
			Column:   col,
			Evidence: evidence,
		}, true
	}

	// "%v capturing by ref: %v (addr=%v assign=%v width=%d)", and the by-value
	// variant. The first %v is the enclosing function and the third is the
	// captured variable; the variable is the subject, since that is the thing
	// the compiler reached a conclusion about.
	//
	// The parenthesised tail is deliberately not parsed into fields. It carries a
	// type width, which is architecture-dependent, and inventing a schema for it
	// would promise a stability the compiler has not offered. It stays in
	// Evidence.
	if idx := strings.Index(msg, " capturing by "); idx > 0 {
		byRef := strings.HasPrefix(msg[idx:], " capturing by ref: ")
		byValue := strings.HasPrefix(msg[idx:], " capturing by value: ")
		tail := msg[idx+len(" capturing by "):]
		colon := strings.Index(tail, ": ")
		if (byRef || byValue) && colon > 0 {
			variable := tail[colon+2:]
			if paren := strings.Index(variable, " ("); paren > 0 {
				variable = variable[:paren]
			}
			return schema.EscapeFinding{
				Kind:     schema.EscapeClosureCapture,
				Package:  pkg,
				File:     file,
				Line:     line,
				Column:   col,
				Subject:  variable,
				Function: msg[:idx],
				ByRef:    &byRef,
				Evidence: evidence,
			}, true
		}
	}

	// "%v ignoring self-assignment in %v"
	if strings.Contains(msg, " ignoring self-assignment in ") {
		idx := strings.Index(msg, " ignoring self-assignment in ")
		if idx > 0 {
			subject := msg[:idx]
			return schema.EscapeFinding{
				Kind:     schema.EscapeSelfAssignment,
				Package:  pkg,
				File:     file,
				Line:     line,
				Column:   col,
				Subject:  subject,
				Evidence: evidence,
			}, true
		}
	}

	return schema.EscapeFinding{}, false
}

// checkVersion checks if the toolchain version is within the validated range
// and adds warnings if not.
func checkVersion(version string, r *Result) {
	const minVersion = "go1.20"
	const validatedThrough = "go1.26.1"

	// Parse version strings to compare them.
	minV := parseVersion(minVersion)
	validV := parseVersion(validatedThrough)
	curV := parseVersion(version)

	if curV == nil {
		r.Warnings = append(r.Warnings, fmt.Sprintf("escape: could not parse toolchain version %q; parsed with default ruleset", version))
		return
	}

	if isNewerThan(curV, validV) {
		r.Warnings = append(r.Warnings, fmt.Sprintf("escape: toolchain version %q is newer than validated version %q; some diagnostics may be unrecognized", version, validatedThrough))
		return
	}

	if isOlderThan(curV, minV) {
		r.Warnings = append(r.Warnings, fmt.Sprintf("escape: toolchain version %q is older than minimum supported version %q; some diagnostics may be unrecognized", version, minVersion))
	}
}

// versionTuple represents a parsed version.
type versionTuple struct {
	major, minor, patch int
}

// parseVersion parses a version string like "go1.20" or "go1.26.1".
func parseVersion(s string) *versionTuple {
	if !strings.HasPrefix(s, "go") {
		return nil
	}
	s = strings.TrimPrefix(s, "go")
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return nil
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return nil
	}
	patch := 0
	if len(parts) > 2 {
		p, err := strconv.Atoi(parts[2])
		if err == nil {
			patch = p
		}
	}
	return &versionTuple{major, minor, patch}
}

// isNewerThan returns true if a is newer than b.
func isNewerThan(a, b *versionTuple) bool {
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	return a.patch > b.patch
}

// isOlderThan returns true if a is older than b.
func isOlderThan(a, b *versionTuple) bool {
	if a.major != b.major {
		return a.major < b.major
	}
	if a.minor != b.minor {
		return a.minor < b.minor
	}
	return a.patch < b.patch
}
