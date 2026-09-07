package benchgen

import (
	"bytes"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateCodeForTuple(t *testing.T) {
	target := Target{Name: "sample", Function: "process", InputTypes: []string{"string", "int64", "float64"}, FuzzName: "FuzzSample", BenchmarkName: "BenchmarkSample"}
	// The literal with NaN will have the placeholder; generateCode will replace it with the alias
	seeds := []Seed{{Hash: "bb", Literals: []string{`"hello"`, "int64(-2)", mathPlaceholder + "NaN()"}}, {Hash: "aa", Literals: []string{`"bye"`, "int64(1)", "float64(2.5)"}}}
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

func TestRenderCallArgumentsKeepsTwoDigitPlaceholdersDistinct(t *testing.T) {
	arguments := make([]string, 11)
	for i := range arguments {
		arguments[i] = "input" + strconv.Itoa(i)
	}
	got, err := renderCallArguments([]string{"Input{First: $0, Eleventh: $10}"}, arguments)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Input{First: input0, Eleventh: input10}"; !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}

func TestStringContainingMathDotSurvivesGeneration(t *testing.T) {
	// A string seed whose content contains "math." must not be rewritten.
	// This verifies that the placeholder mechanism correctly prevents corruption.
	target := Target{Name: "sample", Function: "process", InputTypes: []string{"string"}, FuzzName: "FuzzSample", BenchmarkName: "BenchmarkSample"}
	seeds := []Seed{{Hash: "aa", Literals: []string{`"math.Pi rounds"`}}}
	code, err := generateCode(target, seeds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := string(code)

	// The generated code must contain the exact literal without rewriting
	if !strings.Contains(text, `"math.Pi rounds"`) {
		t.Fatalf("exact literal not found in generated code:\n%s", text)
	}

	// Must not contain the aliased variant
	if strings.Contains(text, "profadvisorMath.Pi") {
		t.Fatalf("string was incorrectly rewritten to profadvisorMath")
	}

	// Must not import math at all (no F.Add call needs it)
	if strings.Contains(text, `"math"`) {
		t.Fatalf("unexpected math import for string-only seed")
	}

	// Verify the generated code is valid Go
	if _, err := parser.ParseFile(token.NewFileSet(), "generated_test.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated code does not parse: %v", err)
	}
}

func TestFloatNaNGenerationWithMathImport(t *testing.T) {
	// A float64 NaN seed must still produce correct code with the math import
	// and aliased math.NaN() call, and must not contain NUL bytes.
	target := Target{Name: "sample", Function: "process", InputTypes: []string{"float64"}, FuzzName: "FuzzSample", BenchmarkName: "BenchmarkSample"}
	// The literal contains the placeholder that will be replaced during code generation
	seeds := []Seed{{Hash: "aa", Literals: []string{mathPlaceholder + "NaN()"}}}
	code, err := generateCode(target, seeds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := string(code)

	// Must import math under an alias
	if !strings.Contains(text, `"math"`) {
		t.Fatalf("math import missing from generated code")
	}

	// Must contain the aliased NaN call with the placeholder replaced
	if !strings.Contains(text, "profadvisorMath.NaN()") {
		t.Fatalf("aliased math.NaN() not found in generated code:\n%s", text)
	}

	// Must not contain any NUL bytes (the placeholder must have been fully replaced)
	if strings.ContainsRune(text, 0) {
		t.Fatalf("NUL byte found in generated code, placeholder not fully substituted")
	}

	// Verify the generated code is valid Go
	if _, err := parser.ParseFile(token.NewFileSet(), "generated_test.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated code does not parse: %v", err)
	}
}
