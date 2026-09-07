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
		data, err := decodeSeed(string(content), inputType)
		if err != nil {
			return nil, fmt.Errorf("corpus %s: %w", path, err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		if seed, ok := byHash[hash]; ok {
			seed.Origins = append(seed.Origins, path)
		} else {
			byHash[hash] = &Seed{Hash: hash, Origins: []string{path}, Data: data}
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
	header, body, ok := strings.Cut(content, "\n")
	if !ok || header != "go test fuzz v1" {
		return nil, fmt.Errorf("expected go test fuzz v1 header")
	}
	expr, err := parser.ParseExpr(strings.TrimSpace(body))
	if err != nil {
		return nil, fmt.Errorf("expected exactly one corpus argument: %w", err)
	}
	var lit *ast.BasicLit
	switch inputType {
	case "string":
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return nil, fmt.Errorf("expected string literal conversion")
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || name.Name != "string" {
			return nil, fmt.Errorf("expected string literal conversion")
		}
		lit, _ = call.Args[0].(*ast.BasicLit)
	case "[]byte":
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return nil, fmt.Errorf("expected []byte string conversion")
		}
		typ, ok := call.Fun.(*ast.ArrayType)
		if !ok || typ.Len != nil {
			return nil, fmt.Errorf("expected []byte string conversion")
		}
		name, ok := typ.Elt.(*ast.Ident)
		if !ok || name.Name != "byte" {
			return nil, fmt.Errorf("expected []byte string conversion")
		}
		lit, _ = call.Args[0].(*ast.BasicLit)
	default:
		return nil, fmt.Errorf("unsupported input type %q", inputType)
	}
	if lit == nil || lit.Kind != token.STRING {
		return nil, fmt.Errorf("expected %s corpus literal", inputType)
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid string literal: %w", err)
	}
	return []byte(value), nil
}
