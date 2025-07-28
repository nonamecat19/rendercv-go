package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// A Markdown link title is stripped, as python-markdown strips it.
//
// Passing it through emitted `#link("u "ti"")[t]` — an unbalanced Typst string
// literal, so the whole document failed to compile. The corpus has no link
// title; an audit found it.
func TestLinkTitleIsStripped(t *testing.T) {
	for _, row := range []struct{ in, want string }{
		{`[text](https://example.com "the title")`, `#link("https://example.com")[text]`},
		{`[t](u "ti")`, `#link("u")[t]`},
		{`[t](u 'ti')`, `#link("u")[t]`},
		{`[t](u)`, `#link("u")[t]`},
	} {
		if got := process.MarkdownToTypst(row.in); got != row.want {
			t.Errorf("MarkdownToTypst(%q) = %q, want %q", row.in, got, row.want)
		}
	}
}
