package benchgen

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateCodeFrozenAndStable(t *testing.T) {
	for _, kind := range []string{"string", "[]byte", "bool", "int64", "uint32", "float64"} {
		t.Run(kind, func(t *testing.T) {
			target := Target{Name: "sample", Function: "profadvisorInput", InputType: kind, FuzzName: "FuzzSample", BenchmarkName: "BenchmarkSample"}
			seeds := []Seed{{Hash: "bb", Data: []byte{0, 255, '\n', '"'}, Literal: scalarTestLiteral(kind, "2.5")}, {Hash: "aa", Data: []byte("hello"), Literal: scalarTestLiteral(kind, "1")}}
			code, err := generateCode(target, seeds)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "generated_test.go", code, parser.AllErrors); err != nil {
				t.Fatal(err)
			}
			seeds[0], seeds[1] = seeds[1], seeds[0]
			again, err := generateCode(target, seeds)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(code, again) {
				t.Fatal("code changes with seed order")
			}
			if !strings.Contains(string(code), "profadvisorInput(profadvisorInput_)") {
				t.Fatalf("target shadowed: %s", code)
			}
			if strings.Count(string(code), ".Add(") != 2 || strings.Count(string(code), ".Run(") != 2 {
				t.Fatalf("missing seeds: %s", code)
			}
		})
	}
}

func scalarTestLiteral(kind, value string) string {
	if kind == "string" || kind == "[]byte" {
		return ""
	}
	if kind == "bool" {
		return "bool(true)"
	}
	return kind + "(" + value + ")"
}
