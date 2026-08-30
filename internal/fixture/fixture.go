// Package fixture loads the recorded profiles under testdata/ for tests.
//
// The source filenames in those profiles are stored relative to testdata/,
// which is not what pprof writes: a profile records absolute paths from the
// machine that produced it. Keeping them absolute would tie the source
// excerpts these tests check to one checkout living at one path, so the paths
// are rewritten when the profile is recorded and re-anchored here.
package fixture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/pprof/profile"
)

// Dir returns the absolute path of this module's testdata directory, found by
// walking up from the caller's working directory.
func Dir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "testdata")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no testdata directory above the working directory")
		}
		dir = parent
	}
}

// Path returns the absolute path of a file in testdata.
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// Load parses a recorded profile from testdata and makes its fixture source
// paths absolute for this checkout. Paths outside testdata — the standard
// library, the toolchain — are left as recorded; they are not expected to
// resolve, and nothing in the tests depends on them resolving.
func Load(name string) (*profile.Profile, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p, err := profile.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, fn := range p.Function {
		if fn == nil || filepath.IsAbs(fn.Filename) || fn.Filename == "" {
			continue
		}
		if !strings.HasPrefix(fn.Filename, "fixture/") {
			continue
		}
		fn.Filename = filepath.Join(dir, fn.Filename)
	}
	return p, nil
}
