package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// `substitute_placeholders`, every row measured against the vendored Python.
func TestSubstitutePlaceholders(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		placeholders map[string]string
		want         string
	}{
		{
			// The reason the names go into one alternation sorted longest
			// first. Replacing one at a time would leave `_IN_TWO_DIGITS`.
			name:         "longest name first",
			text:         "YEAR_IN_TWO_DIGITS and YEAR",
			placeholders: map[string]string{"YEAR": "2025", "YEAR_IN_TWO_DIGITS": "25"},
			want:         "25 and 2025",
		},
		{
			name:         "the result is stripped",
			text:         "  NAME  ",
			placeholders: map[string]string{"NAME": "John"},
			want:         "John",
		},
		{
			// The early return happens **before** the strip, so an empty map is
			// not a no-op plus a trim.
			name:         "an empty map does not strip",
			text:         "  NAME  ",
			placeholders: nil,
			want:         "  NAME  ",
		},
		{
			name:         "several in one string",
			text:         "NAME_CV_YEAR.pdf",
			placeholders: map[string]string{"NAME": "John_Doe", "YEAR": "2025"},
			want:         "John_Doe_CV_2025.pdf",
		},
		{
			name:         "every occurrence, not the first",
			text:         "NAME.NAME",
			placeholders: map[string]string{"NAME": "x"},
			want:         "x.x",
		},
		{
			// The collapse spec 008 §4B behavior 33 relies on: a zero count
			// blanks both halves and the template disappears rather than
			// printing `0 months`.
			name: "the time-span collapse, months empty",
			text: "HOW_MANY_YEARS YEARS HOW_MANY_MONTHS MONTHS",
			placeholders: map[string]string{
				"HOW_MANY_YEARS": "3", "YEARS": "years",
				"HOW_MANY_MONTHS": "", "MONTHS": "",
			},
			want: "3 years",
		},
		{
			// And the mirror, where the strip is doing the work at the front.
			name: "the time-span collapse, years empty",
			text: "HOW_MANY_YEARS YEARS HOW_MANY_MONTHS MONTHS",
			placeholders: map[string]string{
				"HOW_MANY_YEARS": "", "YEARS": "",
				"HOW_MANY_MONTHS": "5", "MONTHS": "months",
			},
			want: "5 months",
		},
		{
			name:         "everything blank",
			text:         "A B C",
			placeholders: map[string]string{"A": "", "B": "", "C": ""},
			want:         "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := process.SubstitutePlaceholders(tc.text, tc.placeholders); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// `clean_url` is two `str.replace` calls and one suffix test, not a URL parse,
// and the last three rows are what that costs.
func TestCleanURL(t *testing.T) {
	tests := map[string]string{
		"https://www.example.com/": "www.example.com",
		"http://a":                 "a",
		"www.x.com/y/":             "www.x.com/y",
		"a":                        "a",

		// Anywhere in the string, not only as a prefix.
		"https://a/https://b/": "a/b",
		// Exactly one trailing slash.
		"a//": "a/",
		// A bare scheme leaves nothing.
		"https://": "",
	}

	for url, want := range tests {
		t.Run(url, func(t *testing.T) {
			if got := process.CleanURL(url); got != want {
				t.Errorf("CleanURL(%q) = %q, want %q", url, got, want)
			}
		})
	}
}

// `strip` is Python's argument-less `str.strip()`.
func TestStrip(t *testing.T) {
	if got := process.Strip(" \t x \n "); got != "x" {
		t.Errorf("= %q, want %q", got, "x")
	}
}
