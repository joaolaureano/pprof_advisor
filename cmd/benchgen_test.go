package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaolaureano/profadvisor/internal/benchgen"
)

func TestBenchgenCLI(t *testing.T) {
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	dir := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module example.com/target\n\ngo 1.24\n", "target.go": "package target\nfunc parse(s string) int { return len(s) }\n", "seeds/one": "go test fuzz v1\nstring(\"hello\")\n"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"benchgen", "--dir", dir, "--pkg", ".", "--func", "parse", "--corpus", filepath.Join(dir, "seeds")}
	code, out, diag := invoke(append(args, "--out", filepath.Join(dir, "artifacts"))...)
	if code != 0 || diag != "" {
		t.Fatalf("code=%d diagnostics=%s", code, diag)
	}
	var r benchgen.Result
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != 4 || !r.Generated || r.Validated || r.InstalledPath != "" {
		t.Fatalf("report=%+v", r)
	}
	code, out, diag = invoke(append(args, "--out", filepath.Join(dir, "text"), "--format", "text")...)
	if code != 0 || !strings.Contains(out, "Validated: false") {
		t.Fatalf("code=%d out=%s diagnostics=%s", code, out, diag)
	}
	code, out, diag = invoke(append(args, "--out", filepath.Join(dir, "artifacts"))...)
	if code != 1 || out != "" || diag == "" {
		t.Fatalf("collision: code=%d out=%s diagnostics=%s", code, out, diag)
	}
}

func TestBenchgenCLIRequiredFlags(t *testing.T) {
	code, out, diag := invoke("benchgen")
	if code != 1 || out != "" || !strings.Contains(diag, "required") {
		t.Fatalf("code=%d out=%s diagnostics=%s", code, out, diag)
	}
}

func TestBenchgenCLIInterfaceParameter(t *testing.T) {
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module example.com/target\n\ngo 1.24\n",
		"target.go": "package target\n" +
			"type Reader interface { Read([]byte) (int, error) }\n" +
			"type MyReader struct{}\n" +
			"func (MyReader) Read(b []byte) (int, error) { return len(b), nil }\n" +
			"func processReader(s string, r Reader) int { return len(s) }\n",
		"seeds/one": "go test fuzz v1\nstring(\"hello\")\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"benchgen", "--dir", dir, "--pkg", ".", "--func", "processReader", "--corpus", filepath.Join(dir, "seeds"), "--out", filepath.Join(dir, "artifacts"), "--impl", "Reader=MyReader"}
	code, out, diag := invoke(args...)
	if code != 0 || diag != "" {
		t.Fatalf("code=%d diagnostics=%s", code, diag)
	}
	var r benchgen.Result
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Manifest.Target.Implementations) != 1 {
		t.Fatalf("expected 1 implementation, got %d: %v", len(r.Manifest.Target.Implementations), r.Manifest.Target.Implementations)
	}
	if r.Manifest.Target.Implementations[0] != "Reader=MyReader" {
		t.Fatalf("got %q, want %q", r.Manifest.Target.Implementations[0], "Reader=MyReader")
	}
}

func TestBenchgenCLIImplFlagValidation(t *testing.T) {
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":    "module example.com/target\n\ngo 1.24\n",
		"target.go": "package target\nfunc parse(s string) int { return len(s) }\n",
		"seeds/one": "go test fuzz v1\nstring(\"hello\")\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"benchgen", "--dir", dir, "--pkg", ".", "--func", "parse", "--corpus", filepath.Join(dir, "seeds"), "--out", filepath.Join(dir, "artifacts"), "--impl", "malformed"}
	code, out, diag := invoke(args...)
	if code != 1 || out != "" {
		t.Fatalf("expected non-zero exit and no stdout; got code=%d out=%s diagnostics=%s", code, out, diag)
	}
	if !strings.Contains(diag, "=") {
		t.Fatalf("diagnostics should mention the expected format; got %s", diag)
	}
}

func TestBenchgenCLIImplFlagRepeatable(t *testing.T) {
	t.Setenv("PROFADVISOR_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module example.com/target\n\ngo 1.24\n",
		"target.go": "package target\n" +
			"type Reader interface { Read([]byte) (int, error) }\n" +
			"type Writer interface { Write([]byte) (int, error) }\n" +
			"type MyReader struct{}\n" +
			"func (MyReader) Read(b []byte) (int, error) { return len(b), nil }\n" +
			"type MyWriter struct{}\n" +
			"func (MyWriter) Write(b []byte) (int, error) { return len(b), nil }\n" +
			"func processInterfaces(s string, r Reader, w Writer) int { return len(s) }\n",
		"seeds/one": "go test fuzz v1\nstring(\"test\")\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"benchgen", "--dir", dir, "--pkg", ".", "--func", "processInterfaces", "--corpus", filepath.Join(dir, "seeds"), "--out", filepath.Join(dir, "artifacts"), "--impl", "Reader=MyReader", "--impl", "Writer=MyWriter"}
	code, out, diag := invoke(args...)
	if code != 0 || diag != "" {
		t.Fatalf("code=%d diagnostics=%s", code, diag)
	}
	var r benchgen.Result
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Manifest.Target.Implementations) != 2 {
		t.Fatalf("expected 2 implementations, got %d: %v", len(r.Manifest.Target.Implementations), r.Manifest.Target.Implementations)
	}
	if r.Manifest.Target.Implementations[0] != "Reader=MyReader" {
		t.Fatalf("got %q, want %q", r.Manifest.Target.Implementations[0], "Reader=MyReader")
	}
	if r.Manifest.Target.Implementations[1] != "Writer=MyWriter" {
		t.Fatalf("got %q, want %q", r.Manifest.Target.Implementations[1], "Writer=MyWriter")
	}
}
