package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tinyModule writes a one-package module into a temp dir and returns its path.
// The source is chosen so the compiler must say something about it: a local
// whose address is returned cannot stay on the stack.
func tinyModule(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/tiny\n\ngo 1.26\n")
	write("tiny.go", body)
	return dir
}

const escapingSource = `package tiny

func Escaping() *int {
	x := 42
	return &x
}
`

func TestDetectDescribesTheToolchainThatWillCompile(t *testing.T) {
	info, err := Detect(context.Background(), ".")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !strings.HasPrefix(info.Version, "go") {
		t.Errorf("Version = %q, want something like go1.26.1", info.Version)
	}
	if !filepath.IsAbs(info.Path) {
		t.Errorf("Path = %q, want an absolute path; a relative one is meaningless in a report", info.Path)
	}
	if info.GOOS == "" || info.GOARCH == "" {
		t.Errorf("GOOS/GOARCH = %q/%q, want both populated", info.GOOS, info.GOARCH)
	}
	// Raw is the whole `go version` line, so it has to mention the version that
	// was reported separately — otherwise the two fields disagree about which
	// compiler ran.
	if !strings.Contains(info.Raw, info.Version) {
		t.Errorf("Raw = %q does not mention Version %q", info.Raw, info.Version)
	}
}

func TestBuildReturnsTheCompilerDiagnostics(t *testing.T) {
	dir := tinyModule(t, escapingSource)
	res, err := Build(context.Background(), BuildOptions{Dir: dir, Gcflags: "-m=2"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// The point of this package is the diagnostics. An empty stderr here means
	// the flag did not reach the compiler, which would make every report say a
	// project has no escapes.
	position := regexp.MustCompile(`(?m)^\./tiny\.go:\d+:\d+: `)
	if !position.Match(res.Stderr) {
		t.Fatalf("no position-prefixed diagnostic in stderr:\n%s", res.Stderr)
	}
	if res.Duration <= 0 {
		t.Error("Duration was not measured")
	}
	// The command is recorded so a run can be reproduced by hand; -o os.DevNull
	// is what keeps a binary out of the target repository.
	got := strings.Join(res.Command, " ")
	for _, want := range []string{"go build", "-o " + os.DevNull, "-gcflags=-m=2", "./..."} {
		if !strings.Contains(got, want) {
			t.Errorf("Command %q is missing %q", got, want)
		}
	}
}

func TestBuildWritesNoBinaryIntoTheTarget(t *testing.T) {
	dir := tinyModule(t, "package main\n\nfunc main() {}\n")
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), BuildOptions{Dir: dir, Gcflags: "-m=2"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A main package is the case where `go build` would otherwise drop an
	// executable next to the source. This tool reads other people's projects.
	if len(after) != len(before) {
		var names []string
		for _, e := range after {
			names = append(names, e.Name())
		}
		t.Errorf("build left files behind in the target: %v", names)
	}
}

func TestBuildFailsLoudlyOnATargetThatDoesNotCompile(t *testing.T) {
	dir := tinyModule(t, "package tiny\n\nfunc Broken() int { return \"not an int\" }\n")
	res, err := Build(context.Background(), BuildOptions{Dir: dir, Gcflags: "-m=2"})
	if err == nil {
		t.Fatal("a target that does not compile was reported as a successful build")
	}
	// Silence here would produce a report claiming the package has no escapes,
	// when in truth it was never analyzed. The reason has to survive.
	if !strings.Contains(err.Error(), "cannot use") {
		t.Errorf("error does not carry the compiler's complaint: %v", err)
	}
	if res == nil || len(res.Stderr) == 0 {
		t.Error("the captured output was discarded along with the failure")
	}
}

func TestBuildRejectsIncompleteOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts BuildOptions
		want string
	}{
		{"no dir", BuildOptions{Gcflags: "-m=2"}, "dir is required"},
		{"no gcflags", BuildOptions{Dir: "."}, "gcflags is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build(context.Background(), tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
}

func TestBuildDefaultsToTheWholeModule(t *testing.T) {
	dir := tinyModule(t, escapingSource)
	res, err := Build(context.Background(), BuildOptions{Dir: dir, Gcflags: "-m=2", Patterns: nil})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Command[len(res.Command)-1] != "./..." {
		t.Errorf("Command = %v, want it to end in ./...", res.Command)
	}
}
