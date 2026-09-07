package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cat == nil {
		t.Fatal("Load returned nil catalog")
	}

	// Verify all expected keys are present.
	expectedKeys := []string{
		"system.base",
		"system.guard.contention",
		"system.guard.cpu",
		"system.guard.memory",
		"system.mechanisms.contention",
		"system.mechanisms.cpu",
		"system.mechanisms.memory",
		"system.objective.contention",
		"system.objective.cpu",
		"system.objective.memory",
		"system.profile.contention",
		"system.profile.cpu",
		"system.profile.memory",
		"user.attributed_line",
		"user.attribution_note.contention",
		"user.attribution_note.memory",
		"user.contention_note",
		"user.excluded_intro",
		"user.header",
		"user.hotspots_heading",
		"user.module_line",
		"user.objective_line",
		"user.profile_line",
		"user.source_unavailable",
		"user.task",
		"user.total_line",
		"user.walltime_line",
		"schema.cause",
		"schema.change",
		"schema.confidence",
		"schema.diff",
		"schema.risks",
		"schema.target",
	}

	for _, key := range expectedKeys {
		if _, ok := cat.entries[key]; !ok {
			t.Errorf("expected key %q not found", key)
		}
	}
}

func TestRenderSimple(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test rendering a simple entry with no placeholders.
	result, err := cat.Render("system.profile.cpu", map[string]string{})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "CPU" {
		t.Errorf("expected %q, got %q", "CPU", result)
	}
}

func TestRenderWithPlaceholders(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test rendering user.header.
	result, err := cat.Render("user.header", map[string]string{"kind": "Memory"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "# Memory profile\n\n" {
		t.Errorf("expected %q, got %q", "# Memory profile\n\n", result)
	}
}

func TestRenderMultiplePlaceholders(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test rendering user.objective_line with two placeholders.
	result, err := cat.Render("user.objective_line", map[string]string{
		"unit":        "ns/op",
		"sample_type": "cpu",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Objective: ns/op (cpu)\n" {
		t.Errorf("expected %q, got %q", "Objective: ns/op (cpu)\n", result)
	}
}

func TestRenderEscaping(t *testing.T) {
	// Create a test catalog with escaped braces.
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "test.escaping": {
      "placeholders": ["value"],
      "text": "This has {{escaped}} and {value} placeholder"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cat, err := LoadFile(testFile)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	result, err := cat.Render("test.escaping", map[string]string{"value": "VALUE"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "This has {escaped} and VALUE placeholder" {
		t.Errorf("expected %q, got %q", "This has {escaped} and VALUE placeholder", result)
	}
}

func TestRenderUnknownKey(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = cat.Render("nonexistent.key", map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("expected 'unknown key' in error, got: %v", err)
	}
}

func TestRenderMissingVariable(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = cat.Render("user.objective_line", map[string]string{"unit": "ns/op"})
	if err == nil {
		t.Fatal("expected error for missing variable")
	}
	if !strings.Contains(err.Error(), "missing variable") || !strings.Contains(err.Error(), "sample_type") {
		t.Errorf("expected 'missing variable sample_type' in error, got: %v", err)
	}
}

func TestRenderExtraVariable(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = cat.Render("user.header", map[string]string{
		"kind":  "Memory",
		"extra": "value",
	})
	if err == nil {
		t.Fatal("expected error for extra variable")
	}
	if !strings.Contains(err.Error(), "unknown variable") || !strings.Contains(err.Error(), "extra") {
		t.Errorf("expected 'unknown variable extra' in error, got: %v", err)
	}
}

func TestMismatchedPlaceholdersDeclaredButAbsent(t *testing.T) {
	// Test a catalog where placeholders are declared but not in text.
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "bad.entry": {
      "placeholders": ["declared_but_absent"],
      "text": "This text has no placeholders"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for mismatched placeholders")
	}
	if !strings.Contains(err.Error(), "declared but not found") {
		t.Errorf("expected 'declared but not found' in error, got: %v", err)
	}
}

func TestMismatchedPlaceholdersPresentButUndeclared(t *testing.T) {
	// Test a catalog where placeholders are in text but not declared.
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "bad.entry": {
      "placeholders": [],
      "text": "This has a {placeholder} in the text"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for mismatched placeholders")
	}
	if !strings.Contains(err.Error(), "found in text but not declared") {
		t.Errorf("expected 'found in text but not declared' in error, got: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	// For each key, render with all declared placeholders and verify no leftover placeholders.
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	for _, key := range cat.Keys() {
		ent := cat.entries[key]
		vars := make(map[string]string)
		for _, p := range ent.Placeholders {
			vars[p] = "TEST_" + p
		}

		result, err := cat.Render(key, vars)
		if err != nil {
			t.Errorf("Render failed for key %q: %v", key, err)
			continue
		}

		// Check that result contains no leftover {placeholder} patterns.
		// This is a simple heuristic: if the string contains { followed by
		// lowercase letters, it's likely a placeholder.
		for i := 0; i < len(result); i++ {
			if result[i] == '{' && i+1 < len(result) {
				next := result[i+1]
				if (next >= 'a' && next <= 'z') || next == '{' {
					t.Errorf("key %q: result contains potential leftover placeholder at position %d: %q", key, i, result[i:i+2])
					break
				}
			}
		}
	}
}

func TestKeys(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	keys := cat.Keys()
	if len(keys) == 0 {
		t.Fatal("Keys returned empty list")
	}

	// Check that keys are sorted.
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Errorf("Keys are not sorted: %v < %v", keys[i], keys[i-1])
		}
	}
}

func TestLoadFileNonexistent(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/to/file.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestValidationInvalidVersion(t *testing.T) {
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 2,
  "prompts": {}
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
	if !strings.Contains(err.Error(), "version must be 1") {
		t.Errorf("expected 'version must be 1' in error, got: %v", err)
	}
}

func TestValidationInvalidPlaceholderName(t *testing.T) {
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "bad.entry": {
      "placeholders": ["123invalid"],
      "text": "text"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for invalid placeholder name")
	}
	if !strings.Contains(err.Error(), "does not match [a-z][a-z0-9_]*") {
		t.Errorf("expected placeholder validation error, got: %v", err)
	}
}

func TestValidationDuplicatePlaceholder(t *testing.T) {
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "bad.entry": {
      "placeholders": ["dup", "dup"],
      "text": "{dup}"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for duplicate placeholder")
	}
	if !strings.Contains(err.Error(), "is duplicated") {
		t.Errorf("expected duplicate placeholder error, got: %v", err)
	}
}

func TestValidationUnterminatedBrace(t *testing.T) {
	tmpdir := t.TempDir()
	testJSON := `{
  "version": 1,
  "prompts": {
    "bad.entry": {
      "placeholders": [],
      "text": "This has an unterminated { brace"
    }
  }
}`
	testFile := filepath.Join(tmpdir, "test.json")
	if err := os.WriteFile(testFile, []byte(testJSON), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadFile(testFile)
	if err == nil {
		t.Fatal("expected error for unterminated brace")
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("expected unterminated brace error, got: %v", err)
	}
}
