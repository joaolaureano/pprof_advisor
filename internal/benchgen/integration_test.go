package benchgen

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaolaureano/profadvisor/internal/verify"
)

func integrationFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func integrationTarget(t *testing.T, source string, seeds ...string) Options {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "target")
	corpus := filepath.Join(root, "corpus")
	for _, path := range []string{dir, corpus} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	integrationFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixture\n\ngo 1.24\n")
	integrationFile(t, filepath.Join(dir, "target.go"), "package fixture\n"+source)
	for i, seed := range seeds {
		integrationFile(t, filepath.Join(corpus, string(rune('a'+i))), "go test fuzz v1\n"+seed+"\n")
	}
	return Options{Dir: dir, Package: ".", Function: "process", Corpus: corpus, Out: filepath.Join(root, "out"), Write: true}
}

func integrationGo(t *testing.T, dir string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

func TestIntegrationGeneratedHarness(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		seeds        []string
	}{
		{"string", "import \"errors\"\nfunc process(s string) (int, error) { return len(s), errors.New(\"expected invalid input\") }", []string{`string("hello")`, `string("\x00\xff")`}},
		{"bytes", "func process(b []byte) int { n := 0; for _, v := range b { n += int(v) }; return n }", []string{`[]byte("hello")`, `[]byte("\x00\xff")`}},
		{"integer", "func process(n int64) int64 { return n * 2 }", []string{`int64(-3)`, `int64(17)`}},
		{"float", "func process(v float64) float64 { return v / 2 }", []string{`float64(-1.5)`, `float64(4.25)`}},
		{"bool", "func process(v bool) bool { return !v }", []string{`bool(true)`, `bool(false)`}},
		{"tuple", "func process(s string, n int64, ok bool) int { if ok { return len(s)+int(n) }; return len(s)-int(n) }", []string{"string(\"hello\")\nint64(-3)\nbool(true)", "string(\"bye\")\nint64(7)\nbool(false)"}},
		{"struct", "type limits struct { Count int64; Ready bool }\ntype request struct { Name string; Limits limits; Data []byte }\nfunc process(r request, suffix string) int { if r.Limits.Ready { return len(r.Name) + int(r.Limits.Count) + len(r.Data) + len(suffix) }; return 0 }", []string{"string(\"hello\")\nint64(3)\nbool(true)\n[]byte(\"data\")\nstring(\"!\")", "string(\"bye\")\nint64(7)\nbool(false)\n[]byte(\"x\")\nstring(\"?\")"}},
		{"byte_rune", "func process(b byte, r rune) int32 { return int32(b) + r }", []string{"byte('K')\nrune('œ')", "uint8(255)\nint32(-1)"}},
		{"float_special", "func process(v float64, n int) float64 { return v + float64(n) }", []string{"float64(NaN)\nint(1)", "math.Float64frombits(0x7ff8000000000001)\nint(-2)"}},
		{"wide_struct", "type wide struct { F0 int64; F1 int64; F2 int64; F3 int64; F4 int64; F5 int64; F6 int64; F7 int64; F8 int64; F9 int64; F10 int64; F11 int64 }\nfunc process(a int64, w wide) int64 { return a + w.F0 + w.F11 }", []string{"int64(1)\nint64(10)\nint64(11)\nint64(12)\nint64(13)\nint64(14)\nint64(15)\nint64(16)\nint64(17)\nint64(18)\nint64(19)\nint64(20)\nint64(21)", "int64(2)\nint64(30)\nint64(31)\nint64(32)\nint64(33)\nint64(34)\nint64(35)\nint64(36)\nint64(37)\nint64(38)\nint64(39)\nint64(40)\nint64(41)"}},
		{"interface_param", "type Source interface { Bytes() []byte }\ntype buffer struct { Data []byte; Offset int }\nfunc (b *buffer) Bytes() []byte { return b.Data }\nfunc process(s Source, n int) int { return len(s.Bytes()) + n }", []string{"[]byte(\"hello\")\nint(10)\nint(5)", "[]byte(\"data\")\nint(20)\nint(15)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := integrationTarget(t, tc.source, tc.seeds...)
			result, err := Generate(context.Background(), opts)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Generated || result.Validated || result.InstalledPath == "" {
				t.Fatalf("unexpected generation status: %+v", result)
			}
			if output, err := integrationGo(t, opts.Dir, "test", "-run", "^"+result.Manifest.Target.FuzzName+"$", "."); err != nil {
				t.Fatalf("seed replay: %v\n%s", err, output)
			}
			output, err := integrationGo(t, opts.Dir, "test", "-run", "^$", "-bench", "^"+result.Manifest.Target.BenchmarkName+"$", "-benchtime=1x", "-count=2", ".")
			if err != nil {
				t.Fatalf("benchmark: %v\n%s", err, output)
			}
			comparison, err := verify.FromReaders(bytes.NewReader(output), "baseline", bytes.NewReader(output), "after", verify.Options{})
			if err != nil {
				t.Fatalf("benchmark reader: %v\n%s", err, output)
			}
			if len(comparison.Comparisons) != len(tc.seeds)*3 {
				t.Fatalf("got %d metric comparisons, want %d\n%s", len(comparison.Comparisons), len(tc.seeds)*3, output)
			}
		})
	}
}

