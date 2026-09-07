package benchgen

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeCorpusSeedTuplesAndNativeValues(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		types, want   []string
		bad           bool
	}{
		{"mixed", "go test fuzz v1\nstring(\"hello\")\nint64(-42)\nbool(true)\n", []string{"string", "int64", "bool"}, []string{`"hello"`, "int64(-42)", "bool(true)"}, false},
		{"aliases", "go test fuzz v1\nbyte('K')\nrune('œ')\n", []string{"uint8", "int32"}, []string{"uint8(75)", "int32(339)"}, false},
		{"float_special", "go test fuzz v1\nfloat64(NaN)\nfloat32(-Inf)\nmath.Float64frombits(0x7ff8000000000001)\n", []string{"float64", "float32", "float64"}, []string{"math.NaN()", "float32(math.Inf(-1))", "math.Float64frombits(0x7ff8000000000001)"}, false},
		{"positive_infinity", "go test fuzz v1\nfloat64(+Inf)\n", []string{"float64"}, []string{"math.Inf(1)"}, false},
		{"wrong_arity", "go test fuzz v1\nstring(\"x\")\n", []string{"string", "int"}, nil, true},
		{"wrong_type", "go test fuzz v1\nint(1)\n", []string{"string"}, nil, true},
		{"bad_bits", "go test fuzz v1\nmath.Float32frombits(0x100000000)\n", []string{"float32"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := decodeCorpusSeed(tc.content, tc.types)
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			if !tc.bad && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestCorpusStableDeduplicatedByCompleteTuple(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadCorpus(dir, []string{"string", "int"}); err == nil {
		t.Fatal("accepted empty corpus")
	}
	files := map[string]string{
		"a": "go test fuzz v1\nstring(\"ab\")\nint(3)\n",
		"b": "go test fuzz v1\nstring(\"ab\")\nint(3)\n",
		"c": "go test fuzz v1\nstring(\"a\")\nint(23)\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	seeds, err := loadCorpus(dir, []string{"string", "int"})
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 2 || seeds[0].Hash >= seeds[1].Hash {
		t.Fatalf("bad seeds: %+v", seeds)
	}
	for _, seed := range seeds {
		if seed.Literals[0] == `"ab"` && len(seed.Origins) != 2 {
			t.Fatal("duplicate origins lost")
		}
	}
	again, err := loadCorpus(dir, []string{"string", "int"})
	if err != nil || !reflect.DeepEqual(seeds, again) {
		t.Fatal("unstable corpus")
	}
}
