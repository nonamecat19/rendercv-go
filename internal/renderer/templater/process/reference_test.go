package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// The rules `reference.go` encodes, named so a failure says which one broke
// rather than which string differs. Every `want` here is the vendored Python's
// answer, taken from the same differential that fills
// `testdata/markdown_to_typst.json`.
//
// **Every id is unique across the whole file, and that is not tidiness.**
// `md.references` is never reset (`markdown/core.py:263-273`), so a definition
// one case writes is visible to every case after it, in this process as in
// upstream's. Reusing an id would make a case's result depend on the order the
// table happens to run in.
func TestMarkdownToTypstReferences(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// The half a CV hits first: the definition is recorded and its line
			// renders as nothing at all.
			name: "a definition emits no element",
			in:   "[r1]: https://e.com",
			want: "",
		},
		{
			name: "the full form resolves",
			in:   "[r2]: https://e.com\n[t][r2]",
			want: "\n#link(\"https://e.com\")[t]",
		},
		{
			// `if not id: id = text` (`inlinepatterns.py:917-918`).
			name: "an empty id falls back to the label",
			in:   "[r3]: https://e.com\n[r3][]",
			want: "\n#link(\"https://e.com\")[r3]",
		},
		{
			name: "the shortcut form resolves",
			in:   "[r4]: https://e.com\nsee [r4] now",
			want: "\nsee #link(\"https://e.com\")[r4] now",
		},
		{
			// The definition lowers and strips; the use lowers and collapses.
			name: "ids are matched case-insensitively",
			in:   "[R5]: https://e.com\n[t][r5]",
			want: "\n#link(\"https://e.com\")[t]",
		},
		{
			// Parsed so that it is consumed, then dropped: `to_typst_string`'s
			// `a` branch reads only `href`.
			name: "a title changes nothing",
			in:   "[r6]: https://e.com \"Ti\"\n[t][r6]",
			want: "\n#link(\"https://e.com\")[t]",
		},
		{
			// An `img` takes `to_typst_string`'s default branch and has neither
			// text nor children, so it contributes the empty string — the tail
			// after it still does not.
			name: "an image reference contributes nothing",
			in:   "[r7]: https://e.com\n![alt][r7] tail",
			want: "\ntail",
		},
		{
			name: "an undefined reference stays literal",
			in:   "[t][never-defined]",
			want: `\[t\]\[never-defined\]`,
		},
		{
			// The line-at-a-time pass converts the use before the definition
			// exists, so the reference is literal and the definition is still
			// stripped. This is the shape measured end to end against upstream.
			name: "a use before its definition stays literal",
			in:   "[ref][r8]\n\n[r8]: https://e.com",
			want: "\\[ref\\]\\[r8\\]\n\n",
		},
		{
			// `reference` (170) declines the whole span, `short_reference`
			// (130) then claims the label and leaves the id bracket behind.
			name: "the short form still gets a span the full form declined",
			in:   "[r9]: https://e.com\n[r9][99]",
			want: "\n#link(\"https://e.com\")[r9]\\[99\\]",
		},
		{
			// The barrier: `short_reference` turned the outer span down, so no
			// reference pattern looks inside it again.
			name: "an undefined outer label shields the brackets inside it",
			in:   "[r10]: https://e.com\n[a[r10]b]",
			want: "\n" + `\[a\[r10\]b\]`,
		},
		{
			// `RE_LINK`'s `\s?` is one optional whitespace character.
			name: "one space between label and id is allowed and two are not",
			in:   "[r11]: https://e.com\n[t] [r11]\n[t]  [r11]",
			want: "\n#link(\"https://e.com\")[t]\n\\[t\\]  #link(\"https://e.com\")[r11]",
		},
		{
			// `reference` is 170 and `link` is 160, so the direct form wins the
			// position even when the label is a defined id.
			name: "an inline link outranks a shortcut reference",
			in:   "[r12]: https://e.com\n[r12](u)",
			want: "\n#link(\"u\")[r12]",
		},
		{
			// The label is re-parsed from `link` (160) down.
			name: "a full reference label may hold an inline link",
			in:   "[r13]: https://e.com\n[a [b](c) d][r13]",
			want: "\n#link(\"https://e.com\")[a #link(\"c\")[b] d]",
		},
		{
			// `code` (80) outranks `reference` (15) in the block registry.
			name: "an indented definition is a code block",
			in:   "    [r14]: https://e.com",
			want: "`[r14]: https://e.com\n`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := process.MarkdownToTypst(test.in); got != test.want {
				t.Errorf("MarkdownToTypst(%q) =\n%q\nwant\n%q", test.in, got, test.want)
			}
		})
	}
}
