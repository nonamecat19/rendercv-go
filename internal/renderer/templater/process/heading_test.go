package process_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// TestHTMLHeadingStripsEveryPythonSpace sweeps `HashHeaderProcessor`'s strip
// (`blockprocessors.py:479`) over all 29 characters `str.strip()` removes, at
// every heading level.
//
// The level is not an independent axis of the rule — `h%d` and the header group
// come out of one match of one regex (`:461`, `:478-479`) — but it is free to
// check, and a wrapper that trimmed only what it recognised as an `<h1>` would
// be caught here.
//
// `\n` and `\r` are the two characters that make it a different document: they
// end the heading line, so the text after them is a paragraph of its own, and
// that is upstream's answer too. Each `want` below is measured through the
// vendored `markdown.markdown`.
func TestHTMLHeadingStripsEveryPythonSpace(t *testing.T) {
	for level := 1; level <= 6; level++ {
		t.Run(fmt.Sprintf("h%d", level), func(t *testing.T) {
			hashes := strings.Repeat("#", level)
			for _, r := range pythonSpaces {
				in := fmt.Sprintf("%s %sh%s", hashes, string(r), string(r))
				want := fmt.Sprintf("<h%d>h</h%d>", level, level)
				if r == '\n' || r == '\r' {
					// The heading ends at the line break and `h` is the next
					// block, in both libraries.
					want = fmt.Sprintf("<h%d></h%d>\n<p>h</p>", level, level)
				}
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

// TestHTMLHeadingStripsBothEnds is the difference between this rule and the
// paragraph's, which is an `lstrip` (`paragraph.go`).
//
// A heading keeps nothing at either end, and a heading of nothing but
// whitespace is `<h1></h1>` rather than a heading of that whitespace. Every
// `want` is upstream's.
func TestHTMLHeadingStripsBothEnds(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"leading", "# \vh", "<h1>h</h1>"},
		{"trailing", "# h\v", "<h1>h</h1>"},
		{"both", "# \vh\v", "<h1>h</h1>"},
		{"space and more", "# \v h \v", "<h1>h</h1>"},
		{"nothing but whitespace", "# \v", "<h1></h1>"},
		{"interior is kept", "# a\vb", "<h1>a\vb</h1>"},
		{"between two paragraphs", "a\n\n# \vh\n\nb", "<p>a</p>\n<h1>h</h1>\n<p>b</p>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := process.MarkdownToHTML(test.in)
			if err != nil {
				t.Fatalf("MarkdownToHTML(%q): %v", test.in, err)
			}
			if got != test.want {
				t.Errorf("MarkdownToHTML(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
