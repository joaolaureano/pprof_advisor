package measurement

// These render a measured value the way a reader expects to see it. They live
// here, rather than in a formatting package, for the reason the cycle made
// obvious: both the prompt builder and the CLI's text output need them, and
// this package is the one that already knows what a profile's numbers mean and
// imports nothing that could depend on either.

import (
	"fmt"
	"strconv"
	"strings"
)

// TrimShape strips the generic instantiation the Go compiler bakes into a
// profile's function names. A method on a generic type comes back as
//
//	store.(*cache[go.shape.interface { Key() string }]).lookup
//
// which costs real tokens and tells the model nothing it cannot see in the
// source. The bracketed part is dropped and the name reads as written.
func TrimShape(name string) string {
	open := strings.Index(name, "[")
	if open < 0 {
		return name
	}
	depth, i := 0, open
	for ; i < len(name); i++ {
		switch name[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return name[:open] + name[i+1:]
			}
		}
	}
	return name
}

// Coster returns the renderer for the profile's sample unit. A profile reader
// expects nanoseconds as "1.20ms" and bytes as "4.0 MB"; printing either as a
// bare integer wastes the model's attention on arithmetic.
func Coster(cfg Config) func(int64) string {
	switch cfg.SampleUnit {
	case "bytes":
		return BytesCost
	case "count":
		return func(n int64) string { return fmt.Sprintf("%d allocs", n) }
	default:
		return Nanos
	}
}

// BytesCost renders a byte count with appropriate units.
func BytesCost(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// Nanos renders a nanosecond count the way a profile reader expects to see it.
func Nanos(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.2fs", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.0fms", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0fµs", float64(n)/1e3)
	default:
		return fmt.Sprintf("%dns", n)
	}
}

// Pct calculates a percentage in a zero-safe manner.
func Pct(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

// Dec1 formats a float64 with 1 decimal place, matching the %.1f format
// the prompts were written against.
func Dec1(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
