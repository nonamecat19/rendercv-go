package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

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
	pythonSpaces := []rune{
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		0x85, 0xa0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004,
		0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029,
		0x202f, 0x205f, 0x3000,
	}

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
