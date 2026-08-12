package process

import (
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

// itemPhase is where a list item's parse has got to, which decides how much
// indentation a continuation line loses. See `Continue`.
type itemPhase int

const (
	// itemFirstBlock is the item's own block: the marker's line and every line
	// up to the first blank one. goldmark's offset governs it.
	itemFirstBlock itemPhase = iota
	// itemAfterBlank is a blank line inside the item; the next non-blank line
	// is a new block and `ListIndentProcessor` gets to claim it.
	itemAfterBlank
	// itemDetabbed is a block `ListIndentProcessor` claimed, whose every line
	// loses one tab length rather than the item's offset.
	itemDetabbed
)

// itemPhases is the phase of every list item open in one parse. It is context
// state rather than a field because the parser is a stateless singleton shared
// by every conversion, and nested items are each in a phase of their own.
var itemPhases = parser.NewContextKey()

func phases(pc parser.Context) map[ast.Node]itemPhase {
	m, _ := pc.Get(itemPhases).(map[ast.Node]itemPhase)
	if m == nil {
		m = map[ast.Node]itemPhase{}
		pc.Set(itemPhases, m)
	}
	return m
}

// Continue applies `ListIndentProcessor`'s dedent to the blocks that follow a
// blank line inside an item.
//
// python-markdown splits the document at blank lines and hands each block to a
// processor. A block indented by at least `tab_length` under a list becomes
// that list's child (`ListIndentProcessor.test`, `blockprocessors.py:175-179`)
// and `run` strips `looseDetab(block, level)` from it — `tab_length` spaces per
// **nesting level** (`:184`, `:99-105`), counted from column 0 and completely
// independent of how wide the item's marker was.
//
// CommonMark instead measures a continuation line against the column the item's
// content starts at, which is the marker plus its padding: 2 for `- x`, 3 for
// `1. x`. So `- x\n\n        a` is a code block holding `a` upstream and one
// holding `  a` in goldmark, and every extra character of marker shifts the
// result again.
//
// The dedent is one tab length here, not `tab_length*level`, because the levels
// compose on their own: an item nested inside another is only reached after the
// outer item's `Continue` has already consumed its own tab length, so a block at
// column 8 under two levels of list arrives at column 0.
//
// The phase, rather than the blank line alone, is what makes the whole block
// move together — `looseDetab` runs over every line of the block, so the second
// line of an indented code block loses a tab length just as its first did.
func (p pythonListItemParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	phase := phases(pc)
	if util.IsBlank(line) {
		phase[node] = itemAfterBlank
		return p.BlockParser.Continue(node, reader, pc)
	}
	if phase[node] == itemFirstBlock {
		return p.BlockParser.Continue(node, reader, pc)
	}
	indent, _ := util.IndentWidth(line, reader.LineOffset())
	if indent < pythonMarkdownTabLength {
		// `ListIndentProcessor.test` declines the block, so it is not the
		// item's child at all and goldmark's own rule decides what it is.
		phase[node] = itemFirstBlock
		return p.BlockParser.Continue(node, reader, pc)
	}
	phase[node] = itemDetabbed
	pos, padding := util.IndentPosition(line, reader.LineOffset(), pythonMarkdownTabLength)
	reader.AdvanceAndSetPadding(pos, padding)
	return parser.Continue | parser.HasChildren
}

// Close forgets the item's phase, so the map cannot grow past the items open at
// any one moment.
func (p pythonListItemParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	delete(phases(pc), node)
	p.BlockParser.Close(node, reader, pc)
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
