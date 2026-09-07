package prompt

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Compiled once. Render is called dozens of times to build a single prompt, and
// compiling these per call cost more than half of that prompt's CPU time — the
// tool found it in its own profile.
var (
	placeholderPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)
	namePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Catalog holds the validated prompt templates.
type Catalog struct {
	version int
	entries map[string]*entry
}

type entry struct {
	Placeholders []string `json:"placeholders"`
	Text         string   `json:"text"`
	// declared is Placeholders as a set, built once at load. Render checks
	// supplied variables against it on every call, and rebuilding the map each
	// time allocated once per call for a set that never changes.
	declared map[string]bool
}

// embeddedJSON holds the compiled-in catalog.
//
//go:embed prompts.json
var embeddedJSON []byte

// Load returns the embedded catalog, validated.
func Load() (*Catalog, error) {
	return load(embeddedJSON)
}

// LoadFile returns a catalog from a file path, validated identically to Load.
func LoadFile(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return load(data)
}

// load parses and validates a catalog from JSON bytes.
func load(data []byte) (*Catalog, error) {
	var raw struct {
		Version int               `json:"version"`
		Prompts map[string]*entry `json:"prompts"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var errs []string

	// Validate version.
	if raw.Version != 1 {
		errs = append(errs, fmt.Sprintf("version must be 1, got %d", raw.Version))
	}

	// Validate each entry.
	for key, ent := range raw.Prompts {
		if ent == nil {
			errs = append(errs, fmt.Sprintf("entry %q is null", key))
			continue
		}

		// Check placeholders are valid identifiers and unique.
		seen := make(map[string]bool)
		for _, p := range ent.Placeholders {
			if !namePattern.MatchString(p) {
				errs = append(errs, fmt.Sprintf("entry %q: placeholder %q does not match [a-z][a-z0-9_]*", key, p))
			}
			if seen[p] {
				errs = append(errs, fmt.Sprintf("entry %q: placeholder %q is duplicated", key, p))
			}
			seen[p] = true
		}

		// Check for unterminated { in text.
		text := ent.Text
		inBrace := false
		for i := 0; i < len(text); i++ {
			if text[i] == '{' {
				if i+1 < len(text) && text[i+1] == '{' {
					i++ // Skip escaped {{
					continue
				}
				inBrace = true
			} else if text[i] == '}' {
				if i+1 < len(text) && text[i+1] == '}' {
					i++ // Skip escaped }}
					continue
				}
				if !inBrace {
					errs = append(errs, fmt.Sprintf("entry %q: unmatched }} in text", key))
				}
				inBrace = false
			}
		}
		if inBrace {
			errs = append(errs, fmt.Sprintf("entry %q: unterminated {{ in text", key))
		}

		// Check declared placeholders match found placeholders.
		found := extractPlaceholders(text)
		declared := make(map[string]bool, len(ent.Placeholders))
		for _, p := range ent.Placeholders {
			declared[p] = true
		}
		ent.declared = declared

		for p := range found {
			if !declared[p] {
				errs = append(errs, fmt.Sprintf("entry %q: placeholder {%s} found in text but not declared", key, p))
			}
		}
		for p := range declared {
			if !found[p] {
				errs = append(errs, fmt.Sprintf("entry %q: placeholder %s declared but not found in text", key, p))
			}
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("validation errors:\n  %s", strings.Join(errs, "\n  "))
	}

	return &Catalog{
		version: raw.Version,
		entries: raw.Prompts,
	}, nil
}

// extractPlaceholders finds all {name} placeholders in text, excluding {{ and }}.
func extractPlaceholders(text string) map[string]bool {
	found := make(map[string]bool)
	// Replace escaped braces with placeholders to avoid matching them.
	temp := strings.ReplaceAll(text, "{{", "\x00")
	temp = strings.ReplaceAll(temp, "}}", "\x01")
	// Now find valid placeholders in the temp version.
	matches := placeholderPattern.FindAllStringSubmatch(temp, -1)
	for _, m := range matches {
		found[m[1]] = true
	}
	return found
}

// Render substitutes variables into a template and returns the result.
// It is strict in both directions: missing variables and extra variables both error.
func (c *Catalog) Render(key string, vars map[string]string) (string, error) {
	ent, ok := c.entries[key]
	if !ok {
		return "", fmt.Errorf("unknown key: %s", key)
	}

	for v := range vars {
		if !ent.declared[v] {
			return "", fmt.Errorf("key %s: unknown variable %s", key, v)
		}
	}

	for p := range ent.declared {
		if _, ok := vars[p]; !ok {
			return "", fmt.Errorf("key %s: missing variable %s", key, p)
		}
	}

	// Perform substitution.
	result := ent.Text
	// First, replace escaped braces with placeholders.
	result = strings.ReplaceAll(result, "{{", "\x00")
	result = strings.ReplaceAll(result, "}}", "\x01")

	// Then substitute variables.
	result = placeholderPattern.ReplaceAllStringFunc(result, func(match string) string {
		name := match[1 : len(match)-1]
		return vars[name]
	})

	// Finally, unescape braces.
	result = strings.ReplaceAll(result, "\x00", "{")
	result = strings.ReplaceAll(result, "\x01", "}")

	return result, nil
}

// Keys returns all prompt keys in sorted order.
func (c *Catalog) Keys() []string {
	var keys []string
	for k := range c.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
