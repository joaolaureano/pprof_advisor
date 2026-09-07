package benchgen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// mathPlaceholder stands in for the math package qualifier in a generated
// literal. The generated file imports math under an alias chosen at generation
// time, so the qualifier cannot be written here; and a plain "math." would be
// indistinguishable from the same text occurring inside a string seed, which is
// how this corrupted corpus data. A NUL byte cannot appear in the output of
// strconv.Quote, which escapes control characters, so this sentinel cannot
// collide with seed content.
const mathPlaceholder = "\x00math."

// loadCorpus accepts the native Go fuzz v1 format. Each non-empty line after
// the header is one argument, so one file is one complete argument tuple.
func loadCorpus(dir string, inputTypes []string) ([]Seed, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read corpus: %w", err)
	}
	byHash := map[string]*Seed{}
	for _, entry := range entries {
		// Skip dotfiles (tooling debris like .DS_Store) and directories
		// without failing, but do fail for malformed seed files since those
		// represent data loss and must not be silently ignored.
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read corpus %s: %w", path, err)
		}
		literals, tuple, err := decodeCorpusSeed(string(content), inputTypes)
		if err != nil {
			return nil, fmt.Errorf("corpus %s: %w", path, err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(tuple))
		if seed, ok := byHash[hash]; ok {
			seed.Origins = append(seed.Origins, path)
		} else {
			byHash[hash] = &Seed{Hash: hash, Origins: []string{path}, Literals: literals}
		}
	}
	if len(byHash) == 0 {
		return nil, fmt.Errorf("corpus is empty")
	}
	seeds := make([]Seed, 0, len(byHash))
	for _, seed := range byHash {
		sort.Strings(seed.Origins)
		seeds = append(seeds, *seed)
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].Hash < seeds[j].Hash })
	return seeds, nil
}

func decodeCorpusSeed(content string, inputTypes []string) ([]string, []byte, error) {
	if len(inputTypes) == 0 {
		return nil, nil, fmt.Errorf("target has no fuzz arguments")
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSuffix(lines[0], "\r") != "go test fuzz v1" {
		return nil, nil, fmt.Errorf("expected go test fuzz v1 header")
	}
	var values []string
	for _, line := range lines[1:] {
		if line = strings.TrimSpace(line); line != "" {
			values = append(values, line)
		}
	}
	if len(values) != len(inputTypes) {
		return nil, nil, fmt.Errorf("got %d corpus arguments, want %d", len(values), len(inputTypes))
	}
	literals := make([]string, len(values))
	for i, value := range values {
		expr, err := parser.ParseExpr(value)
		if err != nil {
			return nil, nil, fmt.Errorf("argument %d: %w", i+1, err)
		}
		literal, err := corpusLiteral(expr, inputTypes[i])
		if err != nil {
			return nil, nil, fmt.Errorf("argument %d: %w", i+1, err)
		}
		literals[i] = literal
	}
	return literals, tupleBytes(inputTypes, literals), nil
}

// tupleBytes is length-delimited so type/value boundaries cannot collide.
func tupleBytes(types, literals []string) []byte {
	var b bytes.Buffer
	for i := range types {
		fmt.Fprintf(&b, "%d:%s%d:%s", len(types[i]), types[i], len(literals[i]), literals[i])
	}
	return b.Bytes()
}

func corpusLiteral(expr ast.Expr, inputType string) (string, error) {
	if inputType == "[]byte" {
		return byteSliceLiteral(expr)
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return "", fmt.Errorf("expected %s literal conversion", inputType)
	}
	if special, ok, err := floatBitsLiteral(call, inputType); ok || err != nil {
		return special, err
	}
	name, ok := call.Fun.(*ast.Ident)
	if !ok || !matchesCorpusType(name.Name, inputType) {
		return "", fmt.Errorf("expected %s literal conversion", inputType)
	}
	arg := call.Args[0]
	if inputType == "string" {
		return stringLiteral(arg)
	}
	if inputType == "bool" {
		return boolLiteral(arg)
	}
	if inputType == "uint8" || inputType == "int32" {
		if literal, ok, err := charLiteral(arg, inputType); ok || err != nil {
			return literal, err
		}
	}
	return numericCorpusLiteral(arg, inputType)
}

func matchesCorpusType(got, want string) bool {
	return got == want || (want == "uint8" && got == "byte") || (want == "int32" && got == "rune")
}

func byteSliceLiteral(expr ast.Expr) (string, error) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return "", fmt.Errorf("expected []byte string conversion")
	}
	typ, ok := call.Fun.(*ast.ArrayType)
	if !ok || typ.Len != nil {
		return "", fmt.Errorf("expected []byte string conversion")
	}
	name, ok := typ.Elt.(*ast.Ident)
	if !ok || name.Name != "byte" {
		return "", fmt.Errorf("expected []byte string conversion")
	}
	value, err := stringLiteralValue(call.Args[0])
	if err != nil {
		return "", err
	}
	return "[]byte(" + strconv.Quote(value) + ")", nil
}

func stringLiteral(expr ast.Expr) (string, error) {
	value, err := stringLiteralValue(expr)
	if err != nil {
		return "", err
	}
	return strconv.Quote(value), nil
}

func stringLiteralValue(expr ast.Expr) (string, error) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", fmt.Errorf("expected string literal")
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", fmt.Errorf("invalid string literal: %w", err)
	}
	return value, nil
}

func boolLiteral(expr ast.Expr) (string, error) {
	v, ok := expr.(*ast.Ident)
	if !ok || (v.Name != "true" && v.Name != "false") {
		return "", fmt.Errorf("expected bool literal")
	}
	return "bool(" + v.Name + ")", nil
}

