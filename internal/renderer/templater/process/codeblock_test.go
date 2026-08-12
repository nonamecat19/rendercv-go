package process_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// pythonSpaces is `str.rstrip()`'s character set: the 25 Go calls
// `unicode.IsSpace` plus the four C0 separators U+001C–U+001F.
var pythonSpaces = []rune{
	0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004,
	0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029,
	0x202f, 0x205f, 0x3000,
}

// TestHTMLCodeBlockSurvivesEveryPythonSpaceLine walks the same 29 characters as
// a whole line **between** two indented ones.
//
// `detab` empties such a line and keeps the block open
// (`blockprocessors.py:96-99`), so all 29 are one `<pre>` upstream — but
// goldmark's blank-line predicate is only `\t`, `\n`, `\r` and a space
// (`util/util.go:806`), and before `pythonCodeBlockParser` the other 25 ended
// the block and opened a paragraph.
//
// U+000A is the one that differs, and it differs upstream too: it makes a second
// blank line, and `'    a\n\n\n    b'.split('\n\n')` leaves one of them at the
// head of the next chunk rather than dropping it.
func TestHTMLCodeBlockSurvivesEveryPythonSpaceLine(t *testing.T) {
	for _, r := range pythonSpaces {
		want := "<pre><code>a\n\nb\n</code></pre>"
		if r == '\n' {
			want = "<pre><code>a\n\n\nb\n</code></pre>"
		}
		in := "    a\n" + string(r) + "\n    b"
		got, err := process.MarkdownToHTML(in)
		if err != nil {
			t.Fatalf("MarkdownToHTML(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHTMLCodeBlockEmptiedLineIsNotAChunkBoundary is the distinction
// `pythonCodeBlockParser` records and `codeBlockText` reads back.
//
// A line `detab` empties sits *inside* a chunk, because `parseChunk` cut the
// document before `detab` ran (`blockparser.py:136`). One chunk is one
// `rstrip`, so `a  ` keeps its trailing spaces here and loses them when the
// separating line is genuinely blank. Both outputs are upstream's.
func TestHTMLCodeBlockEmptiedLineIsNotAChunkBoundary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"emptied line", "    a  \n\v\n    b  ", "<pre><code>a  \n\nb\n</code></pre>"},
		{"blank line", "    a  \n\n    b  ", "<pre><code>a\n\nb\n</code></pre>"},
		{"spaces-only line", "    a  \n   \n    b  ", "<pre><code>a\n\nb\n</code></pre>"},
		{"indented, so kept as text", "    a  \n    \v\n    b  ", "<pre><code>a  \n\v\nb\n</code></pre>"},
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

// TestHTMLCodeBlockEndsAtABlankLineBeforeAnEmptiedOne pins the third condition
// of `detabEmpties`, the one no fixture row can carry.
//
// A blank line ends the chunk, and `CodeBlockProcessor.test`
// (`blockprocessors.py:255`) then asks the *next* chunk to start with four
// spaces of its own — so `"    a\n\n\v\n    b"` is a code block of `a` and then
// a paragraph upstream, not one block of `a\n\n\nb`. The paragraph's own text
// still differs (`ParagraphProcessor` strips a `\v` this port keeps, spec 011's
// open work), so only the block is asserted.
func TestHTMLCodeBlockEndsAtABlankLineBeforeAnEmptiedOne(t *testing.T) {
	const in = "    a\n\n\v\n    b"
	got, err := process.MarkdownToHTML(in)
	if err != nil {
		t.Fatalf("MarkdownToHTML(%q): %v", in, err)
	}
	if !strings.HasPrefix(got, "<pre><code>a\n</code></pre>\n<p>") {
		t.Errorf("MarkdownToHTML(%q) = %q, want a code block of `a` and then a paragraph", in, got)
	}
}

// TestHTMLCodeBlockStripsEveryPythonSpace is the HTML-path twin of
// `TestCodeBlockStripsEveryPythonSpace`, and it exists for the same reason: the
// predicate is `str.rstrip()`'s 29 characters, not `unicode.IsSpace`'s 25.
//
// All 29 are `<pre><code>a\n</code></pre>` upstream — measured through the
// vendored `markdown.markdown`, one call per character, including the four C0
// separators U+001C–U+001F that Go's `White_Space` property does not carry.
// Unlike the Typst path, `\r` is no exception here: `MarkdownToHTML` runs
// `NormalizeWhitespace` over the whole document before parsing.
func TestHTMLCodeBlockStripsEveryPythonSpace(t *testing.T) {
	const want = "<pre><code>a\n</code></pre>"
	for _, r := range pythonSpaces {
		in := "    a" + string(r)
		got, err := process.MarkdownToHTML(in)
		if err != nil {
			t.Fatalf("MarkdownToHTML(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("MarkdownToHTML(%q) = %q, want %q", in, got, want)
		}
	}
}
