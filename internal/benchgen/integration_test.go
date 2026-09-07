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
