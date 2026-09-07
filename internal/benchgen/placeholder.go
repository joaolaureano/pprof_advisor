package benchgen

import (
	"strconv"
	"strings"
)

// mapPlaceholders rewrites every $N placeholder in expression using fn.
//
// It scans left to right and never re-reads what it has written. A sequence of
// strings.ReplaceAll calls cannot promise that when the replacement itself
// contains a placeholder: each call writes new $N text into the string the next
// call searches, so past nine placeholders "$1" starts matching the prefix of a
// "$12" an earlier call produced. That is how a twelve-field struct sitting
// behind another argument generated a harness referring to identifiers that do
// not exist.
//
// fn receives the placeholder's number and returns its replacement; a false
// second result leaves the placeholder as it was.
func mapPlaceholders(expression string, fn func(n int) (string, bool)) string {
	var out strings.Builder
	for i := 0; i < len(expression); {
		if expression[i] != '$' {
			out.WriteByte(expression[i])
			i++
			continue
		}
		j := i + 1
		for j < len(expression) && expression[j] >= '0' && expression[j] <= '9' {
			j++
		}
		// A lone '$', or a digit run too long for an int, is not a placeholder
		// this package produced, so it passes through as written.
		n, err := strconv.Atoi(expression[i+1 : j])
		if j == i+1 || err != nil {
			out.WriteString(expression[i:j])
			i = j
			continue
		}
		if replacement, ok := fn(n); ok {
			out.WriteString(replacement)
		} else {
			out.WriteString(expression[i:j])
		}
		i = j
	}
	return out.String()
}
