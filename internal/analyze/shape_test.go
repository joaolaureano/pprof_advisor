package analyze

import "testing"

func TestTrimShape(t *testing.T) {
	cases := map[string]string{
		"store.(*cache[go.shape.interface { Key() string }]).lookup": "store.(*cache).lookup",
		"store.searchEntries[go.shape.interface { Key() string }]":   "store.searchEntries",
		"store.(*Cache).Lookup": "store.(*Cache).Lookup",
		"pkg.f[a[b]]":           "pkg.f",
		"pkg.unbalanced[oops":   "pkg.unbalanced[oops",
	}
	for in, want := range cases {
		if got := trimShape(in); got != want {
			t.Errorf("trimShape(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
