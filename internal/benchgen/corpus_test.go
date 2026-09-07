package benchgen

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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
		{"float_special", "go test fuzz v1\nfloat64(NaN)\nfloat32(-Inf)\nmath.Float64frombits(0x7ff8000000000001)\n", []string{"float64", "float32", "float64"}, []string{mathPlaceholder + "NaN()", "float32(" + mathPlaceholder + "Inf(-1))", mathPlaceholder + "Float64frombits(0x7ff8000000000001)"}, false},
		{"positive_infinity", "go test fuzz v1\nfloat64(+Inf)\n", []string{"float64"}, []string{mathPlaceholder + "Inf(1)"}, false},
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

func TestStringWithMathPlaceholder(t *testing.T) {
	// A string seed whose content is literally "math." should be preserved
	// as-is and not rewritten to profadvisorMath. This verifies that the
	// placeholder sentinel is used instead of plain string matching.
	for _, tc := range []struct {
		name, content string
		want          string
	}{
		{"math dot in string", "go test fuzz v1\nstring(\"math.Pi rounds\")\n", `"math.Pi rounds"`},
		{"multiple occurrences", "go test fuzz v1\nstring(\"math.sin and math.cos\")\n", `"math.sin and math.cos"`},
		{"empty string", "go test fuzz v1\nstring(\"\")\n", `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := decodeCorpusSeed(tc.content, []string{"string"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got[0] != tc.want {
				t.Fatalf("got literal %q, want %q", got[0], tc.want)
			}
		})
	}
}

func TestIntOutOfHostWordSize(t *testing.T) {
	// An int seed outside the host word size must be rejected, not silently
	// wrapped. This test adapts to the host's IntSize.
	var outOfRange string
	if strconv.IntSize == 64 {
		// On 64-bit systems, use a value too large for int
		outOfRange = "9223372036854775808" // math.MaxInt64 + 1
	} else {
		// On 32-bit systems, use a value too large for int
		outOfRange = "2147483648" // math.MaxInt32 + 1
	}
	content := "go test fuzz v1\nint(" + outOfRange + ")\n"
	_, _, err := decodeCorpusSeed(content, []string{"int"})
	if err == nil {
		t.Fatalf("accepted out-of-range int value on %d-bit system: %s", strconv.IntSize, outOfRange)
	}
}

func TestLoadCorpusSkipsDotfilesAndDirectories(t *testing.T) {
	// loadCorpus should skip dotfiles and directories without failing,
	// but still load real seed files and fail on malformed seeds.
	dir := t.TempDir()

	// Create a real seed file
	validSeed := "go test fuzz v1\nstring(\"valid\")\n"
	if err := os.WriteFile(filepath.Join(dir, "valid_seed"), []byte(validSeed), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a dotfile (should be skipped)
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("ignore me"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory (should be skipped)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	seeds, err := loadCorpus(dir, []string{"string"})
	if err != nil {
		t.Fatalf("unexpected error loading valid corpus with dotfile and directory: %v", err)
	}
	if len(seeds) != 1 {
		t.Fatalf("got %d seeds, want 1", len(seeds))
	}
	if seeds[0].Literals[0] != `"valid"` {
		t.Fatalf("wrong seed loaded: %q", seeds[0].Literals[0])
	}

	// Now test that a malformed seed file still causes an error
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "malformed"), []byte("this is not a seed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = loadCorpus(dir2, []string{"string"})
	if err == nil {
		t.Fatal("accepted malformed seed file")
	}
}
