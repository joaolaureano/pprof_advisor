package benchgen

import (
	"fmt"
	"strings"
	"testing"
)

func TestShiftPlaceholdersWithTwelveFields(t *testing.T) {
	// Test case (a): shiftPlaceholders with TWELVE placeholders $0..$11 and offset 1.
	// This is the exact bug that was fixed. Build the input as a realistic struct literal.
	var inputFields []string
	var expectedFields []string
	for i := 0; i < 12; i++ {
		inputFields = append(inputFields, fmt.Sprintf("F%d: $%d", i, i))
		expectedFields = append(expectedFields, fmt.Sprintf("F%d: $%d", i, i+1))
	}
	input := "Wide{" + strings.Join(inputFields, ", ") + "}"
	expected := "Wide{" + strings.Join(expectedFields, ", ") + "}"

	got := shiftPlaceholders(input, 12, 1)
	if got != expected {
		t.Fatalf("shiftPlaceholders(input, 12, 1):\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestShiftPlaceholdersWithOffsetZero(t *testing.T) {
	// Test case (b): shiftPlaceholders with offset 0 is the identity.
	input := "Value{A: $0, B: $1, C: $2}"
	got := shiftPlaceholders(input, 3, 0)
	if got != input {
		t.Fatalf("shiftPlaceholders(input, 3, 0) should be identity:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestShiftPlaceholdersLeavesUnrelatedPlaceholders(t *testing.T) {
	// Test case (c): shiftPlaceholders leaves a placeholder at or above count untouched.
	input := "Value{A: $0, B: $1, C: $5, D: $10}"
	// With count=2 and offset=1, only $0 and $1 should be shifted.
	// $5 and $10 should remain unchanged.
	expected := "Value{A: $1, B: $2, C: $5, D: $10}"

	got := shiftPlaceholders(input, 2, 1)
	if got != expected {
		t.Fatalf("shiftPlaceholders(input, 2, 1):\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestRenderCallArgumentsWithThirteenArguments(t *testing.T) {
	// Test case (d): renderCallArguments with THIRTEEN arguments and a template
	// using $0, $9, $10, and $12. Each must map to the right identifier.
	// In particular, $12 must not become <arg1>2 or similar corruption.
	arguments := make([]string, 13)
	for i := 0; i < 13; i++ {
		arguments[i] = fmt.Sprintf("input%d", i)
	}

	templates := []string{"Result{First: $0, Ninth: $9, Tenth: $10, Twelfth: $12}"}
	got, err := renderCallArguments(templates, arguments)
	if err != nil {
		t.Fatalf("renderCallArguments failed: %v", err)
	}

	expected := []string{"Result{First: input0, Ninth: input9, Tenth: input10, Twelfth: input12}"}
	if len(got) != len(expected) {
		t.Fatalf("renderCallArguments returned %d templates, want %d", len(got), len(expected))
	}
	if got[0] != expected[0] {
		t.Fatalf("renderCallArguments(templates, arguments)[0]:\n  got:  %q\n  want: %q", got[0], expected[0])
	}
}

func TestRenderCallArgumentsErrorOnMissingArgument(t *testing.T) {
	// Test case (e): renderCallArguments still returns an error for a template
	// with a placeholder that has no argument (leftover $).
	arguments := []string{"input0", "input1"}
	templates := []string{"Value{A: $0, B: $5}"}

	got, err := renderCallArguments(templates, arguments)
	if err == nil {
		t.Fatalf("renderCallArguments should have returned an error for leftover $5, got %v", got)
	}
	if got != nil {
		t.Fatalf("renderCallArguments returned %v on error, want nil", got)
	}
	// Verify the error message contains the expected template
	if !strings.Contains(err.Error(), "invalid generated argument template") {
		t.Fatalf("error message unexpected: %v", err)
	}
}

func TestMapPlaceholdersCopiesBareDollar(t *testing.T) {
	// Test case (f): mapPlaceholders copies a bare $ (no digits) through unchanged.
	input := "Price: $100 or amount: $0"
	got := mapPlaceholders(input, func(n int) (string, bool) {
		if n == 0 {
			return "fifty", true
		}
		return "", false
	})
	expected := "Price: $100 or amount: fifty"
	if got != expected {
		t.Fatalf("mapPlaceholders with bare $:\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestMapPlaceholdersEmptyInput(t *testing.T) {
	// Edge case: empty input string
	got := mapPlaceholders("", func(n int) (string, bool) {
		return "x", true
	})
	if got != "" {
		t.Fatalf("mapPlaceholders on empty string:\n  got:  %q\n  want: %q", got, "")
	}
}

func TestMapPlaceholdersNoPlaceholders(t *testing.T) {
	// Edge case: input with no placeholders
	input := "no placeholders here"
	got := mapPlaceholders(input, func(n int) (string, bool) {
		return "x", true
	})
	if got != input {
		t.Fatalf("mapPlaceholders on no-placeholder input:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestMapPlaceholdersLargePlaceholderNumbers(t *testing.T) {
	// Edge case: large placeholder numbers
	input := "Value{A: $0, B: $999, C: $1000}"
	got := mapPlaceholders(input, func(n int) (string, bool) {
		if n == 0 {
			return "zero", true
		}
		if n == 999 {
			return "largeNum", true
		}
		if n == 1000 {
			return "veryLarge", true
		}
		return "", false
	})
	expected := "Value{A: zero, B: largeNum, C: veryLarge}"
	if got != expected {
		t.Fatalf("mapPlaceholders with large numbers:\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestMapPlaceholdersConsecutiveDollars(t *testing.T) {
	// Edge case: consecutive dollars (e.g., "$$" should become "$" + first $'s handling)
	input := "Currency: $$50"
	got := mapPlaceholders(input, func(n int) (string, bool) {
		if n == 50 {
			return "FIFTY", true
		}
		return "", false
	})
	// First $ is bare, second $ starts a placeholder
	expected := "Currency: $FIFTY"
	if got != expected {
		t.Fatalf("mapPlaceholders with consecutive dollars:\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestMapPlaceholdersFunctionReturnsFalse(t *testing.T) {
	// Test that mapPlaceholders respects false return value from fn
	input := "A: $0, B: $1, C: $2"
	got := mapPlaceholders(input, func(n int) (string, bool) {
		if n == 1 {
			return "one", true
		}
		return "", false
	})
	expected := "A: $0, B: one, C: $2"
	if got != expected {
		t.Fatalf("mapPlaceholders respecting fn return false:\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestRenderCallArgumentsEmptyTemplates(t *testing.T) {
	// Verify existing behavior: empty templates returns a copy of arguments
	arguments := []string{"arg0", "arg1"}
	got, err := renderCallArguments([]string{}, arguments)
	if err != nil {
		t.Fatalf("renderCallArguments with empty templates failed: %v", err)
	}
	if len(got) != len(arguments) {
		t.Fatalf("renderCallArguments with empty templates returned %d results, want %d", len(got), len(arguments))
	}
	for i, arg := range arguments {
		if got[i] != arg {
			t.Fatalf("renderCallArguments with empty templates: got[%d]=%q, want %q", i, got[i], arg)
		}
	}
}

func TestShiftPlaceholdersMultipleFieldsRealistic(t *testing.T) {
	// Another realistic test with nested structures
	input := "Outer{Inner{A: $0, B: $1}, C: $2}"
	expected := "Outer{Inner{A: $5, B: $6}, C: $7}"
	got := shiftPlaceholders(input, 3, 5)
	if got != expected {
		t.Fatalf("shiftPlaceholders nested structure:\n  got:  %q\n  want: %q", got, expected)
	}
}
