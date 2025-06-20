package process_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// `process_highlights`. The sub-bullet separator is **space-hyphen-space**, so a
// hyphenated word is untouched.
func TestHighlights(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"two flat highlights", []string{"a", "b"}, "- a\n- b"},
		{
			name: "one highlight carrying its own nesting",
			in:   []string{"Reduced costs - Server optimization - Database indexing"},
			want: "- Reduced costs\n  - Server optimization\n  - Database indexing",
		},
		{"a hyphenated word is not a separator", []string{"a-b"}, "- a-b"},
		{"none", nil, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := process.Highlights(tc.in); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// `process_authors`: no `and`, no serial comma.
func TestAuthors(t *testing.T) {
	if got := process.Authors([]string{"A", "B", "C"}); got != "A, B, C" {
		t.Errorf("= %q, want %q", got, "A, B, C")
	}
	if got := process.Authors([]string{"A"}); got != "A" {
		t.Errorf("= %q, want %q", got, "A")
	}
}

// `process_summary` wraps in the admonition that MarkdownToTypst is built to
// recognize, and `textwrap.indent` **skips a blank line** — which matters
// because an indented blank line would end the block collector's run early.
func TestSummaryBlock(t *testing.T) {
	tests := map[string]string{
		"one line":           "!!! summary\n    one line",
		"line one\nline two": "!!! summary\n    line one\n    line two",
		"a\n\nb":             "!!! summary\n    a\n\n    b",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			if got := process.SummaryBlock(in); got != want {
				t.Errorf("= %q, want %q", got, want)
			}
		})
	}
}

// And the round trip: what `process_summary` writes is what MarkdownToTypst
// collects. The two were written from different upstream modules and this is the
// only test that puts them together.
func TestASummaryRoundTripsIntoASummaryBlock(t *testing.T) {
	got := process.MarkdownToTypst(process.SummaryBlock("line one\nline two"))
	if got != `#summary[line one \ line two]` {
		t.Errorf("= %q, want a single #summary block", got)
	}
	if strings.Contains(got, "!!!") {
		t.Error("the admonition marker survived; the block was not collected")
	}
}

// A publication with a DOI shows the DOI link for **both** placeholders.
func TestEntryURLDelegatesToDOI(t *testing.T) {
	const doiURL = "https://doi.org/10.1000/xyz"
	if got := process.EntryURL("https://example.com", "10.1000/xyz", doiURL); got != "[10.1000/xyz]("+doiURL+")" {
		t.Errorf("= %q, want the DOI link", got)
	}
	// Without a DOI it is the entry's own URL, cleaned for display only.
	if got := process.EntryURL("https://www.example.com/p", "", ""); got != "[www.example.com/p](https://www.example.com/p)" {
		t.Errorf("= %q", got)
	}
}

// The wrap depends on the **theme's template**, not on the entry.
func TestSummaryIsStandalone(t *testing.T) {
	if !process.SummaryIsStandalone(map[string]string{"m": "**A**\nSUMMARY\nB"}) {
		t.Error("a line of exactly SUMMARY is standalone")
	}
	if !process.SummaryIsStandalone(map[string]string{"m": "  SUMMARY  "}) {
		t.Error("surrounding whitespace is stripped before the test")
	}
	if process.SummaryIsStandalone(map[string]string{"m": "A SUMMARY B"}) {
		t.Error("a summary interpolated mid-line is not standalone")
	}
}

// Phrase expansion leaves the sub-placeholders in place, which is what lets the
// removal passes drop `DEGREE` when the entry has none.
func TestExpandPhrases(t *testing.T) {
	got := process.ExpandPhrases(
		map[string]string{"m": "**INSTITUTION**, DEGREE_WITH_AREA"},
		map[string]string{"degree_with_area": "DEGREE in AREA"},
	)
	if got["m"] != "**INSTITUTION**, DEGREE in AREA" {
		t.Errorf("= %q", got["m"])
	}

	// The French catalog's, to show the connector is the locale's and not a
	// constant.
	got = process.ExpandPhrases(
		map[string]string{"m": "DEGREE_WITH_AREA"},
		map[string]string{"degree_with_area": "DEGREE en AREA"},
	)
	if got["m"] != "DEGREE en AREA" {
		t.Errorf("= %q", got["m"])
	}
}

// An empty-string field is dropped, so the removal passes clean up around it.
func TestEntryFieldsDropsEmptyStrings(t *testing.T) {
	got := process.EntryFields(map[string]string{"institution": "MIT", "area": ""})
	if _, present := got["AREA"]; present {
		t.Error("an empty field must count as not provided")
	}
	if got["INSTITUTION"] != "MIT" {
		t.Errorf("INSTITUTION = %q", got["INSTITUTION"])
	}
}

// The four skipped fields, plus the underscore rule.
func TestIsSkippedField(t *testing.T) {
	for _, name := range []string{"start_date", "end_date", "doi", "url", "_private"} {
		if !process.IsSkippedField(name) {
			t.Errorf("%s should be skipped", name)
		}
	}
	for _, name := range []string{"date", "summary", "highlights", "company"} {
		if process.IsSkippedField(name) {
			t.Errorf("%s should be processed", name)
		}
	}
}
