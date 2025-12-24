package process_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// The markdown→Typst differential: 101 inputs and the vendored Python's answer
// for each, captured into testdata rather than restated here.
//
// **The fixture is generated, and what that does and does not buy is the same
// trade `tools/localeprobe` states.** It was produced by running upstream's
// `markdown_to_typst` over the input list, so it pins the port against a
// measurement rather than against my reading — but it cannot notice an input
// nobody thought to add. The golden `.typ` diff is the gate that can.
//
// The inputs were not chosen for coverage of the parser's code; they were chosen
// by running the port and adding whatever disagreed, which is why the list
// includes `****` and `_**a**_`.
func TestMarkdownToTypst(t *testing.T) {
	var rows []struct {
		In   string `json:"in"`
		Want string `json:"want"`
	}

	path := filepath.Join("testdata", "markdown_to_typst.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(rows) < 100 {
		t.Fatalf("%d rows, want the full fixture", len(rows))
	}

	for _, row := range rows {
		t.Run(row.In, func(t *testing.T) {
			if got := process.MarkdownToTypst(row.In); got != row.Want {
				t.Errorf("MarkdownToTypst(%q) =\n%q\nwant\n%q", row.In, got, row.Want)
			}
		})
	}
}

// The findings the fixture encodes, named so a failure says which rule broke
// rather than which string differs.
func TestMarkdownToTypstFindings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// Five block processors are deregistered, so these are text.
			name: "a heading is literal text", in: "# Heading", want: `\# Heading`,
		},
		{"a bullet is literal text", "- item", "- item"},
		{"a numbered item is literal text", "1. item", "1. item"},
		{"a quote is literal text", "> quote", `\> quote`},
		{
			// But `hr` and `indent` are **not** deregistered.
			name: "a horizontal rule renders as nothing", in: "---", want: "",
		},
		{"an indented line is a code block", "    x", "`x\n`"},
		{
			// EMPHASIS_RE's body admits no asterisk, so this is three emphases
			// rather than one containing a strong.
			name: "a strong inside an emphasis splits it",
			in:   "*a **b** c*",
			want: "#emph[a ]#emph[b]#emph[ c]",
		},
		{
			// The per-processor cutoff: a `*` inside an `_` starts fresh.
			name: "asterisks nest inside underscores",
			in:   "_**a**_",
			want: "#emph[#strong[a]]",
		},
		{
			// The smart guards: a word character before the `_` blocks it.
			name: "an underscore inside a word is not emphasis",
			in:   "snake_case_word",
			want: `snake\_case\_word`,
		},
		{
			// Links run at every depth, not only at the top.
			name: "a link inside a strong",
			in:   "**[a](u)**",
			want: `#strong[#link("u")[a]]`,
		},
		{
			// An empty href becomes the example URL rather than an empty one.
			name: "an empty link URL", in: "[t]()", want: `#link("https://example.com")[t]`,
		},
		{
			// The admonition is the one multi-line construct, and its keyword
			// selects nothing — the title child is dropped.
			name: "an admonition becomes a summary",
			in:   "!!! note\n    a\n    b",
			want: `#summary[a \ b]`,
		},
		{
			// BACKTICK_RE's `\2` backreference: the closing run is as wide as
			// the opening one, so the second backtick here does not close the
			// span and the third is left as literal text.
			name: "a code span closes on a run of its own width",
			in:   "backtick `a ` b` here",
			want: "backtick `a` b` here",
		},
		{
			// The same rule the other way round: a wider opener swallows the
			// lone backtick inside it.
			name: "a double-backtick span may contain a backtick",
			in:   "``a ` b`` here",
			want: "`a ` b` here",
		},
		{
			// `BacktickInlineProcessor` strips the body.
			name: "a code span's body is stripped", in: "` a ` x", want: "`a` x",
		},
		{
			// An opener with no closing run of its width stays literal.
			name: "an unclosed code span is text", in: "`a`` x", want: "`a`` x",
		},
		{
			// `to_typst_string` has no `img` branch, so an image renders as
			// nothing — and the link pattern must not see the `[alt](src)` half
			// of it.
			name: "an image renders as nothing",
			in:   "image ![alt](src.png) here",
			want: "image  here",
		},
		{
			// NOIMG again: the `!` stays and the brackets are escaped when the
			// image construct is incomplete.
			name: "an incomplete image is not a link",
			in:   "img no close ![a] x",
			want: `img no close !\[a\] x`,
		},
		{
			// `escape` outranks `link`, so an escaped bang is not the bang the
			// NOIMG lookbehind is looking for.
			name: "an escaped bang leaves a link",
			in:   `not img \![a](s.png) x`,
			want: `not img !#link("s.png")[a] x`,
		},
		{
			// AUTOLINK_RE: the href is raw and the link text is escaped.
			name: "a URL autolink becomes a link",
			in:   "autolink <https://go.dev> here",
			want: `autolink #link("https://go.dev")[https:\/\/go.dev] here`,
		},
		{
			// AUTOMAIL_RE, and every character of the address is obfuscated —
			// decimal in the href, `codepoint2name` first in the text, which is
			// then escaped.
			name: "a mail autolink is obfuscated",
			in:   "mail <a@b> x",
			want: `mail #link("&#109;&#97;&#105;&#108;&#116;&#111;&#58;&#97;&#64;&#98;")` +
				`[&\#97;&\#64;&\#98;] x`,
		},
		{
			// autolink outranks automail, so a userinfo `@` stays a plain URL.
			name: "a userinfo at-sign is not a mail autolink",
			in:   "auto <http://a@b.com/x> y",
			want: `auto #link("http://a@b.com/x")[http:\/\/a\@b.com\/x] y`,
		},
		{
			// Line-by-line conversion: the asterisks do not pair across the
			// newline.
			name: "emphasis does not cross a line boundary",
			in:   "*a\nb*",
			want: "#sym.ast.basic#h(0pt, weak: true) a\nb#sym.ast.basic#h(0pt, weak: true)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := process.MarkdownToTypst(tc.in); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}
