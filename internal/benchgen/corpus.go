package benchgen

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func loadCorpus(dir, inputType string) ([]Seed, error) {
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
		if entry.IsDir() {
			return nil, fmt.Errorf("corpus entry %s is a directory", entry.Name())
		}
		path := filepath.Join(root, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read corpus %s: %w", path, err)
		}
		data, literal, err := decodeCorpusSeed(string(content), inputType)
		if err != nil {
			return nil, fmt.Errorf("corpus %s: %w", path, err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		if seed, ok := byHash[hash]; ok {
			seed.Origins = append(seed.Origins, path)
		} else {
			byHash[hash] = &Seed{Hash: hash, Origins: []string{path}, Data: data, Literal: literal}
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

func decodeSeed(content, inputType string) ([]byte, error) {
	data, _, err := decodeCorpusSeed(content, inputType)
	return data, err
}

func decodeCorpusSeed(content, inputType string) ([]byte, string, error) {
	header, body, ok := strings.Cut(content, "\n")
	if !ok || header != "go test fuzz v1" {
		return nil, "", fmt.Errorf("expected go test fuzz v1 header")
	}
	expr, err := parser.ParseExpr(strings.TrimSpace(body))
	if err != nil {
		return nil, "", fmt.Errorf("expected exactly one corpus argument: %w", err)
	}
	if inputType != "string" && inputType != "[]byte" {
		literal, err := scalarLiteral(expr, inputType)
		if err != nil {
			return nil, "", err
		}
		return []byte(literal), literal, nil
	}
	var lit *ast.BasicLit
	switch inputType {
	case "string":
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return nil, "", fmt.Errorf("expected string literal conversion")
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "string" {
			return nil, "", fmt.Errorf("expected string literal conversion")
		}
		lit, _ = call.Args[0].(*ast.BasicLit)
	case "[]byte":
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return nil, "", fmt.Errorf("expected []byte string conversion")
		}
		typ, ok := call.Fun.(*ast.ArrayType)
		if !ok || typ.Len != nil {
			return nil, "", fmt.Errorf("expected []byte string conversion")
		}
		name, ok := typ.Elt.(*ast.Ident)
		if !ok || name.Name != "byte" {
			return nil, "", fmt.Errorf("expected []byte string conversion")
		}
		lit, _ = call.Args[0].(*ast.BasicLit)
	default:
		return nil, "", fmt.Errorf("unsupported input type %q", inputType)
	}
	if lit == nil || lit.Kind != token.STRING {
		return nil, "", fmt.Errorf("expected %s corpus literal", inputType)
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil, "", fmt.Errorf("invalid string literal: %w", err)
	}
	literal := strconv.Quote(value)
	if inputType == "[]byte" {
		literal = "[]byte(" + literal + ")"
	}
	return []byte(value), literal, nil
}

func scalarLiteral(expr ast.Expr, inputType string) (string, error) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return "", fmt.Errorf("expected %s literal conversion", inputType)
	}
	name, ok := call.Fun.(*ast.Ident)
	if !ok || name.Name != inputType {
		return "", fmt.Errorf("expected %s literal conversion", inputType)
	}
	arg := call.Args[0]
	if inputType == "bool" {
		v, ok := arg.(*ast.Ident)
		if !ok || (v.Name != "true" && v.Name != "false") {
			return "", fmt.Errorf("expected bool literal")
		}
		return "bool(" + v.Name + ")", nil
	}
	text, isFloat, err := numericLiteral(arg)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(inputType, "float") {
		if !isFloat && strings.ContainsAny(text, "xX") {
			return "", fmt.Errorf("expected decimal %s literal", inputType)
		}
		bits := 64
		if inputType == "float32" {
			bits = 32
		}
		value, err := strconv.ParseFloat(text, bits)
		if err != nil {
			return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
		}
		return inputType + "(" + strconv.FormatFloat(value, 'g', -1, bits) + ")", nil
	}
	if isFloat {
		return "", fmt.Errorf("expected integer %s literal", inputType)
	}
	if strings.HasPrefix(inputType, "uint") {
		if strings.HasPrefix(text, "-") {
			return "", fmt.Errorf("invalid unsigned %s literal", inputType)
		}
		bits := integerBits(inputType)
		value, err := strconv.ParseUint(text, 0, bits)
		if err != nil {
			return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
		}
		return inputType + "(" + strconv.FormatUint(value, 10) + ")", nil
	}
	bits := integerBits(inputType)
	value, err := strconv.ParseInt(text, 0, bits)
	if err != nil {
		return "", fmt.Errorf("invalid %s literal: %w", inputType, err)
	}
	return inputType + "(" + strconv.FormatInt(value, 10) + ")", nil
}

func numericLiteral(expr ast.Expr) (string, bool, error) {
	negative := false
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if unary.Op != token.SUB {
			return "", false, fmt.Errorf("expected numeric literal")
		}
		negative, expr = true, unary.X
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || (lit.Kind != token.INT && lit.Kind != token.FLOAT) {
		return "", false, fmt.Errorf("expected numeric literal")
	}
	text := lit.Value
	if negative {
		text = "-" + text
	}
	return text, lit.Kind == token.FLOAT, nil
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
