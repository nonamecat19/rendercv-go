package process

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// widePaddedListMarker is a list marker followed by more than a tab length of
// spaces — five or more — which is the only shape where python-markdown and
// CommonMark disagree about where an item's content begins: `calcListOffset`
// (`parser/list.go:83-94`) keeps a run of at most 4 as the item's offset and
// otherwise falls back to 1, leaving the remainder to the indented-code rule.
// The leading `[ ]{0,3}` and the marker alphabet are goldmark's own
// (`parser/list.go:26-81`, `parseListItem`); the line it is matched against has
// already had any enclosing block's indentation removed by the reader, so a
// nested item is seen at column 0..3 here too.
var widePaddedListMarker = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])( {5,})[^ \n]`)

// pythonListItemParser is goldmark's list item parser with python-markdown's
// rule for the whitespace between a marker and its content.
//
// `UListProcessor.CHILD_RE` is `^[ ]{0,3}((\d+\.)|[*+-])[ ]+(.*)`
// (`markdown/blockprocessors.py:350-351`, shared with `OListProcessor`), and
// `get_items` keeps only `group(3)` (:431). The `[ ]+` is greedy, so **every**
// space between the marker and the content is dropped, however many there are,
// and an item's first line can never carry indentation into the block parser.
// python-markdown's indented-code test is `block.startswith(' ' * 4)`
// (`CodeBlockProcessor.test`, :255) run on that already-stripped text, so it
// cannot fire on marker padding.
//
// CommonMark, and so goldmark, instead reads a run of more than four spaces as
// one space of item offset plus an indented code block. `-     a` is
// `<li>a</li>` upstream and `<li><pre><code>a</code></pre></li>` here, and a
// highlight lines its text up under a bullet often enough to reach it.
//
// The fix is deliberately not a rewrite of the input the way
// `flattenShallowLists` is: the same line means different things by context —
// `a\n\n    -     x` is an indented **code block** in both libraries, marker and
// all — and only the parser knows which one it is looking at. So the default
// parser decides what a list item is, and this one only moves the reader past
// the padding it left behind.
type pythonListItemParser struct {
	parser.BlockParser
}

// Open delegates to goldmark and then consumes the padding CommonMark would
// have left for the indented-code rule.
func (p pythonListItemParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	extra := widePadding(line)

	node, state := p.BlockParser.Open(parent, reader, pc)
	if node == nil || state&parser.HasChildren == 0 || extra == 0 {
		return node, state
	}
	// goldmark has advanced one column past the marker; the item's `Offset` is
	// already marker + 1, which is what a single-space item would have had, so
	// the continuation lines keep the offsets python-markdown gives them.
	reader.Advance(extra)
	return node, state
}

// widePadding is the number of spaces goldmark leaves in front of a list item's
// content, which is zero unless the marker is followed by more than a tab
// length of them. Tabs cannot appear here: `normalizeWhitespace` expanded them
// before the parser ever saw the line.
func widePadding(line []byte) int {
	match := widePaddedListMarker.FindSubmatchIndex(line)
	if match == nil {
		return 0
	}
	run := match[3] - match[2]
	return run - 1
}
