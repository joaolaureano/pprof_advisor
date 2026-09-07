package measurement

import (
	"testing"
)

func TestTrimShape(t *testing.T) {
	cases := map[string]string{
		"store.(*cache[go.shape.interface { Key() string }]).lookup": "store.(*cache).lookup",
		"store.searchEntries[go.shape.interface { Key() string }]":   "store.searchEntries",
		"store.(*Cache).Lookup": "store.(*Cache).Lookup",
		"pkg.f[a[b]]":           "pkg.f",
		"pkg.unbalanced[oops":   "pkg.unbalanced[oops",
	}
	for in, want := range cases {
		if got := TrimShape(in); got != want {
			t.Errorf("TrimShape(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
