package process

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

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
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(pythonBlockParsers()...),
		parser.WithInlineParsers(append(pythonInlineParsers(),
			util.Prioritized(automailParser{}, 250),
			util.Prioritized(imageParser{}, 150),
			util.Prioritized(emphasisParser{}, 450))...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		parser.WithASTTransformers(util.Prioritized(linkTitleSplitter{}, 100)),
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
			util.Prioritized(codeSpanRenderer{writer: pythonMarkdownWriter}, 101),
			util.Prioritized(linkRenderer{writer: pythonMarkdownWriter}, 103),
			util.Prioritized(textRenderer{writer: pythonMarkdownWriter, inner: defaultNodeRendererFunc(ast.KindText, defaultRenderer)}, 104),
			util.Prioritized(blockRenderer{
				writer: pythonMarkdownWriter,
				inner: map[ast.NodeKind]renderer.NodeRendererFunc{
					ast.KindParagraph: defaultNodeRendererFunc(ast.KindParagraph, defaultRenderer),
					ast.KindTextBlock: defaultNodeRendererFunc(ast.KindTextBlock, defaultRenderer),
				},
			}, 105),
			util.Prioritized(autoLinkRenderer{inner: defaultNodeRendererFunc(ast.KindAutoLink, defaultRenderer)}, 102),
		),
	),
)

// defaultRenderer is goldmark's own HTML renderer, kept so the node renderers
// here can delegate the parts the two libraries already agree on.
var defaultRenderer = html.NewRenderer(
	html.WithUnsafe(), html.WithXHTML(), html.WithWriter(pythonMarkdownWriter))

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
	source := flattenShallowLists(normalizeWhitespace(markdown))
	if err := converter.Convert([]byte(source), &out); err != nil {
		return "", err
	}
	// goldmark ends the document with a newline; upstream's `markdown.markdown`
	// returns the body without one, and `Full.html` supplies the layout.
	return strings.TrimRight(out.String(), "\n"), nil
}

// pythonBlockParsers is goldmark's default block parser set with two changes
// upstream forces: no fenced code, and a narrowed raw-HTML block.
//
// **Fenced code has no counterpart in `markdown.markdown(string)`.**
//
// It is an *extension* in python-markdown (`markdown/extensions/
// fenced_code.py`) and `markdown_to_html` enables none (`markdown_parser.py:202`
// — contrast the Typst instance at :147, which asks for `admonition`). So a
// ``` fence is ordinary paragraph text there, and its backticks are read as
// **inline code spans**: ```` ```\ncode\n``` ```` is `<p><code>code</code></p>`
// upstream against goldmark's `<pre><code>`. Indented code blocks stay — those
// are core Markdown and python-markdown has them.
//
// The parsers are goldmark singletons (`parser.NewFencedCodeBlockParser` and
// `parser.NewHTMLBlockParser` return package-level values), so identity is the
// test.
func pythonBlockParsers() []util.PrioritizedValue {
	fenced := parser.NewFencedCodeBlockParser()
	htmlBlock := parser.NewHTMLBlockParser()

	kept := make([]util.PrioritizedValue, 0, len(parser.DefaultBlockParsers()))
	for _, p := range parser.DefaultBlockParsers() {
		switch p.Value {
		case fenced:
		case htmlBlock:
			kept = append(kept, util.Prioritized(
				blockLevelHTMLParser{BlockParser: htmlBlock}, p.Priority))
		default:
			kept = append(kept, p)
		}
	}
	return kept
}

// pythonInlineParsers is goldmark's default inline parser set minus its own
// emphasis parser — `emphasisParser` (spec 011 §7, `emphasis_html.go`) replaces
// it rather than merely outranking it, since python-markdown's ordered-pattern
// algorithm and CommonMark's delimiter stack can each accept a position the
// other declines, and letting both run would let goldmark's own algorithm
// answer for cases this port's own patterns turn down.
//
// The default emphasis parser is a goldmark singleton (`NewEmphasisParser`
// returns a package-level value), so identity is the test — the same pattern
// `pythonBlockParsers` uses for the fenced-code and raw-HTML-block parsers.
func pythonInlineParsers() []util.PrioritizedValue {
	emphasis := parser.NewEmphasisParser()

	kept := make([]util.PrioritizedValue, 0, len(parser.DefaultInlineParsers()))
	for _, p := range parser.DefaultInlineParsers() {
		if p.Value == emphasis {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// normalizeWhitespace is python-markdown's `NormalizeWhitespace` preprocessor
// (`markdown/preprocessors.py:66-73`), which runs before any parsing and does
// two things: it folds both `\r\n` and a lone `\r` into `\n`, and it expands
// tabs.
//
// goldmark treats a lone `\r` as an ordinary character, so `a\rb` stayed one line
// and a `\r`-separated list stayed one item — a CV pasted from a Windows editor
// reaches that. And it keeps a tab as a tab, so an indented line came out with
// the tab still in it against upstream's spaces.
func normalizeWhitespace(markdown string) string {
	if strings.ContainsRune(markdown, '\r') {
		markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
		markdown = strings.ReplaceAll(markdown, "\r", "\n")
	}
	return expandTabs(markdown)
}

// expandTabs is Python's `str.expandtabs(4)`: a tab advances to the next
// multiple of `tab_length`, counted from the start of the line, so its width
// depends on what precedes it and a blind replacement with four spaces is wrong.
func expandTabs(markdown string) string {
	if !strings.ContainsRune(markdown, '\t') {
		return markdown
	}
	var out strings.Builder
	column := 0
	for _, r := range markdown {
		switch r {
		case '\t':
			width := pythonMarkdownTabLength - column%pythonMarkdownTabLength
			out.WriteString(strings.Repeat(" ", width))
			column += width
		case '\n':
			out.WriteRune(r)
			column = 0
		default:
			out.WriteRune(r)
			column++
		}
	}
	return out.String()
}

// flattenShallowLists moves every list marker indented by less than a tab length
// to column 0, together with every **block** that starts there under-indented,
// which is what python-markdown's list processor effectively does with both.
//
// The second half is the same `tab_length` rule seen from the other side.
// `ListIndentProcessor` claims a block that follows a blank line as a child of
// the item above it only when it is indented by a full 4, so
//
//   - item
//
//     continued
//
// is a list and then a **separate top-level paragraph** upstream, where
// CommonMark keeps `continued` inside the item. Two spaces is what a person
// naturally types to line text up under a bullet, so a CV reaches this.
//
// The rule is not restricted to lists because upstream's is not: a paragraph
// opening at one to three spaces is dedented wherever it appears.
func flattenShallowLists(markdown string) string {
	lines := strings.Split(markdown, "\n")
	afterBlank := true
	for i, line := range lines {
		blank := strings.TrimSpace(line) == ""
		indent := len(line) - len(strings.TrimLeft(line, " "))

		switch {
		case blank:
		case afterBlank && indent > 0 && indent < pythonMarkdownTabLength:
			lines[i] = line[indent:]
		default:
			match := listMarker.FindStringSubmatch(line)
			if match != nil && len(match[1]) < pythonMarkdownTabLength {
				lines[i] = line[len(match[1]):]
			}
		}
		afterBlank = blank
	}
	return strings.Join(lines, "\n")
}
