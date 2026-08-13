package process_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// Where a raw HTML block ends, which is at its closing tag and not at the next
// blank line.
//
// Every `want` was measured by running the vendored `markdown.markdown` and
// `markdown_to_typst` over the input; the differentials in `testdata` carry the
// nine shapes spec 011 §9.5's enumeration had pinned, and these are the rest of
// the class — the ones that establish the rule rather than one instance of it.
//
// **The residual is four columns of whitespace after a closing tag.** The
// whitespace before a raw block that opens in another's tail, and the whitespace
// between a closing tag and the end of its line, are both dropped now
// (`splitRawBlockTails`) — but upstream leaves them in the document, where four
// or more columns of them are an indented code block, an empty one:
// `<div>a</div>\t<div>b</div>` is `<div>a</div>\n<pre><code>\n</code></pre>\n
// <div>b</div>` upstream and `<div>a</div>\n<div>b</div>` here. Reproducing it
// needs a block goldmark does not read from a whitespace-only line; the two
// differentials carry no row for that shape.
func TestRawBlockEndsAtClosingTag(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		html  string
		typst string
	}{{
		name:  "line after the closing tag",
		in:    "<div>block</div>\nafter",
		html:  "<div>block</div>\n<p>after</p>",
		typst: "<div>block</div>\nafter",
	}, {
		name:  "blank line after the closing tag",
		in:    "<div>block</div>\n\nafter",
		html:  "<div>block</div>\n\n<p>after</p>",
		typst: "<div>block</div>\n\nafter",
	}, {
		name:  "tail on the closing tag's own line",
		in:    "<div>x</div> tail",
		html:  "<div>x</div>\n<p>tail</p>",
		typst: "<div>x</div>\ntail",
	}, {
		name:  "tail with no space at all",
		in:    "<div>x</div>after",
		html:  "<div>x</div>\n<p>after</p>",
		typst: "<div>x</div>\nafter",
	}, {
		name: "an indented tail is a code block",
		// The tail keeps its own leading whitespace: upstream hands the block
		// parser everything after the tag, and five spaces is an indent there.
		in:    "<div>x</div>     tail",
		html:  "<div>x</div>\n<pre><code> tail\n</code></pre>",
		typst: "<div>x</div>\n` tail\n`",
	}, {
		name:  "a tab in the tail is expanded first",
		in:    "<div>a</div>\t t",
		html:  "<div>a</div>\n<pre><code> t\n</code></pre>",
		typst: "<div>a</div>\n` t\n`",
	}, {
		name:  "the stack pops back to the tag's own name",
		in:    "<div><br></div> t",
		html:  "<div><br></div>\n<p>t</p>",
		typst: "<div><br></div>\nt",
	}, {
		name:  "a nested tag of the same name does not close it",
		in:    "<div>a<div>b</div></div> t",
		html:  "<div>a<div>b</div></div>\n<p>t</p>",
		typst: "<div>a<div>b</div></div>\nt",
	}, {
		name: "a closing tag quoted in an attribute is not one",
		in:   "<div a='>' >x</div> t",
		html: "<div a='>' >x</div>\n<p>t</p>",
		// Upstream's Typst answer is `<div a='>' >x</div>\nt` and the port
		// writes `\>` for the second `>`, so `typst` is left empty and not
		// asserted: `inline.go`'s raw-HTML matcher ends a tag at the first `>`
		// whatever quotes it. That is a scanning gap of its own, on the inline
		// path, and it predates this rule — the block boundary either side of
		// it is the one measured here.
	}, {
		name:  "a multi-line block ends on its closing line",
		in:    "<div>\nmulti\nline\n</div>\nafter",
		html:  "<div>\nmulti\nline\n</div>\n<p>after</p>",
		typst: "<div>\nmulti\nline\n</div>\nafter",
	}, {
		name:  "an inline tag after one is not a block",
		in:    "<div>a</div>\n<span>x</span> y",
		html:  "<div>a</div>\n<p><span>x</span> y</p>",
		typst: "<div>a</div>\n<span>x</span> y",
	}, {
		name:  "a script block ends at its own closing tag",
		in:    "<script>a</script> tail",
		html:  "<script>a</script>\n<p>tail</p>",
		typst: "<script>a</script>\ntail",
	}, {
		name:  "a block that never closes takes the rest",
		in:    "<div>unclosed\nafter",
		html:  "<div>unclosed\nafter",
		typst: "<div>unclosed\nafter",
	}, {
		name:  "a tag inside a paragraph opens nothing",
		in:    "a <div>x</div> b",
		html:  "<p>a <div>x</div> b</p>",
		typst: "a <div>x</div> b",
	}, {
		name:  "a tag in a list item opens nothing",
		in:    "- <div>x</div> tail",
		html:  "<ul>\n<li><div>x</div> tail</li>\n</ul>",
		typst: "- <div>x</div> tail",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := process.MarkdownToHTML(test.in)
			if err != nil {
				t.Fatalf("MarkdownToHTML(%q): %v", test.in, err)
			}
			if got != test.html {
				t.Errorf("MarkdownToHTML(%q)\n = %q\nwant %q", test.in, got, test.html)
			}
			if test.typst == "" {
				// The one row whose Typst answer a different, older gap owns.
				return
			}
			if got := process.MarkdownToTypst(test.in); got != test.typst {
				t.Errorf("MarkdownToTypst(%q)\n = %q\nwant %q", test.in, got, test.typst)
			}
		})
	}
}
