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

// pythonSetextHeadingParser is goldmark's setext heading parser with
// `SetextHeaderProcessor`'s strip (`blockprocessors.py:510`).
//
// `h.text = lines[0].strip()` is the same rule as the ATX processor's at :479 —
// a full strip over Python's whitespace — reached through a different seam.
// goldmark builds the heading's text from the paragraph above the bar and only
// hands it over in `Close` (`parser/setext_headings.go:79-104`,
// `heading.SetLines(tmp.Lines())`), so `Open` has nothing to trim yet.
//
// The leading end is already right when this runs, because the lines come from
// a paragraph and `pythonParagraphParser` has `lstrip`ped them (`paragraph.go`).
// The trailing end is what was left: `"h\v\n==="` was `<h1>h\v</h1>` against
// `<h1>h</h1>`. Both ends are trimmed here anyway — `lines[0].strip()` is one
// call and splitting it across two files by which half currently shows would be
// a rule nobody could check.
type pythonSetextHeadingParser struct {
	parser.BlockParser
}

// Close delegates and strips what the paragraph handed over.
func (p pythonSetextHeadingParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	p.BlockParser.Close(node, reader, pc)
	if node.Parent() == nil || node.Kind() != ast.KindHeading {
		// goldmark turns a heading with no text above the bar back into a
		// paragraph and removes the node (`setext_headings.go:87-95`).
		return
	}
	stripHeadingText(node, reader.Source())
}

// stripHeadingText is `str.strip()` over a heading's own lines: the leading run
// off the first and the trailing run off the last, which for the one line a
// heading has is the single strip upstream performs.
func stripHeadingText(node ast.Node, source []byte) {
	lines := node.Lines()
	if lines.Len() == 0 {
		return
	}

	first := lines.At(0)
	value := first.Value(source)
	if left := pythonSpacePrefix(value); left == len(value) && lines.Len() == 1 {
		lines.Clear()
		return
	} else if left > 0 {
		lines.Set(0, first.WithStart(first.Start+left))
	}

	last := lines.At(lines.Len() - 1)
	if right := pythonSpaceSuffix(last.Value(source)); right > 0 {
		lines.Set(lines.Len()-1, last.WithStop(last.Stop-right))
	}
}
