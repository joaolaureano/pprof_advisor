package benchgen

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateCodeForTuple(t *testing.T) {
	target := Target{Name: "sample", Function: "process", InputTypes: []string{"string", "int64", "float64"}, FuzzName: "FuzzSample", BenchmarkName: "BenchmarkSample"}
	seeds := []Seed{{Hash: "bb", Literals: []string{`"hello"`, "int64(-2)", "math.NaN()"}}, {Hash: "aa", Literals: []string{`"bye"`, "int64(1)", "float64(2.5)"}}}
	code, err := generateCode(target, seeds)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated_test.go", code, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	text := string(code)
	for _, fragment := range []string{"\"math\"", "profadvisorF.Add(\"bye\", int64(1), float64(2.5))", "func(profadvisorT *profadvisorTesting.T, profadvisorInput1 string, profadvisorInput2 int64, profadvisorInput3 float64)", "process(profadvisorInput1, profadvisorInput2, profadvisorInput3)"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing %q:\n%s", fragment, text)
		}
	}
	seeds[0], seeds[1] = seeds[1], seeds[0]
	again, err := generateCode(target, seeds)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code, again) {
		t.Fatal("code changes with seed order")
	}
}

func TestGenerateRejectsWrongTupleArity(t *testing.T) {
	_, err := generateCode(Target{Name: "sample", Function: "process", InputTypes: []string{"string", "bool"}}, []Seed{{Hash: "x", Literals: []string{`"one"`}}})
	if err == nil {
		t.Fatal("accepted short tuple")
	}
}
