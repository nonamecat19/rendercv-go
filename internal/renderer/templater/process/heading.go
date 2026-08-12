package process

import (
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
//
// It also carries the opening rules the same expression states and goldmark
// does not share — see `Open`.
type pythonATXHeadingParser struct {
	parser.BlockParser
}

// Open declines an indented hash run, then delegates and strips what goldmark
// left.
//
// **An indent is not allowed at all** (spec 011 delta A §3.4). The hash run has
// to sit immediately after `^` or a `\n`:
//
//	# markdown/blockprocessors.py:461
//	RE = re.compile(r'(?:^|\n)(?P<level>#{1,6})(?P<header>(?:\\.|[^\\])*?)#*(?:\n|$)')
//
// There is no `[ ]{0,3}` in it, where CommonMark 0.31 §4.2 permits three spaces
// and goldmark consumes them before this parser is ever asked
// (`parser/parser.go:962-968`, `pc.SetBlockOffset`). So ` # h` is `<p># h</p>`
// upstream and was `<h1>h</h1>` here — the port produced a heading nobody wrote.
//
// The indent is the one **inside the block**: a list marker's padding and a
// blockquote's one space are gone before upstream's processor sees the line, so
// `- # h` and `> # h` are headings in both libraries and only the space the
// container did not claim counts. That is `BlockIndent`, and it is most of the
// rule — `containerSwallowedIndent` is the rest, for the one place the two
// libraries disagree about who the space belongs to.
//
// **No space is required after the hashes** either (§3.1): nothing in the
// expression separates `#{1,6}` from `header`, where CommonMark 0.31 §4.2 asks
// for a space, a tab or the end of the line and goldmark declines without one
// (`parser/atx_heading.go:97-100`). `openTightHeading` is that case, and it is
// the one an ordinary CV reaches — `#1 in sales` is `<h1>1 in sales</h1>`
// upstream and was a paragraph here.
func (p pythonATXHeadingParser) Open(
	parent ast.Node, reader text.Reader, pc parser.Context,
) (ast.Node, parser.State) {
	if pc.BlockIndent() > 0 || containerSwallowedIndent(reader) {
		return nil, parser.NoChildren
	}

	node, state := openTightHeading(reader, pc)
	if node == nil {
		node, state = p.BlockParser.Open(parent, reader, pc)
	}
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

// openTightHeading opens the heading goldmark turns down for want of a space
// after the hashes, and returns nil for every other shape so goldmark's own
// parser keeps the ones the two libraries already agree on.
//
// It is goldmark's `Open` with the space requirement removed
// (`parser/atx_heading.go:82-137`) rather than upstream's `RE.search` over the
// whole block: with a space present every `search`-reachable shape already
// agrees, including a heading that interrupts a paragraph or ends a list, so
// nothing here needs the `before`/`after` splitting of
// `blockprocessors.py:470-487` (spec-delta-atx §3.5).
func openTightHeading(reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 {
		return nil, parser.NoChildren
	}
	end := pos
	for ; end < len(line) && line[end] == '#'; end++ {
	}
	level := end - pos
	if level == 0 || level > 6 {
		return nil, parser.NoChildren
	}
	if end == len(line) || util.IsSpace(line[end]) {
		// A space, a tab or the end of the line: goldmark and upstream open the
		// same heading, so let goldmark open it.
		return nil, parser.NoChildren
	}

	body := text.NewSegment(
		segment.Start+end-segment.Padding,
		segment.Start+len(line)-segment.Padding)
	body = body.TrimRightSpace(reader.Source())
	node := ast.NewHeading(level)
	if body.Len() > 0 {
		body.Stop = body.Start + cutClosingRun(body.Value(reader.Source()))
		node.Lines().Append(body)
	}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

// cutClosingRun is where a heading's text ends: goldmark's closing-sequence
// rule (`parser/atx_heading.go:120-136`), which drops a trailing run of hashes
// only when a space precedes it.
func cutClosingRun(line []byte) int {
	stop := len(line)
	if stop == 0 {
		return 0
	}
	i := stop - 1
	for ; line[i] == '#' && i > 0; i-- {
	}
	if i == 0 && line[0] == '#' {
		return 0
	}
	if i != stop-1 && util.IsSpace(line[i]) {
		return i - util.TrimRightSpaceLength(line[0:i])
	}
	return stop
}

// containerSwallowedIndent reports whether the line's own indentation was taken
// by a list item before `BlockIndent` could count it.
//
// A list item's continuation line is where the two libraries disagree about who
// the spaces belong to. `- item\n  # h` is `<li>item\n  # h</li>` upstream: the
// item's block is `item\n  # h`, spaces and all, because
// `ListIndentProcessor` only claims a following block at a full `tab_length` of
// four (`markdown/blockprocessors.py`, and `flattenShallowLists` in `html.go`
// for the other half of that rule), and the hash run inside that block is
// therefore indented and not a heading. goldmark's item content starts at
// column 2, so it reads the same two spaces as its own padding and offers the
// parser a line beginning at `#`.
//
// The test is the raw line, and the bound is a tab length: at four the block
// really is the item's child upstream, dedented by four, and `# h` there is a
// heading in both libraries (`- item\n\n    # h`). Below four it never is. A
// line whose prefix is not all spaces — `- # h`, `-   # h`, `> # h` — is the
// marker line itself, whose padding upstream removes too, and is not this case.
func containerSwallowedIndent(reader text.Reader) bool {
	_, segment := reader.PeekLine()
	source := reader.Source()
	start := segment.Start
	if start > len(source) {
		return false
	}
	indent := 0
	for start-indent > 0 {
		c := source[start-indent-1]
		if c == '\n' {
			return indent > 0 && indent < pythonMarkdownTabLength
		}
		if c != ' ' {
			return false
		}
		indent++
	}
	return false
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
