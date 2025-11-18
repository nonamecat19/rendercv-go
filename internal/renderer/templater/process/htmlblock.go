package process

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
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
