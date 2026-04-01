package process

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// blockLevelElements is python-markdown's `BLOCK_LEVEL_ELEMENTS`
// (`markdown/util.py:47-59`), the list its HTML preprocessor uses to decide
// whether a tag opens a raw block or is just markup inside a paragraph.
var blockLevelElements = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"details": true, "div": true, "dl": true, "fieldset": true,
	"figcaption": true, "figure": true, "footer": true, "form": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hgroup": true, "hr": true, "main": true, "menu": true,
	"nav": true, "ol": true, "p": true, "pre": true, "section": true,
	"table": true, "ul": true,
	// "Other elements which Markdown should not be mucking up the contents of."
	"canvas": true, "colgroup": true, "dd": true, "body": true, "dt": true,
	"group": true, "html": true, "iframe": true, "li": true, "legend": true,
	"math": true, "map": true, "noscript": true, "output": true,
	"object": true, "option": true, "progress": true, "script": true,
	"style": true, "summary": true, "tbody": true, "td": true,
	"textarea": true, "tfoot": true, "th": true, "thead": true, "tr": true,
	"video": true, "center": true,
}

// openingTag matches a line that begins with an HTML tag, capturing its name.
var openingTag = regexp.MustCompile(`^</?([A-Za-z][A-Za-z0-9-]*)`)

// blockLevelHTMLParser narrows goldmark's HTML block parser to the tags
// python-markdown treats as blocks.
//
// CommonMark's HTML block type 7 opens a raw block for **any** tag standing
// alone on a line, so `<img src="…" />` and `<not-an-email>` came out bare where
// upstream leaves them inside a paragraph:
//
//	<img src="a@b.png" />  →  <p><img src="a@b.png" /></p>   not the tag alone
//
// python-markdown's `HTMLBlockPreprocessor` asks `BLOCK_LEVEL_ELEMENTS` instead,
// and anything not on that list stays inline markup in a paragraph. A one-line
// CV field holding a single inline tag is what reaches this.
//
// Everything else is delegated, including the comment, processing-instruction,
// declaration and CDATA forms, which carry no tag name to look up.
type blockLevelHTMLParser struct {
	parser.BlockParser
}

// Open declines a non-block-level tag and otherwise defers to goldmark.
func (p blockLevelHTMLParser) Open(
	parent ast.Node, reader text.Reader, pc parser.Context,
) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	if match := openingTag.FindSubmatch(line); match != nil && !blockLevelElements[lower(match[1])] {
		return nil, parser.NoChildren
	}
	return p.BlockParser.Open(parent, reader, pc)
}

func lower(name []byte) string {
	out := make([]byte, len(name))
	for i, c := range name {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// blankLineRe is `htmlparser.py:93`'s `blank_line_re`, applied to the source
// **from just after the raw block's closing tag**.
var blankLineRe = regexp.MustCompile(`^(?:[ ]*\n){2}`)

// htmlBlockRenderer gives a raw HTML block the trailing newline upstream keeps
// inside the stash, so a block that follows one is separated by a **blank
// line** and not by the single newline every other block boundary gets.
//
// # Why there are two newlines here and one everywhere else
//
// A raw block does not travel through the tree as markup. `HTMLBlockPreprocessor`
// (`preprocessors.py:86-91`) hands the source to `HTMLExtractor`, which replaces
// each raw block with an opaque placeholder and keeps the text in
// `md.htmlStash`. When the outermost tag closes, `htmlparser.py:242-244` appends
// **a newline to the stashed text itself** — its comment is "Preserve blank line
// and end of raw block" — if `blank_line_re` (`:93`, `^([ ]*\n){2}`) matches what
// follows the closing tag. `NormalizeWhitespace` has already appended `"\n\n"` to
// the document (`preprocessors.py:73`), so a raw block at the end of the input
// satisfies it too.
//
// That newline is part of the *stashed string*. `RawHtmlPostprocessor`
// (`postprocessors.py:83-86`) substitutes it back into the already-serialized
// document verbatim, and it therefore arrives **in addition to** the ordinary
// inter-block separator, which `PrettifyTreeprocessor` writes as the element's
// tail (`treeprocessors.py:421, 432-433`). One newline from the tree, one from
// the stash: a blank line.
//
//	<div>block</div>\n\nafter   →  <div>block</div>\n\n<p>after</p>
//	after\n\n<div>block</div>   →  <p>after</p>\n<div>block</div>
//
// The second is not an exception. The stash newline is there too, at the very
// end of the document, where `Markdown.convert`'s closing `output.strip()`
// (`core.py:392`) removes it — as `MarkdownToHTML`'s own `TrimRight` does here.
//
// goldmark writes the block's source lines with their own newlines, which
// supplies the tree's separator and nothing else, so the extra one is what this
// renderer adds. **The condition is `blank_line_re`, not "has a next sibling".**
// The two agree on every shape measured, and only the first is upstream's rule.
type htmlBlockRenderer struct {
	inner renderer.NodeRendererFunc
}

// RegisterFuncs claims the HTML block node, and only that one.
func (r htmlBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
}

func (r htmlBlockRenderer) renderHTMLBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	status, err := r.inner(w, source, node, entering)
	if entering || err != nil {
		return status, err
	}
	if blankLineAfterRawBlock(source, node.(*ast.HTMLBlock)) {
		_ = w.WriteByte('\n')
	}
	return status, nil
}

// blankLineAfterRawBlock is `blank_line_re.match(rawdata[after the end tag:])`.
//
// The offset is measured from the closing tag, so the newline ending the tag's
// own line is the match's first `[ ]*\n` — goldmark's segments carry that
// newline, so it is stepped back over rather than reconstructed. The `"\n\n"`
// `NormalizeWhitespace` appends is supplied here for the same reason it is
// there: without it a raw block at the end of the input would not match.
func blankLineAfterRawBlock(source []byte, block *ast.HTMLBlock) bool {
	stop := len(source)
	if block.HasClosure() {
		stop = block.ClosureLine.Stop
	} else if lines := block.Lines(); lines.Len() > 0 {
		stop = lines.At(lines.Len() - 1).Stop
	}
	if stop > len(source) {
		stop = len(source)
	}
	if stop > 0 && source[stop-1] == '\n' {
		stop--
	}
	return blankLineRe.MatchString(string(source[stop:]) + "\n\n")
}
