package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// `validate_typst_dimension`, measured against the vendored Python on every row.
func TestValidTypstDimension(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"1cm", true},
		{"0.1cm", true},
		{"-0.5in", true},
		{"10pt", true},
		{"2mm", true},
		{"1.25em", true},

		// A bare number: the unit is required, and this is the row a pattern
		// written with `*` instead of a required group would let through.
		{"1", false},
		// An unsupported unit. `px` reads like a plausible member and is not one.
		{"1px", false},
		// No space is admitted between number and unit.
		{"1 cm", false},
		// `fullmatch`, not `search`: neither a prefix nor a suffix is enough.
		{"x1cm", false},
		{"1cmx", false},
		{"", false},
		{".5cm", false},

		// **Python's `\d` on a `str` matches every Unicode decimal digit,
		// not just ASCII.** `"٢cm"` is an Arabic-Indic 2. Found by a
		// fresh-context verifier (iteration 14's non-colour-design-slice
		// sweep).
		{"٢cm", true},
		{"١٢.٥mm", true},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			if got := design.ValidTypstDimension(tc.value); got != tc.want {
				t.Errorf("ValidTypstDimension(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// The message is upstream's literal, asserted here rather than through the Go
// constant so that changing the constant fails.
func TestBadTypstDimensionMessage(t *testing.T) {
	const want = "The value must be a number followed by a unit (cm, in, pt, mm, em)." +
		" For example, 0.1cm."
	if design.MessageBadTypstDimension != want {
		t.Errorf("= %q, want %q", design.MessageBadTypstDimension, want)
	}
	if design.CodeTypstDimension != "rendercv_other_error" {
		t.Errorf("code = %q, want %q", design.CodeTypstDimension, "rendercv_other_error")
	}
}
