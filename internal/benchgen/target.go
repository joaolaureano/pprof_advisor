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
	"sort"
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

// parseImplementations parses "Interface=Type" pairs from the CLI, returning
// a map keyed by interface name as written. Rejects duplicates, empty keys/values, or malformed syntax.
func parseImplementations(pairs []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("malformed --impl: %q (expected format: Interface=Type)", pair)
		}
		if key == "" {
			return nil, fmt.Errorf("malformed --impl: %q (empty interface name)", pair)
		}
		if value == "" {
			return nil, fmt.Errorf("malformed --impl: %q (empty type name)", pair)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("malformed --impl: duplicate key %q in %q", key, pair)
		}
		result[key] = value
	}
	return result, nil
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

	// Parse --impl pairs early so we error out before type-checking parameters.
	impls, err := parseImplementations(o.Implementations)
	if err != nil {
		return target, err
	}

	inputs := make([]string, 0, sig.Params().Len())
	argumentTypes := make([]string, sig.Params().Len())
	argumentTemplates := make([]string, sig.Params().Len())
	chosen := make(map[string]string) // output: interface name -> concrete type actually used
	for i := 0; i < sig.Params().Len(); i++ {
		parameterType := sig.Params().At(i).Type()
		argumentTypes[i] = goTypeString(parameterType, pkg)
		seen := make(map[string]bool) // cycle guard: path set for this parameter
		leaves, template, err := flattenFuzzArgument(parameterType, pkg, impls, chosen, seen)
		if err != nil {
			return target, fmt.Errorf("parameter %d: %w", i+1, err)
		}
		argumentTemplates[i] = shiftPlaceholders(template, len(leaves), len(inputs))
		inputs = append(inputs, leaves...)
	}
	if len(inputs) == 0 {
		return target, fmt.Errorf("function must include at least one native Go fuzz input; empty structs cannot be fuzzed")
	}

	// Build Target.Implementations as sorted "Iface=Type" strings.
	implementations := make([]string, 0, len(chosen))
	for iface, impl := range chosen {
		implementations = append(implementations, iface+"="+impl)
	}
	sort.Strings(implementations)

	return Target{Dir: p.Dir, Package: p.ImportPath, Name: p.Name, Function: o.Function, ArgumentTypes: argumentTypes, InputTypes: inputs, Implementations: implementations, ArgumentTemplates: argumentTemplates, GoVersion: version, FuzzName: fuzzName, BenchmarkName: benchmarkName}, nil
}

