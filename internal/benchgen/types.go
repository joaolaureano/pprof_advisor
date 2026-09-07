// Package benchgen generates offline benchmark and fuzz harnesses from a frozen Go corpus.
package benchgen

import (
	"context"
	"fmt"
)

const SchemaVersion = 1
const GeneratorVersion = "1"

type Options struct {
	Dir      string
	Package  string
	Function string
	Corpus   string
	Out      string
	Write    bool
}

type Target struct {
	Dir           string `json:"dir"`
	Package       string `json:"package"`
	Name          string `json:"name"`
	Function      string `json:"function"`
	InputType     string `json:"input_type"`
	GoVersion     string `json:"go_version"`
	FuzzName      string `json:"fuzz_name"`
	BenchmarkName string `json:"benchmark_name"`
}

type Seed struct {
	Hash    string   `json:"hash"`
	Origins []string `json:"origins"`
	Data    []byte   `json:"-"`
	// Literal is a canonical Go expression used in generated source. It is kept
	// out of the manifest because the hash is the stable seed identity.
	Literal string `json:"-"`
}

type Manifest struct {
	SchemaVersion    int    `json:"schema_version"`
	GeneratorVersion string `json:"generator_version"`
	Target           Target `json:"target"`
	CorpusHash       string `json:"corpus_hash"`
	CodeHash         string `json:"code_hash"`
	Seeds            []Seed `json:"seeds"`
}

type Result struct {
	SchemaVersion int      `json:"schema_version"`
	Manifest      Manifest `json:"manifest"`
	CodePath      string   `json:"code_path"`
	ManifestPath  string   `json:"manifest_path"`
	InstalledPath string   `json:"installed_path,omitempty"`
	Generated     bool     `json:"generated"`
	Validated     bool     `json:"validated"`
}

func Generate(ctx context.Context, o Options) (*Result, error) {
	if o.Out == "" || o.Corpus == "" {
		return nil, fmt.Errorf("--out and --corpus are required")
	}
	target, err := resolveTarget(ctx, o)
	if err != nil {
		return nil, err
	}
	seeds, err := loadCorpus(o.Corpus, target.InputType)
	if err != nil {
		return nil, err
	}
	code, err := generateCode(target, seeds)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return writeArtifacts(o, target, seeds, code)
}
