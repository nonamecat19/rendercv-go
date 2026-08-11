package process

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// converter is goldmark configured to match python-markdown's defaults where it
// can.
//
// **`WithUnsafe` is not a security decision here, it is a parity one.**
// python-markdown passes raw HTML through; goldmark replaces it with
// `<!-- raw HTML omitted -->` unless told otherwise, so a `<b>` in a CV summary
// vanished and a `<tag>` in ordinary prose took its surrounding text with it. A
// verifier measured both. The input is the user's own CV, which the port
// already renders verbatim into Typst, so passing it through is what the rest of
// the pipeline does too.
// listMarker is a list item's indentation and bullet.
var listMarker = regexp.MustCompile(`^( +)([-*+] |\d+\. )`)

// pythonMarkdownTabLength is `markdown.Markdown`'s `tab_length` default, which
// decides whether an indented list item is a **child or a sibling**
// (`markdown/blockprocessors.py`, `ListIndentProcessor.tab_length`).
const pythonMarkdownTabLength = 4

// converter is goldmark configured to match python-markdown where it can.
//
// **`WithUnsafe` is a parity decision, not a security one.** python-markdown
// passes raw HTML through; goldmark replaces it with `<!-- raw HTML omitted -->`
// otherwise, so a `<b>` in a summary vanished and a `<tag>` in prose took its
// surrounding text with it. The input is the user's own CV, which the port
// already renders verbatim into Typst.
// Its escaping is python-markdown's, not goldmark's, which needs a writer and
// one node renderer — see `htmlescape.go` for the three-context rule and for why
// an earlier attempt that wrapped only the writer was reverted.
var converter = goldmark.New(
	goldmark.WithParserOptions(parser.WithASTTransformers(
		util.Prioritized(linkTitleSplitter{}, 100),
	)),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
		// python-markdown's serializer is XHTML: `<br />`, `<hr />`, `<img … />`
		// (measured on all three). goldmark writes HTML5 void elements by
		// default.
		html.WithXHTML(),
		html.WithWriter(pythonMarkdownWriter),
		renderer.WithNodeRenderers(
			// Below the default HTML renderer's 1000, which is what lets this one
			// win: goldmark registers node renderers from the end of the sorted
			// list backwards, so the lowest priority is registered last.
			util.Prioritized(imageRenderer{writer: pythonMarkdownWriter}, 100),
		),
	),
)

// pythonMarkdownWriter is shared between the converter and the image renderer so
// the attribute cell and the text cell cannot drift apart.
var pythonMarkdownWriter = pythonWriter{inner: html.DefaultWriter}

// MarkdownToHTML is `markdown_to_html` (markdown_parser.py:193-202), which is
// `markdown.markdown(string)` with no extensions and no configuration.
//
// **The engine is goldmark, and one rule of upstream's is applied to its input
// rather than to its output.** python-markdown nests a list item only when it is
// indented by a full `tab_length` of 4; CommonMark — and so goldmark — nests at
// 2. The entry templates emit their nested highlights at **2** spaces, so
// upstream *flattens* them into siblings and goldmark nests them. Measured over
// the 24 corpus documents: goldmark alone matches 8, and matches all 24 once the
// under-indented markers are moved to column 0 first.
//
// That is a normalization of the *input* to upstream's own list rule, not a
// rewrite of goldmark's output. It is the whole difference between the two
// libraries on this corpus, which is the answer to the question spec 011 §6
// posed — and it is the opposite of the answer this port first recorded, from
// reading the diff rather than reducing it.
func MarkdownToHTML(markdown string) (string, error) {
	var out bytes.Buffer
	source := flattenShallowLists(normalizeNewlines(markdown))
	if err := converter.Convert([]byte(source), &out); err != nil {
		return "", err
	}
	// goldmark ends the document with a newline; upstream's `markdown.markdown`
	// returns the body without one, and `Full.html` supplies the layout.
	return strings.TrimRight(out.String(), "\n"), nil
}

// normalizeNewlines is python-markdown's `NormalizeWhitespace` preprocessor
// (`markdown/preprocessors.py:66-72`), which runs before any parsing and folds
// both `\r\n` and a lone `\r` into `\n`.
//
// goldmark treats a lone `\r` as an ordinary character, so `a\rb` stayed one
// line and a `\r`-separated list stayed one item. A CV pasted from a Windows
// editor reaches this.
func normalizeNewlines(markdown string) string {
	if !strings.ContainsRune(markdown, '\r') {
		return markdown
	}
	return strings.ReplaceAll(strings.ReplaceAll(markdown, "\r\n", "\n"), "\r", "\n")
}

// flattenShallowLists moves every list marker indented by less than a tab length
// to column 0, which is what python-markdown's list processor effectively does
// with it.
func flattenShallowLists(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		match := listMarker.FindStringSubmatch(line)
		if match == nil || len(match[1]) >= pythonMarkdownTabLength {
			continue
		}
		lines[i] = line[len(match[1]):]
	}
	return strings.Join(lines, "\n")
}
