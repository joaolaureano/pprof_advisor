package extract

import (
	"testing"

	"github.com/joaolaureano/profadvisor/internal/fixture"
)

// BenchmarkFromProfile measures the performance of extracting hotspots from
// a profile at two different result sizes.
//
// The benchmark loads the fixture profile once outside the loop, since
// fixture.Load rewrites fn.Filename in place and reloading per iteration would
// measure gzip and protobuf decoding instead of the extract algorithm itself.
//
// This benchmark includes filesystem I/O: sourceExcerpt reads the source file
// from disk once per hotspot (page-cache-warm, but present). The tool's own
// guidance warns against I/O-bound targets, so this number should be read as
// throughput of the whole extract stage rather than as pure CPU. The I/O is
// inherent to the feature: the tool renders source excerpts to the prompt.
func BenchmarkFromProfile(b *testing.B) {
	// Load the profile once.
	p, err := fixture.Load("all.prof")
	if err != nil {
		b.Fatalf("fixture load: %v", err)
	}

	cases := []struct {
		name string
		topN int
	}{
		{"TopN=3", 3},
		{"TopN=10", 10},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			// Benchmark FromProfile.
			for b.Loop() {
				_, err := FromProfile(p, "all.prof", Options{TopN: tc.topN})
				if err != nil {
					b.Fatalf("extract: %v", err)
				}
			}
		})
	}
}