func charLiteral(expr ast.Expr, inputType string) (string, bool, error) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.CHAR {
		return "", false, nil
	}
	if len(lit.Value) < 2 {
		return "", true, fmt.Errorf("invalid character literal")
	}
	r, _, _, err := strconv.UnquoteChar(lit.Value[1:len(lit.Value)-1], '\'')
	if err != nil {
		return "", true, fmt.Errorf("invalid character literal: %w", err)
	}
	if inputType == "uint8" {
		if r > 255 {
			return "", true, fmt.Errorf("byte literal is out of range")
		}
		return "uint8(" + strconv.FormatUint(uint64(r), 10) + ")", true, nil
	}
	return "int32(" + strconv.FormatInt(int64(r), 10) + ")", true, nil
}

func numericCorpusLiteral(expr ast.Expr, inputType string) (string, error) {
	text, kind, err := numericText(expr)
	if err != nil {
		return "", err
	}
	if inputType == "float32" || inputType == "float64" {
		return floatLiteral(text, kind, inputType)
	}
	if kind != token.INT {
		return "", fmt.Errorf("expected integer %s literal", inputType)
	}
	if strings.HasPrefix(inputType, "uint") {
		if strings.HasPrefix(text, "-") {
			return "", fmt.Errorf("invalid unsigned %s literal", inputType)
		}
		bits := integerBits(inputType)
		v, err := strconv.ParseUint(text, 0, bits)
		if err != nil {
			return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
		}
		if inputType == "uint" {
			return "uint(uint64(" + strconv.FormatUint(v, 10) + "))", nil
		}
		return inputType + "(" + strconv.FormatUint(v, 10) + ")", nil
	}
	// An int seed is only meaningful at the word size that produced it. The
	// host's strconv.IntSize is the only sensible width, so using it directly
	// avoids silently accepting values the target's int cannot hold. This will
	// fail at generation time with a clear message rather than producing a
	// harness that does not compile.
	bits := integerBits(inputType)
	v, err := strconv.ParseInt(text, 0, bits)
	if err != nil {
		return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
	}
	if inputType == "int" {
		return "int(int64(" + strconv.FormatInt(v, 10) + "))", nil
	}
	return inputType + "(" + strconv.FormatInt(v, 10) + ")", nil
}

func numericText(expr ast.Expr) (string, token.Token, error) {
	negative := false
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		switch x := unary.X.(type) {
		case *ast.BasicLit:
			if unary.Op != token.SUB {
				return "", token.ILLEGAL, fmt.Errorf("expected numeric literal")
			}
			negative, expr = true, x
		case *ast.Ident:
			if x.Name != "Inf" || (unary.Op != token.ADD && unary.Op != token.SUB) {
				return "", token.ILLEGAL, fmt.Errorf("expected numeric literal")
			}
			if unary.Op == token.SUB {
				return "-Inf", token.FLOAT, nil
			}
			return "+Inf", token.FLOAT, nil
		default:
			return "", token.ILLEGAL, fmt.Errorf("expected numeric literal")
		}
	}
	if id, ok := expr.(*ast.Ident); ok && id.Name == "NaN" {
		return "NaN", token.FLOAT, nil
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || (lit.Kind != token.INT && lit.Kind != token.FLOAT) {
		return "", token.ILLEGAL, fmt.Errorf("expected numeric literal")
	}
	if negative {
		return "-" + lit.Value, lit.Kind, nil
	}
	return lit.Value, lit.Kind, nil
}

func floatLiteral(text string, kind token.Token, inputType string) (string, error) {
	if kind != token.INT && kind != token.FLOAT {
		return "", fmt.Errorf("expected float %s literal", inputType)
	}
	if text == "NaN" {
		if inputType == "float32" {
			return "float32(" + mathPlaceholder + "NaN())", nil
		}
		return mathPlaceholder + "NaN()", nil
	}
	if text == "+Inf" {
		if inputType == "float32" {
			return "float32(" + mathPlaceholder + "Inf(1))", nil
		}
		return mathPlaceholder + "Inf(1)", nil
	}
	if text == "-Inf" {
		if inputType == "float32" {
			return "float32(" + mathPlaceholder + "Inf(-1))", nil
		}
		return mathPlaceholder + "Inf(-1)", nil
	}
	bits := 64
	if inputType == "float32" {
		bits = 32
	}
	v, err := strconv.ParseFloat(text, bits)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
	}
	return inputType + "(" + strconv.FormatFloat(v, 'g', -1, bits) + ")", nil
}

func floatBitsLiteral(call *ast.CallExpr, inputType string) (string, bool, error) {
	if inputType != "float32" && inputType != "float64" {
		return "", false, nil
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false, nil
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "math" {
		return "", true, fmt.Errorf("invalid float conversion")
	}
	want, bits := "Float64frombits", 64
	if inputType == "float32" {
		want, bits = "Float32frombits", 32
	}
	if selector.Sel.Name != want {
		return "", true, fmt.Errorf("expected math.%s", want)
	}
	text, kind, err := numericText(call.Args[0])
	if err != nil || kind != token.INT || strings.HasPrefix(text, "-") {
		return "", true, fmt.Errorf("expected unsigned integer bits")
	}
	v, err := strconv.ParseUint(text, 0, bits)
	if err != nil {
		return "", true, fmt.Errorf("invalid float bits: %w", err)
	}
	return mathPlaceholder + want + "(0x" + strconv.FormatUint(v, 16) + ")", true, nil
}

func integerBits(inputType string) int {
	switch inputType {
	case "int8", "uint8":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32":
		return 32
	case "int64", "uint64":
		return 64
	default:
		return strconv.IntSize
	}
}
