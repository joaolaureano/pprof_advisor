package benchgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type listedPackage struct {
	Dir, ImportPath, Name, Export                string
	GoFiles, CgoFiles, TestGoFiles, XTestGoFiles []string
	DepOnly                                      bool
	Error                                        *struct{ Err string }
}

// collectPackageLevelNames returns the package-level names the given files
// declare. It reads the syntax rather than a type-checked scope because the
// names that matter here live in test files, which are not part of the scope
// resolveTarget type-checks.
func collectPackageLevelNames(files []*ast.File) map[string]bool {
	names := make(map[string]bool)
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					names[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						names[s.Name.Name] = true
					case *ast.ValueSpec:
						for _, n := range s.Names {
							names[n.Name] = true
						}
					}
				}
			}
		}
	}
	return names
}

func resolveTarget(ctx context.Context, o Options) (Target, error) {
	var target Target
	if o.Function == "" || !token.IsIdentifier(o.Function) || o.Function == "_" {
		return target, fmt.Errorf("--func must name a function")
	}
	dir := o.Dir
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return target, err
	}
	pattern := o.Package
	if pattern == "" {
		return target, fmt.Errorf("--pkg is required")
	}
	if strings.HasPrefix(pattern, "-") {
		return target, fmt.Errorf("invalid package pattern")
	}
	versionCmd := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	versionCmd.Dir = abs
	versionBytes, err := versionCmd.Output()
	if err != nil {
		return target, fmt.Errorf("resolve Go toolchain: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if !supportsLoop(version) {
		return target, fmt.Errorf("benchgen requires Go 1.24 or newer; target uses %s", version)
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-export", "-json", pattern)
	cmd.Dir = abs
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return target, fmt.Errorf("load target package: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	exports := map[string]string{}
	var roots []listedPackage
	dec := json.NewDecoder(bytes.NewReader(output))
	for {
		var p listedPackage
		err := dec.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			return target, fmt.Errorf("decode go list: %w", err)
		}
		if p.Error != nil {
			return target, fmt.Errorf("load package: %s", p.Error.Err)
		}
		exports[p.ImportPath] = p.Export
		if !p.DepOnly {
			roots = append(roots, p)
		}
	}
	if len(roots) != 1 {
		return target, fmt.Errorf("--pkg must resolve to exactly one package, got %d", len(roots))
	}
	p := roots[0]
	if len(p.CgoFiles) > 0 {
		return target, fmt.Errorf("packages using cgo are not supported")
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(p.GoFiles))
	fuzzName := "FuzzProfadvisor_" + o.Function
	benchmarkName := "BenchmarkProfadvisor_" + o.Function
	allFiles := append(append(append([]string{}, p.GoFiles...), p.TestGoFiles...), p.XTestGoFiles...)
	// The generated file is installed into the package's internal test files,
	// so those are kept apart from the rest: a name declared there collides
	// with the generated one, while a name in an external _test package does
	// not, and the type-checked scope built below sees neither.
	var allDecls, testDecls []*ast.File
	for i, name := range allFiles {
		file, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, 0)
		if err != nil {
			return target, fmt.Errorf("parse target: %w", err)
		}
		switch {
		case i < len(p.GoFiles):
			files = append(files, file)
		case i < len(p.GoFiles)+len(p.TestGoFiles):
			testDecls = append(testDecls, file)
		}
		allDecls = append(allDecls, file)
	}
	declared := collectPackageLevelNames(allDecls)
	for _, name := range []string{fuzzName, benchmarkName} {
		if declared[name] {
			return target, fmt.Errorf("generated symbol %s already exists", name)
		}
	}
	testFileNames := collectPackageLevelNames(testDecls)
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		archive := exports[path]
		if archive == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(archive)
	})
	conf := types.Config{Importer: imp}
	pkg, err := conf.Check(p.ImportPath, fset, files, nil)
	if err != nil {
		return target, fmt.Errorf("type-check target: %w", err)
	}
	for _, name := range []string{"string", "byte"} {
		if pkg.Scope().Lookup(name) != nil || testFileNames[name] {
			return target, fmt.Errorf("package shadows predeclared %s used by generated code", name)
		}
	}
	testingAlias := "profadvisorTesting"
	if o.Function == testingAlias {
		testingAlias += "_"
	}
	if pkg.Scope().Lookup(testingAlias) != nil || testFileNames[testingAlias] {
		return target, fmt.Errorf("package symbol %s conflicts with generated testing import", testingAlias)
	}
	for _, name := range []string{"profadvisorMath", "profadvisorMath_"} {
		if pkg.Scope().Lookup(name) != nil || testFileNames[name] {
			return target, fmt.Errorf("package symbol %s conflicts with generated math import", name)
		}
	}
	fn, ok := pkg.Scope().Lookup(o.Function).(*types.Func)
	if !ok {
		return target, fmt.Errorf("%s is not a package function", o.Function)
	}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() != nil || sig.TypeParams().Len() != 0 || sig.Variadic() || sig.Params().Len() == 0 {
		return target, fmt.Errorf("function must be non-generic, non-variadic and accept one or more fuzzable arguments")
	}
	inputs := make([]string, 0, sig.Params().Len())
	argumentTypes := make([]string, sig.Params().Len())
	argumentTemplates := make([]string, sig.Params().Len())
	for i := 0; i < sig.Params().Len(); i++ {
		parameterType := sig.Params().At(i).Type()
		argumentTypes[i] = goTypeString(parameterType, pkg)
		leaves, template, err := flattenFuzzArgument(parameterType, pkg)
		if err != nil {
			return target, fmt.Errorf("parameter %d: %w", i+1, err)
		}
		argumentTemplates[i] = shiftPlaceholders(template, len(leaves), len(inputs))
		inputs = append(inputs, leaves...)
	}
	if len(inputs) == 0 {
		return target, fmt.Errorf("function must include at least one native Go fuzz input; empty structs cannot be fuzzed")
	}
	return Target{Dir: p.Dir, Package: p.ImportPath, Name: p.Name, Function: o.Function, ArgumentTypes: argumentTypes, InputTypes: inputs, ArgumentTemplates: argumentTemplates, GoVersion: version, FuzzName: fuzzName, BenchmarkName: benchmarkName}, nil
}

