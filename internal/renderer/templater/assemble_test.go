package templater_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
)

// The assembly separators, measured by running `render_full_template`'s own
// arithmetic over stub fragments.
//
// Stubs rather than real templates on purpose: this is the one part of the
// renderer testable before a single template exists, and getting a separator
// wrong is a byte diff on **every** corpus case at once — a failure that says
// nothing about which of the two dozen templates is at fault. Pinning it here
// means the first golden failure is about a template.
func TestAssemble(t *testing.T) {
	tests := []struct {
		name     string
		sections []templater.RenderedSection
		typst    bool
		want     string
	}{
		{
			name:  "one section, two entries",
			typst: true,
			sections: []templater.RenderedSection{
				{Beginning: "B", Entries: []string{"E1", "E2"}, Ending: "X"},
			},
			// The entries are joined by **two** newlines and the section by one.
			want: "P\n\nH\n\nB\nE1\n\nE2\nX",
		},
		{
			// Markdown has no preamble at all, so it opens with the header.
			name:  "markdown has no preamble",
			typst: false,
			sections: []templater.RenderedSection{
				{Beginning: "B", Entries: []string{"E1"}, Ending: "X"},
			},
			want: "H\n\nB\nE1\nX",
		},
		{
			// No sections: the document is the opening and **does** keep its
			// trailing newline, which only this case shows.
			name:  "no sections",
			typst: true,
			want:  "P\n\nH\n",
		},
		{
			// An empty section still emits both separators, so two consecutive
			// newlines appear between the beginning and the ending.
			name:  "a section with no entries",
			typst: true,
			sections: []templater.RenderedSection{
				{Beginning: "B", Ending: "X"},
			},
			want: "P\n\nH\n\nB\n\nX",
		},
		{
			// Two sections: the second's leading newline is what separates them,
			// and there is **no** blank line between one ending and the next
			// beginning.
			name:  "two sections",
			typst: true,
			sections: []templater.RenderedSection{
				{Beginning: "B1", Entries: []string{"E"}, Ending: "X1"},
				{Beginning: "B2", Entries: []string{"E"}, Ending: "X2"},
			},
			want: "P\n\nH\n\nB1\nE\nX1\nB2\nE\nX2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := templater.Assemble("P", "H", tc.sections, tc.typst)
			if got != tc.want {
				t.Errorf("=\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// The document has **no trailing newline** beyond whatever its last section
// ends with — each section is appended with a *leading* newline, not a trailing
// one. A port that appended trailing newlines instead passes every row above
// except this one.
func TestAssembleHasNoTrailingNewline(t *testing.T) {
	got := templater.Assemble("P", "H", []templater.RenderedSection{
		{Beginning: "B", Entries: []string{"E"}, Ending: "X"},
	}, true)

	if got[len(got)-1] == '\n' {
		t.Errorf("= %q, want no trailing newline", got)
	}
}
