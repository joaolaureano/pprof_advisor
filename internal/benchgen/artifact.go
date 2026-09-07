package benchgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// inertCopy prepends a build constraint that excludes the generated artifact
// from all builds. The record exists to be committed and diffed as documentation
// of what was generated, and a second copy of the same package-level test
// functions in the tree would cause a duplicate-symbol or orphan-package build
// failure for the target repository. The constraint is followed by a blank line
// per the Go build constraint syntax.
func inertCopy(code []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("//go:build ignore\n\n")
	buf.Write(code)
	return buf.Bytes()
}

func writeArtifacts(o Options, target Target, seeds []Seed, code []byte) (*Result, error) {
	if o.Out == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return nil, err
	}
	ordered := append([]Seed(nil), seeds...)
	for i := range ordered {
		ordered[i].Origins = append([]string(nil), ordered[i].Origins...)
		sort.Strings(ordered[i].Origins)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Hash < ordered[j].Hash })
	h := sha256.New()
	for _, seed := range ordered {
		fmt.Fprintln(h, seed.Hash)
	}
	codeHash := sha256.Sum256(code)
	manifest := Manifest{SchemaVersion: SchemaVersion, GeneratorVersion: GeneratorVersion, Target: target, CorpusHash: hex.EncodeToString(h.Sum(nil)), CodeHash: hex.EncodeToString(codeHash[:]), Seeds: ordered}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestBytes = append(manifestBytes, '\n')
	name := "profadvisor_" + target.Function
	result := &Result{SchemaVersion: SchemaVersion, Manifest: manifest, CodePath: filepath.Join(out, name+"_test.go"), ManifestPath: filepath.Join(out, name+".manifest.json"), Generated: true, Validated: false}
	type artifact struct {
		path string
		data []byte
	}
	files := []artifact{{result.CodePath, inertCopy(code)}, {result.ManifestPath, manifestBytes}}
	if o.Write {
		result.InstalledPath, err = filepath.Abs(filepath.Join(target.Dir, name+"_test.go"))
		if err != nil {
			return nil, err
		}
		if result.InstalledPath != result.CodePath {
			files = append(files, artifact{result.InstalledPath, code})
		}
	}
	for _, file := range files {
		if _, err := os.Lstat(file.path); err == nil {
			return nil, fmt.Errorf("refusing to overwrite %s", file.path)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return nil, err
	}
	var created []string
	rollback := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	for _, file := range files {
		f, err := os.OpenFile(file.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			rollback()
			return nil, fmt.Errorf("create artifact: %w", err)
		}
		created = append(created, file.path)
		_, writeErr := f.Write(file.data)
		closeErr := f.Close()
		if writeErr != nil {
			rollback()
			return nil, writeErr
		}
		if closeErr != nil {
			rollback()
			return nil, closeErr
		}
	}
	return result, nil
}