// resolveInterfaceImplementation finds a concrete named type that implements the
// given interface. If impls has an entry for the interface, it uses that after
// validation. Otherwise, it scans pkg.Scope().Names() and returns the first
// candidate that implements the interface and flattens (or is close enough).
// The returned string is the implementation's bare name, and needsPointer indicates
// whether to wrap the template as &T{...} rather than T{...}.
// resolveInterfaceImplementation picks the concrete local type to instantiate
// for an interface parameter, and reports whether it must be addressed to
// satisfy the interface.
//
// The benchmark measures that type, not "the interface": the cost behind an
// interface call is entirely the implementation's. So an ambiguous choice is
// never made silently, and an explicit --impl always wins over discovery — a
// silent fallback would let the user ask for one program and measure another.
func resolveInterfaceImplementation(iface *types.Interface, ifaceType *types.Named, pkg *types.Package, impls map[string]string, seen map[string]bool) (string, bool, error) {
	name := ifaceType.Obj().Name()
	// Both spellings are accepted, so --impl io.Reader=T reads as naturally as
	// --impl Reader=T for an interface the target package did not declare.
	keys := []string{name}
	if obj := ifaceType.Obj(); obj.Pkg() != nil {
		keys = append(keys, obj.Pkg().Name()+"."+name)
	}
	for _, key := range keys {
		selected, ok := impls[key]
		if !ok {
			continue
		}
		pointer, err := localImplements(selected, name, iface, pkg)
		if err != nil {
			return "", false, fmt.Errorf("--impl %s=%s: %w", key, selected, err)
		}
		return selected, pointer, nil
	}

	type candidate struct {
		name    string
		pointer bool
	}
	var candidates []candidate
	// go/types returns Names sorted, so discovery is deterministic.
	for _, declared := range pkg.Scope().Names() {
		typeName, ok := pkg.Scope().Lookup(declared).(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok || named.Obj().Pkg() != pkg {
			continue
		}
		// An interface satisfies itself, so types.Implements would offer the
		// interface under resolution as an implementation of itself. It is not
		// a concrete type and flattening it leads straight back here.
		if _, isInterface := named.Underlying().(*types.Interface); isInterface {
			continue
		}
		switch {
		case types.Implements(named, iface):
			candidates = append(candidates, candidate{declared, false})
		case types.Implements(types.NewPointer(named), iface):
			// Methods on a pointer receiver are the norm in Go. Skipping this
			// case would discard most of the candidates that actually exist.
			candidates = append(candidates, candidate{declared, true})
		}
	}
	switch len(candidates) {
	case 0:
		return "", false, fmt.Errorf("no local type implements %s, and no type is imported to supply one", name)
	case 1:
		// Returned without a trial flatten deliberately. If the only candidate
		// cannot be flattened, its own error — a channel field, a cycle back
		// through this interface — tells the user far more than a report that
		// no candidate was found.
		return candidates[0].name, candidates[0].pointer, nil
	}
	// Several types implement it, so the ones that cannot be flattened are not
	// real choices: they are dropped rather than counted into an ambiguity the
	// user cannot resolve. The trial runs the real flattener, so it can never
	// disagree with what generation would do.
	var usable []candidate
	all := make([]string, 0, len(candidates))
	for _, c := range candidates {
		all = append(all, c.name)
		trial := make(map[string]bool, len(seen))
		for path := range seen {
			trial[path] = true
		}
		if _, _, err := flattenFuzzArgument(pkg.Scope().Lookup(c.name).Type(), pkg, impls, map[string]string{}, trial); err == nil {
			usable = append(usable, c)
		}
	}
	switch len(usable) {
	case 0:
		return "", false, fmt.Errorf("%d types implement %s (%s) but none is composed of native Go fuzz types", len(all), name, strings.Join(all, ", "))
	case 1:
		return usable[0].name, usable[0].pointer, nil
	}
	names := make([]string, 0, len(usable))
	for _, c := range usable {
		names = append(names, c.name)
	}
	return "", false, fmt.Errorf("multiple types implement %s: %s; pick one with --impl %s=<Type>", name, strings.Join(names, ", "), name)
}

// localImplements reports whether the named local type satisfies iface, and
// whether it has to be addressed to do so.
func localImplements(selected, ifaceName string, iface *types.Interface, pkg *types.Package) (bool, error) {
	obj := pkg.Scope().Lookup(selected)
	if obj == nil {
		return false, fmt.Errorf("%s is not declared in package %s", selected, pkg.Name())
	}
	named, ok := obj.Type().(*types.Named)
	if !ok || named.Obj().Pkg() != pkg {
		return false, fmt.Errorf("%s is not a type declared in package %s", selected, pkg.Name())
	}
	if _, isInterface := named.Underlying().(*types.Interface); isInterface {
		return false, fmt.Errorf("%s is itself an interface, not a concrete type", selected)
	}
	if types.Implements(named, iface) {
		return false, nil
	}
	if types.Implements(types.NewPointer(named), iface) {
		return true, nil
	}
	return false, fmt.Errorf("%s does not implement %s", selected, ifaceName)
}

func flattenFuzzArgument(typ types.Type, pkg *types.Package, impls map[string]string, chosen map[string]string, seen map[string]bool) ([]string, string, error) {
	if input := fuzzInputType(typ); input != "" {
		return []string{input}, "$0", nil
	}
	unaliased := types.Unalias(typ)

	// Handle interface types.
	if iface, ok := unaliased.Underlying().(*types.Interface); ok {
		ifaceNamed, ok := unaliased.(*types.Named)
		if !ok {
			// For anonymous interfaces, provide a more general error mentioning zero methods if applicable
			if iface.NumMethods() == 0 {
				return nil, "", fmt.Errorf("interface{} (or any) has zero methods; every type satisfies it so no implementation can be chosen")
			}
			return nil, "", fmt.Errorf("anonymous interface cannot be used; a named interface type is required")
		}
		if iface.NumMethods() == 0 {
			return nil, "", fmt.Errorf("interface %s has zero methods (every type satisfies it); no implementation can be chosen", ifaceNamed.Obj().Name())
		}

		// The interface node carries its own cycle guard. The struct case has
		// one too, but an implementation reached through a chain of interfaces
		// would never touch it, and the recursion has to terminate either way.
		ifaceKey := "interface " + types.TypeString(ifaceNamed, nil)
		if seen[ifaceKey] {
			return nil, "", fmt.Errorf("cycle detected through interface %s", ifaceNamed.Obj().Name())
		}
		seen[ifaceKey] = true
		defer delete(seen, ifaceKey)

		implTypeName, needsPointer, err := resolveInterfaceImplementation(iface, ifaceNamed, pkg, impls, seen)
		if err != nil {
			return nil, "", err
		}

		// Get the concrete type from the package scope.
		implObj := pkg.Scope().Lookup(implTypeName)
		implNamed := implObj.Type().(*types.Named)

		// Record the choice.
		chosen[ifaceNamed.Obj().Name()] = implTypeName

		// Recurse into the concrete type. The struct case will handle cycle detection.
		leaves, template, err := flattenFuzzArgument(implNamed, pkg, impls, chosen, seen)

		if err != nil {
			return nil, "", err
		}

		// The template already includes the type name (e.g., "MyReader{...}").
		// We only need to add & if pointer receiver is needed.
		if needsPointer {
			template = "&" + template
		}
		return leaves, template, nil
	}

	underlying, ok := unaliased.Underlying().(*types.Struct)
	if !ok {
		return nil, "", fmt.Errorf("must be a native Go fuzz type or a struct composed of native Go fuzz types")
	}
	if named, ok := unaliased.(*types.Named); ok && named.Obj().Pkg() != pkg {
		return nil, "", fmt.Errorf("struct %s is declared outside the target package", named.Obj().Name())
	}

	// Check for cycles on entering a named type.
	if named, ok := unaliased.(*types.Named); ok {
		fullName := named.Obj().Pkg().Path() + "." + named.Obj().Name()
		if seen[fullName] {
			return nil, "", fmt.Errorf("cycle detected in type %s", named.Obj().Name())
		}
		seen[fullName] = true
		defer delete(seen, fullName)
	}

	inputs := make([]string, 0, underlying.NumFields())
	fields := make([]string, 0, underlying.NumFields())
	for i := 0; i < underlying.NumFields(); i++ {
		field := underlying.Field(i)
		if field.Name() == "_" {
			return nil, "", fmt.Errorf("struct contains blank field %d, which cannot be reconstructed", i+1)
		}
		leaves, expression, err := flattenFuzzArgument(field.Type(), pkg, impls, chosen, seen)
		if err != nil {
			return nil, "", fmt.Errorf("struct field %s: %w", field.Name(), err)
		}
		fields = append(fields, field.Name()+": "+shiftPlaceholders(expression, len(leaves), len(inputs)))
		inputs = append(inputs, leaves...)
	}
	return inputs, goTypeString(unaliased, pkg) + "{" + strings.Join(fields, ", ") + "}", nil
}

func shiftPlaceholders(expression string, count, offset int) string {
	return mapPlaceholders(expression, func(n int) (string, bool) {
		if n < count {
			return "$" + strconv.Itoa(n+offset), true
		}
		return "", false
	})
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
