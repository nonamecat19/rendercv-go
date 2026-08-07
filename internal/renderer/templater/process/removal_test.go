package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// The whole removal path, measured against the vendored Python.
//
// This is the unit with the most surface in the iteration: a missing field has
// to take its formatting, its punctuation and its connector word with it, and
// leave everything that only looks like those alone.
func TestRemoveNotProvidedPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		fields   []string
		want     string
	}{
		{
			// The documented case: `in` goes because DEGREE is missing.
			name:     "a connector between a present and a missing placeholder",
			template: "**INSTITUTION**, DEGREE in AREA",
			fields:   []string{"INSTITUTION", "AREA"},
			want:     "**INSTITUTION**, AREA",
		},
		{
			// `at` **stays**: both its neighbours are present, and LOCATION —
			// which is missing — is not adjacent to it.
			name:     "a connector between two present placeholders stays",
			template: "**POSITION** at COMPANY, LOCATION",
			fields:   []string{"POSITION", "COMPANY"},
			want:     "**POSITION** at COMPANY",
		},
		{
			// Two whole lines disappear, which only works because
			// clean_trailing_parts drops a line that became empty.
			name:     "lines that were only placeholders are dropped",
			template: "**INSTITUTION**, AREA\nSUMMARY\nHIGHLIGHTS",
			fields:   []string{"INSTITUTION"},
			want:     "**INSTITUTION**",
		},
		{
			// `*in*` survives: the connector pattern rejects a first character
			// that is not a letter, so formatting is kept where a bare word is
			// not. The leading space is upstream's too.
			name:     "formatting is not a connector",
			template: "DEGREE *in* AREA",
			fields:   []string{"AREA"},
			want:     " *in* AREA",
		},
		{
			// And punctuation-only separators.
			name:     "punctuation is not a connector",
			template: "A -- B",
			fields:   []string{"A"},
			want:     "A",
		},
		{
			// The missing pattern eats adjacent non-space characters, so the
			// parentheses go with URL.
			name:     "surrounding punctuation goes with the placeholder",
			template: "NAME (URL)",
			fields:   []string{"NAME"},
			want:     "NAME",
		},
		{
			name:     "a trailing comma is cleaned",
			template: "TITLE, JOURNAL, DOI",
			fields:   []string{"TITLE", "JOURNAL"},
			want:     "TITLE, JOURNAL",
		},
		{
			// Both connectors go, because B sits between them and is missing.
			name:     "two connectors around one missing placeholder",
			template: "A and B or C",
			fields:   []string{"A", "C"},
			want:     "A C",
		},
		{
			// Lowercase words are not placeholders and are never scanned, so a
			// template of prose survives untouched.
			name:     "nothing missing, nothing touched",
			template: "everything provided AREA",
			fields:   []string{"AREA"},
			want:     "everything provided AREA",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			for _, name := range tc.fields {
				fields[name] = "x"
			}
			got := process.RemoveNotProvidedPlaceholders(
				map[string]string{"m": tc.template}, fields)
			if got["m"] != tc.want {
				t.Errorf("= %q, want %q", got["m"], tc.want)
			}
		})
	}
}

// **A literal uppercase word is treated as a missing placeholder.** Nothing
// distinguishes them and upstream does not try, so a template mentioning `CV`
// loses it. Pinned rather than fixed: matching upstream is the contract.
func TestALiteralUppercaseWordIsAMissingPlaceholder(t *testing.T) {
	got := process.RemoveNotProvidedPlaceholders(
		map[string]string{"m": "NAME CV"},
		map[string]string{"NAME": "x"},
	)
	if got["m"] != "NAME" {
		t.Errorf("= %q, want %q — upstream removes it too", got["m"], "NAME")
	}
}

// `clean_trailing_parts` keeps Markdown's own punctuation and drops separators,
// line by line.
func TestCleanTrailingParts(t *testing.T) {
	tests := map[string]string{
		"Position at Company, \nLink: ": "Position at Company\nLink",
		// An empty line is dropped rather than kept as a blank.
		"a\n\nb": "a\nb",
		// Only the *right* is trimmed, so the leading space survives.
		"  \n x , ": " x",
		// Markdown emphasis and links end in characters the pattern keeps.
		"*bold*,": "*bold*",
		"[a](b),": "[a](b)",
		"x--":     "x",
	}

	for text, want := range tests {
		t.Run(text, func(t *testing.T) {
			if got := process.CleanTrailingParts(text); got != want {
				t.Errorf("CleanTrailingParts(%q) = %q, want %q", text, got, want)
			}
		})
	}
}

// Nothing missing means nothing runs — the early return, which is what keeps a
// fully-populated entry byte-identical to its template.
func TestNothingMissingIsANoOp(t *testing.T) {
	templates := map[string]string{"m": "A  B ,"}
	got := process.RemoveNotProvidedPlaceholders(templates,
		map[string]string{"A": "1", "B": "2"})
	if got["m"] != "A  B ," {
		t.Errorf("= %q, want it untouched", got["m"])
	}
}
