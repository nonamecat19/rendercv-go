package process_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// TestHTMLParagraphAppliesPythonsWhitespaceSet sweeps `ParagraphProcessor.run`'s
// two string operations over all 29 characters `str.strip()` removes.
//
// `html.json` carries four of the characters across every shape; this carries
// every character across the shapes, which is the axis a fixture row per
// combination would only repeat 600 times. Each `want` is upstream's, measured
// through the vendored `markdown.markdown` one call per cell.
//
// The two arms are `blockprocessors.py:614` — a block that is entirely
// whitespace is thrown away — and `:641`/`:637`, `lstrip` on the block that is
// kept. `\v` and `\f` name the class in spec 011, `\x1c`-`\x1f` are outside Go's
// own `unicode.IsSpace`, and `\xa0` is the one an ordinary CV reaches.
func TestHTMLParagraphAppliesPythonsWhitespaceSet(t *testing.T) {
	tests := []struct {
		name  string
		shape string
		want  func(rune) string
		// skip names the characters for which the shape is a different
		// document upstream, for a reason that is not this rule.
		skip map[rune]string
	}{
		{
			name:  "a whitespace-only document is empty",
			shape: "%s",
			want:  func(rune) string { return "" },
		},
		{
			name:  "a whitespace-only chunk is thrown away",
			shape: "a\n\n%s\n\nb",
			want:  func(rune) string { return "<p>a</p>\n<p>b</p>" },
		},
		{
			name:  "a leading run is stripped",
			shape: "%sa",
			want: func(r rune) string {
				if r == '\t' {
					// `expandtabs` makes it four spaces, and four spaces is an
					// indented code block (`preprocessors.py:73`).
					return "<pre><code>a\n</code></pre>"
				}
				return "<p>a</p>"
			},
		},
		{
			name:  "the strip runs across the newline",
			shape: "%s\na",
			want:  func(rune) string { return "<p>a</p>" },
			skip: map[rune]string{
				'\t': "the expanded tab is an empty code block, not a paragraph",
			},
		},
		{
			name:  "a tight list item is stripped too",
			shape: "- %sx",
			want:  func(rune) string { return "<ul>\n<li>x</li>\n</ul>" },
			skip: map[rune]string{
				'\n': "an item whose first line is empty, then a lazy continuation",
				'\r': "an item whose first line is empty, then a lazy continuation",
			},
		},
		{
			name:  "a blockquote's paragraph is stripped too",
			shape: "> %sq",
			want:  func(rune) string { return "<blockquote>\n<p>q</p>\n</blockquote>" },
			skip: map[rune]string{
				'\n': "an empty quote, then a paragraph outside it",
				'\r': "an empty quote, then a paragraph outside it",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, r := range pythonSpaces {
				if _, skipped := test.skip[r]; skipped {
					continue
				}
				in := strings.ReplaceAll(test.shape, "%s", string(r))
				got, err := process.MarkdownToHTML(in)
				if err != nil {
					t.Fatalf("MarkdownToHTML(%q): %v", in, err)
				}
				if want := test.want(r); got != want {
					t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
				}
			}
		})
	}
}

// TestHTMLParagraphStripsOnlyTheHeadOfTheBlock is the boundary of the rule
// above, and the reason it is not a strip per line.
//
// `p.text = block.lstrip()` runs once over a block that still has its newlines
// in it (`blockprocessors.py:641`), so it eats a whole first line but stops at
// the first character that is not whitespace and never touches the lines below.
// Every `want` is upstream's.
func TestHTMLParagraphStripsOnlyTheHeadOfTheBlock(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a trailing run stays", "a\v", "<p>a\v</p>"},
		{"an interior line keeps its own", "a\n\vb", "<p>a\n\vb</p>"},
		{"an interior indent stays", "a\n    b", "<p>a\n    b</p>"},
		{"every leading line goes", "\v\n\v\na", "<p>a</p>"},
		{"a run of mixed whitespace goes", "\v  a", "<p>a</p>"},
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

// TestHTMLBlankParagraphUnderASetextBarIsAHeading is the boundary of
// `:614`'s throw-away, and the shape that showed the rule needs one.
//
// `SetextHeaderProcessor` is registered above `ParagraphProcessor`
// (`blockprocessors.py:493`), so a chunk whose second line is a bar is a
// heading before it is ever a block to discard — `"\v\n==="` is `<h1></h1>`,
// not nothing. goldmark reaches the same verdict one line later, at the bar,
// and needs the paragraph still in the tree to do it.
func TestHTMLBlankParagraphUnderASetextBarIsAHeading(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"equals bar", "\v\n===", "<h1></h1>"},
		{"dash bar", "\v\n---", "<h2></h2>"},
		{"padded bar", "\v\n===  ", "<h1></h1>"},
		{"a blank line between is not a heading", "\v\n\n===", "<p>===</p>"},
		{"and an ordinary blank block is still dropped", "a\n\n\v\n\nb", "<p>a</p>\n<p>b</p>"},
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