func TestIntegrationPanicOnlyFailsReplay(t *testing.T) {
	opts := integrationTarget(t, `func process(s string) { panic("seed failure") }`, `string("trigger")`)
	result, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("generation must not execute target: %v", err)
	}
	output, err := integrationGo(t, opts.Dir, "test", "-run", "^"+result.Manifest.Target.FuzzName+"$", ".")
	if err == nil || !strings.Contains(string(output), "seed failure") {
		t.Fatalf("expected panic on replay, got %v\n%s", err, output)
	}
}

func TestIntegrationArtifactsReproducible(t *testing.T) {
	opts := integrationTarget(t, `func process(s string) int { return len(s) }`, `string("second")`, `string("first")`, `string("second")`)
	opts.Write = false
	first, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate directory entries in reverse order without changing their origins.
	for _, name := range []string{"c", "b", "a"} {
		path := filepath.Join(opts.Corpus, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		integrationFile(t, path, string(data))
	}
	opts.Out += "-second"
	second, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Manifest.Seeds) != 2 {
		t.Fatalf("duplicates retained: %+v", first.Manifest.Seeds)
	}
	for _, paths := range [][2]string{{first.CodePath, second.CodePath}, {first.ManifestPath, second.ManifestPath}} {
		a, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(paths[1])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("artifacts differ: %s and %s", paths[0], paths[1])
		}
	}
	before, err := os.ReadFile(second.CodePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(context.Background(), opts); err == nil {
		t.Fatal("existing output accepted")
	}
	after, err := os.ReadFile(second.CodePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing output modified")
	}
}

func TestIntegrationSymbolCollision(t *testing.T) {
	opts := integrationTarget(t, `func process(s string) {}`, `string("seed")`)
	opts.Write = false
	result, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	integrationFile(t, filepath.Join(opts.Dir, "existing_test.go"), "package fixture\nimport \"testing\"\nfunc "+result.Manifest.Target.FuzzName+"(f *testing.F) {}\n")
	opts.Out += "-collision"
	if _, err := Generate(context.Background(), opts); err == nil {
		t.Fatal("existing fuzz symbol accepted")
	}
}

func TestIntegrationRecordInTargetModule(t *testing.T) {
	// Test that writing the record (--out) into a subdirectory of the target
	// module does not break go vet, because the record has a build constraint.
	opts := integrationTarget(t, `func process(s string) int { return len(s) }`, `string("test")`)
	opts.Write = false
	// Point --out to a subdirectory within the target module, simulating the
	// recommended practice of committing generated artifacts.
	opts.Out = filepath.Join(opts.Dir, "artifacts")
	result, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Verify that go vet still succeeds in the target module despite the record.
	output, err := integrationGo(t, opts.Dir, "vet", "./...")
	if err != nil {
		t.Fatalf("go vet failed in target module with record present: %v\n%s", err, output)
	}

	// Verify that go build still succeeds in the target module.
	output, err = integrationGo(t, opts.Dir, "build", "./...")
	if err != nil {
		t.Fatalf("go build failed in target module with record present: %v\n%s", err, output)
	}

	// Verify that the record exists and has the build constraint.
	recordContent, err := os.ReadFile(result.CodePath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	const prefix = "//go:build ignore\n\n"
	if !bytes.HasPrefix(recordContent, []byte(prefix)) {
		t.Fatalf("record must start with build constraint. Got: %q", recordContent[:len(prefix)])
	}
}
