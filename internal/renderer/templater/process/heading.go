package process

import (
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// pythonATXHeadingParser is goldmark's ATX heading parser with
// `HashHeaderProcessor`'s strip (`blockprocessors.py:479`).
//
// `h.text = m.group('header').strip()` — a full `str.strip()`, both ends, over
// Python's whitespace, where goldmark trims only its own four space characters
// (`parser/atx_heading.go:94-137`). So `"# \vh"` was `<h1>\vh</h1>` here against
// `<h1>h</h1>` upstream, and `"# h\v"` kept the `\v` on the other side.
//
// **It is a strip, not the `lstrip` the paragraph gets** (`paragraph.go`). The
// two processors read the same way in upstream's source and do not do the same
// thing: a paragraph keeps its trailing whitespace and a heading does not.
//
// The trim is applied to the parser's own output rather than to the line before
// it, because goldmark decides where the heading text begins and ends — the
// leading `#`s, the optional closing run of them, and the level — and only what
// is left after that decision is `m.group('header')`.
type pythonATXHeadingParser struct {
	parser.BlockParser
}

// Open delegates and strips what goldmark left.
func (p pythonATXHeadingParser) Open(
	parent ast.Node, reader text.Reader, pc parser.Context,
) (ast.Node, parser.State) {
	node, state := p.BlockParser.Open(parent, reader, pc)
	if node == nil || node.Kind() != ast.KindHeading || node.Lines().Len() == 0 {
		return node, state
	}

	line := node.Lines().At(0)
	value := line.Value(reader.Source())
	left := pythonSpacePrefix(value)
	if left == len(value) {
		// `"# \v"` is `<h1></h1>` upstream: the group strips to nothing. The
		// lines are cleared rather than left zero-length, which is how
		// goldmark's own empty-heading branches leave the node.
		node.Lines().Clear()
		return node, state
	}
	right := pythonSpaceSuffix(value[left:])

	trimmed := line.WithStart(line.Start + left)
	node.Lines().Set(0, trimmed.WithStop(trimmed.Stop-right))
	return node, state
}

// pythonSpaceSuffix is the byte length of the trailing whitespace
// `str.rstrip()` would remove, counted in runes for `pythonSpacePrefix`'s
// reason.
func pythonSpaceSuffix(line []byte) int {
	width := 0
	for width < len(line) {
		r, size := utf8.DecodeLastRune(line[:len(line)-width])
		if !isPythonSpace(r) {
			break
		}
		width += size
	}
	return width
}
