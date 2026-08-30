// Package matcher is a deliberately unoptimized path matcher.
//
// It exists so profadvisor can test itself without borrowing another project:
// the benchmarks here produce the CPU profiles and benchmark output committed
// under testdata/, and internal/capture's end-to-end test runs against this
// module. Match is a linear scan and Params allocates a map per call, which
// gives one benchmark dominated by the code under test and one dominated by
// the runtime — the two shapes the extract step has to tell apart.
package matcher

import "strings"

// Table matches a path against a set of patterns. A pattern segment beginning
// with ':' captures whatever segment is in that position.
type Table struct {
	patterns []string
}

// New returns a table over patterns, in the order they are given.
func New(patterns []string) *Table {
	return &Table{patterns: append([]string(nil), patterns...)}
}

// Match reports whether path matches any pattern.
func (t *Table) Match(path string) bool {
	for _, pattern := range t.patterns {
		if matches(pattern, path) {
			return true
		}
	}
	return false
}

// Params returns the captured segments of the first matching pattern, or nil.
func (t *Table) Params(path string) map[string]string {
	for _, pattern := range t.patterns {
		if !matches(pattern, path) {
			continue
		}
		captured := make(map[string]string)
		remainingPattern, remainingPath := pattern, path
		for remainingPattern != "" || remainingPath != "" {
			var patternSegment, pathSegment string
			patternSegment, remainingPattern = segment(remainingPattern)
			pathSegment, remainingPath = segment(remainingPath)
			if strings.HasPrefix(patternSegment, ":") {
				captured[patternSegment[1:]] = pathSegment
			}
		}
		return captured
	}
	return nil
}

func matches(pattern, path string) bool {
	for pattern != "" || path != "" {
		var patternSegment, pathSegment string
		patternSegment, pattern = segment(pattern)
		pathSegment, path = segment(path)
		if pathSegment == "" || patternSegment == "" {
			return false
		}
		if strings.HasPrefix(patternSegment, ":") {
			continue
		}
		if patternSegment != pathSegment {
			return false
		}
	}
	return true
}

// segment splits off the first '/'-delimited element of s.
func segment(s string) (head, rest string) {
	s = strings.TrimPrefix(s, "/")
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i], s[i:]
		}
	}
	return s, ""
}