// flattenFuzzArgument turns a function argument into the native values that
// testing.F can accept. Structs are reconstructed with keyed literals in the
// generated same-package test, so no reflection or serialization runs in a
// benchmark loop. Only local structs are accepted: external struct names would
// require imports and may expose inaccessible fields.
func flattenFuzzArgument(typ types.Type, pkg *types.Package) ([]string, string, error) {
	if input := fuzzInputType(typ); input != "" {
		return []string{input}, "$0", nil
	}
	unaliased := types.Unalias(typ)
	underlying, ok := unaliased.Underlying().(*types.Struct)
	if !ok {
		return nil, "", fmt.Errorf("must be a native Go fuzz type or a struct composed of native Go fuzz types")
	}
	if named, ok := unaliased.(*types.Named); ok && named.Obj().Pkg() != pkg {
		return nil, "", fmt.Errorf("struct %s is declared outside the target package", named.Obj().Name())
	}
	inputs := make([]string, 0, underlying.NumFields())
	fields := make([]string, 0, underlying.NumFields())
	for i := 0; i < underlying.NumFields(); i++ {
		field := underlying.Field(i)
		if field.Name() == "_" {
			return nil, "", fmt.Errorf("struct contains blank field %d, which cannot be reconstructed", i+1)
		}
		leaves, expression, err := flattenFuzzArgument(field.Type(), pkg)
		if err != nil {
			return nil, "", fmt.Errorf("struct field %s: %w", field.Name(), err)
		}
		fields = append(fields, field.Name()+": "+shiftPlaceholders(expression, len(leaves), len(inputs)))
		inputs = append(inputs, leaves...)
	}
	return inputs, goTypeString(unaliased, pkg) + "{" + strings.Join(fields, ", ") + "}", nil
}

func shiftPlaceholders(expression string, count, offset int) string {
	for i := count - 1; i >= 0; i-- {
		expression = strings.ReplaceAll(expression, "$"+strconv.Itoa(i), "$"+strconv.Itoa(i+offset))
	}
	return expression
}

func goTypeString(typ types.Type, current *types.Package) string {
	if named, ok := types.Unalias(typ).(*types.Named); ok {
		return named.Obj().Name()
	}
	return types.TypeString(typ, func(p *types.Package) string {
		if p == current {
			return ""
		}
		return p.Name()
	})
}

// fuzzInputType uses types.Identical deliberately: aliases such as rune and
// byte are accepted as their underlying predeclared fuzz types, while defined
// types are rejected because testing.F cannot seed them directly.
func fuzzInputType(typ types.Type) string {
	if types.Identical(typ, types.Typ[types.String]) {
		return "string"
	}
	if types.Identical(typ, types.NewSlice(types.Typ[types.Byte])) {
		return "[]byte"
	}
	for _, basic := range []types.BasicKind{
		types.Bool, types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Float32, types.Float64,
	} {
		if types.Identical(typ, types.Typ[basic]) {
			return types.Typ[basic].Name()
		}
	}
	return ""
}

func supportsLoop(version string) bool {
	if strings.HasPrefix(version, "devel ") {
		version = strings.TrimPrefix(version, "devel ")
	}
	version = strings.TrimPrefix(version, "go")
	major, rest, ok := strings.Cut(version, ".")
	if !ok {
		return false
	}
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	a, err := strconv.Atoi(major)
	if err != nil {
		return false
	}
	b, err := strconv.Atoi(rest[:end])
	return err == nil && (a > 1 || a == 1 && b >= 24)
}
