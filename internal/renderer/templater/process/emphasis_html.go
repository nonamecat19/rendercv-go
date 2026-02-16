package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// emphasisParser is the HTML backend for the five-pattern machinery
// emphasis.go already carries for the Typst path (spec 011 §7, plan §3.3).
//
// It replaces goldmark's own `parser.NewEmphasisParser` entirely rather than
// merely outranking it (see `pythonInlineParsers` in html.go): python-markdown
// and CommonMark disagree on what counts as emphasis at all, not only on how it
// nests, so a position python's five patterns decline is not necessarily one
// CommonMark's delimiter stack would decline too, and letting the two
// algorithms both have a turn would reintroduce the divergence this file
// exists to close.
//
// It owns the whole matched span, open delimiters to close, in one `Parse`
// call — unlike goldmark's own emphasis, which is delimiter-stack-based and
// resolves nesting later. python-markdown's algorithm decides nesting by
// *pattern index*, not by proximity, which does not fit that stack; §7.1's
// behavior 20-23 are the reason. `parseEmphasisBody` is this file's version of
// `inline.go`'s `matchPrefix` loop, walking a matched body for the inline
// elements that still need to run inside it — code spans, links, images, raw
// HTML, autolinks and escapes, `plan.md` §3.3's third bullet — while consuming
// the reader exactly, so it composes with whatever consumed it. `linkParser`
// (`link.go`) reuses the same dispatcher for a link's label.
type emphasisParser struct{}

// Trigger is both delimiter characters python-markdown's two processors claim.
func (emphasisParser) Trigger() []byte { return []byte{'*', '_'} }

// Parse claims one full emphasis span — open delimiters to close — at the
// trigger position, or declines by returning nil.
func (emphasisParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) == 0 {
		return nil
	}
	delim := line[0]

	patterns, floor := asteriskPatterns, prioEmStrong
	if delim == '_' {
		patterns, floor = underscorePatterns, prioEmStrong2
	}

	// **The data has to begin at the line, not at the trigger.** Upstream
	// matches each pattern against the whole block with `pattern.match(data,
	// m.start(0))` (`inlinepatterns.py:663-668`), so every lookbehind — the
	// `(?<!\w)` that all three smart underscore patterns open with, and
	// `NOT_STRONG_RE`'s `(?<=\s)` — sees what precedes the delimiter. Handing
	// the matchers a string that starts at the delimiter made every one of
	// those guards vacuous, which is how `snake_case_` became
	// `snake<em>case</em>`: the trailing `_` opened an emphasis whose `(?<!\w)`
	// had no `e` to look at.
	//
	// **The line starts where the reader says it does, not at the previous
	// newline in the source.** A block parser has already stripped the item's
	// marker or the blockquote's `>`, and those bytes are not part of the text
	// upstream matches against. Scanning back to the raw `\n` handed the
	// matchers a `>` where upstream has the start of the block, which is enough
	// to change `NOT_STRONG_RE`'s `(^|(?<=\s))`: a `>_` line left its `_`
	// unclaimed here and emphasised the rest, where upstream takes the `_` as a
	// stand-alone delimiter.
	lineStart := lineHead(block)
	// **A pattern's body may cross a soft line break.** Every emphasis regex is
	// compiled with `re.DOTALL` (`inlinepatterns.py:546-552`) and matched
	// against the whole block, so `*a\nb*` in a wrapped paragraph is one
	// emphasis upstream. goldmark's own parser handled that before this one
	// replaced it, which made line-bounded matching a regression rather than a
	// standing limitation.
	//
	// The window extends across following lines whose gap from the previous one
	// is only whitespace goldmark stripped; see buildWindow.
	// The window is the **reader's** text, not a slice of the source: each line
	// arrives with its block marker already stripped, and `offs` maps every
	// byte back to where it came from. A blockquote's `> ` is the case that
	// forces this — it sits between two lines in the source but not in the text
	// upstream matches, so a source slice would feed the matchers two bytes
	// that are not there and a window that refused to span it lost 40 shapes
	// that worked before this parser existed.
	w := buildWindow(block, lineStart, -1)
	pos := w.index(segment.Start)
	if pos < 0 {
		return nil
	}

	index, end, firstStart, firstEnd, secondStart, secondEnd, ok := matchEmphasis(patterns, string(w.text), pos, -1, floor)
	if !ok {
		return nil
	}

	return buildEmphasis(block, pc, index, delim,
		w.source(end), w.source(firstStart), w.source(firstEnd),
		w.source(secondStart), w.source(secondEnd))
}

// buildEmphasis builds the node tree for one matched pattern and advances
// block exactly to the end of the match, consuming every delimiter run along
// the way — the opening one, the middle one for a double pattern, and the
// closing one — none of which are literal content.
//
// It assumes block is positioned at `dataStart + pos`; every offset parameter
// is an index into the same data `matchEmphasis` was handed, whose first byte
// sits at absolute source offset `dataStart`.
func buildEmphasis(block text.Reader, pc parser.Context, index int, delim byte, end, firstStart, firstEnd, secondStart, secondEnd int) ast.Node {
	advanceTo(block, firstStart)

	switch index {
	case 0: // EM_STRONG_RE / EM_STRONG2_RE: strong[ em[first] second ]
		strong := ast.NewEmphasis(2)
		em := ast.NewEmphasis(1)
		strong.AppendChild(strong, em)
		parseEmphasisBody(em, block, pc, firstEnd, index, delim, true)
		advanceTo(block, secondStart)
		parseEmphasisBody(strong, block, pc, secondEnd, index, delim, true)
		advanceTo(block, end)
		return strong
	case 1: // STRONG_EM_RE / STRONG_EM2_RE: em[ strong[first] second ]
		em := ast.NewEmphasis(1)
		strong := ast.NewEmphasis(2)
		em.AppendChild(em, strong)
		parseEmphasisBody(strong, block, pc, firstEnd, index, delim, true)
		advanceTo(block, secondStart)
		parseEmphasisBody(em, block, pc, secondEnd, index, delim, true)
		advanceTo(block, end)
		return em
	case 2: // STRONG_EM3_RE / SMART_STRONG_EM_RE: strong[ first em[second] ]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, pc, firstEnd, index, delim, true)
		advanceTo(block, secondStart)
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, pc, secondEnd, index, delim, true)
		strong.AppendChild(strong, em)
		advanceTo(block, end)
		return strong
	case 3: // STRONG_RE / SMART_STRONG_RE: strong[first]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, pc, firstEnd, index, delim, true)
		advanceTo(block, end)
		return strong
	default: // 4, EMPHASIS_RE / SMART_EMPHASIS_RE: em[first]
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, pc, firstEnd, index, delim, true)
		advanceTo(block, end)
		return em
	}
}

// window is the block text the emphasis matchers see — the reader's own lines,
// markers stripped, joined exactly as upstream joins them — together with the
// source offset of every byte. `text` is what the patterns match; `offs` is how
// a match's boundaries become positions the reader can be seeked to.
type window struct {
	text []byte
	offs []int
}

// index is the window position of a source offset, or -1.
func (w window) index(src int) int {
	for i, off := range w.offs {
		if off == src {
			return i
		}
	}
	return -1
}

// source is the source offset of a window position. One past the end maps to
// one past the last byte's source offset, which is what an exclusive bound
// needs; a negative position stays negative, marking "no second group".
func (w window) source(i int) int {
	switch {
	case i < 0:
		return i
	case i < len(w.offs):
		return w.offs[i]
	case len(w.offs) == 0:
		return 0
	default:
		return w.offs[len(w.offs)-1] + 1
	}
}

// buildWindow joins the block's lines over the source range [from, to), where
// a `to` of -1 means the rest of the block. It walks the reader and restores
// it, restoring the reader's position exactly.
func buildWindow(block text.Reader, from, to int) window {
	line, seg := block.Position()
	defer block.SetPosition(line, seg)

	block.SetPosition(line, text.NewSegment(-1, -1))

	source := block.Source()
	var w window
	prevStop := -1
	for {
		next, nextSeg := block.PeekLine()
		if len(next) == 0 {
			return w
		}
		// **The gap is not all marker.** goldmark strips a blockquote's `>` and
		// one space, then the paragraph trims whatever indent follows, so those
		// trimmed bytes sit between two segments and reach neither. Upstream
		// keeps them — `> *a\n>   b*` is `<em>a\n  b</em>`, two spaces and all —
		// so the window takes back the part of the gap that is only whitespace
		// once the marker is accounted for. Dropping them deleted bytes from a
		// user's text, which is worse than any missed match.
		if prevStop >= 0 && nextSeg.Start > prevStop {
			for at := markerEnd(source, prevStop, nextSeg.Start); at < nextSeg.Start; at++ {
				if to >= 0 && at >= to {
					return w
				}
				if at >= from {
					w.text = append(w.text, source[at])
					w.offs = append(w.offs, at)
				}
			}
		}
		prevStop = nextSeg.Stop
		for i := 0; i < len(next); i++ {
			at := nextSeg.Start + i
			if at < from {
				continue
			}
			if to >= 0 && at >= to {
				return w
			}
			w.text = append(w.text, next[i])
			w.offs = append(w.offs, at)
		}
		block.AdvanceLine()
	}
}

// appendSpans adds w[from:to) to node as text segments, one per maximal
// contiguous run of source. A range that steps over a stripped block marker
// becomes two segments rather than one that would drag the marker along.
func appendSpans(node ast.Node, w window, from, to int) {
	for i := from; i < to && i < len(w.offs); {
		start := w.offs[i]
		j := i + 1
		for j < to && j < len(w.offs) && w.offs[j] == w.offs[j-1]+1 {
			j++
		}
		node.AppendChild(node, ast.NewTextSegment(text.NewSegment(start, w.offs[j-1]+1)))
		i = j
	}
}

// markerEnd is the offset just past the block markers at the head of a gap —
// each `>` and the single space CommonMark lets follow it. What remains is the
// author's own indentation, which upstream keeps. A gap holding anything else
// is not a marker at all and the whole gap is skipped, which is what `from`
// and the whitespace test at the call site enforce.
func markerEnd(source []byte, start, stop int) int {
	at := start
	for at < stop {
		switch source[at] {
		case '>':
			at++
			if at < stop && source[at] == ' ' {
				at++
			}
		case '\n':
			at++
		default:
			// Not a marker byte: the rest is the author's, unless it is not
			// whitespace at all, in which case nothing here belongs to them.
			for i := at; i < stop; i++ {
				if !isSpaceByte(source[i]) {
					return stop
				}
			}
			return at
		}
	}
	return at
}

// lineHead is the source offset the reader's current line begins at, once its
// block marker has been stripped. `text.Reader` exposes it only by seeking:
// `SetPosition` with an invalid segment resets to the line's own segment
// (`text/reader.go:497-506`), so this asks and puts the position straight back.
func lineHead(block text.Reader) int {
	line, pos := block.Position()
	block.SetPosition(line, text.NewSegment(-1, -1))
	_, lineSeg := block.PeekLine()
	block.SetPosition(line, pos)
	return lineSeg.Start
}

// advanceTo moves the reader to an absolute **source** offset.
//
// `text.Reader.Advance` counts in the reader's own coordinates, which skip the
// indent goldmark strips from a continuation line, so advancing by a delta
// measured in the source runs past the target by exactly the stripped bytes —
// that is how `x**a\n b**y` lost its `y`. Seeking instead of stepping removes
// the mismatch: each hop re-reads the current segment, so a gap costs nothing
// and an offset that lands *inside* one settles at the next real byte, which
// is the closest position the reader can represent.
func advanceTo(block text.Reader, abs int) {
	for {
		line, segment := block.PeekLine()
		if len(line) == 0 {
			return
		}
		if abs <= segment.Stop {
			if abs > segment.Start {
				block.Advance(abs - segment.Start)
			}
			return
		}
		block.AdvanceLine()
	}
}

// noCutoffDelim is what a fresh inline context — a link's label, which is not
// nested inside any emphasis pattern's body — passes as `delim` to
// `parseEmphasisBody`. It never equals a real trigger byte, so the cutoff
// check below always takes its `nestedCutoff = -1` branch: every pattern is
// available, matching `inline.go`'s `parseFrom(text, -1, 0)` at a link's text.
const noCutoffDelim = 0

// parseEmphasisBody walks block from its current position up to the absolute
// offset endAbs, appending children to container: literal text, a code span,
// a link, an image, an autolink or raw HTML, an escape, or a nested emphasis
// matched at the same cutoff `inline.go`'s `parseFrom` uses (spec 011 behavior
// 22). It leaves block positioned at exactly endAbs.
//
// The trigger set and the parsers behind it are exactly `matchPrefix`'s list
// (`inline.go`), reimplemented against goldmark's `[]byte` reader instead of
// the Typst path's `string`, plus the emphasis recursion `matchPrefix` does
// not need. Escapes are handled here because goldmark's own escape handling
// lives in its core dispatch loop (`parser.go`'s `escaped` flag), which this
// dispatcher bypasses entirely.
//
// `allowLink` is false only inside a link's own label, where upstream's
// reprocessing starts one pattern below `link` and so cannot reach it again
// (`treeprocessors.py:315`, and `linkParser.Parse`'s comment).
func parseEmphasisBody(container ast.Node, block text.Reader, pc parser.Context, endAbs, cutoff int, delim byte, allowLink bool) {
	source := block.Source()
	startAbs := -1
	for {
		peek, segment := block.PeekLine()
		if len(peek) == 0 || segment.Start >= endAbs {
			return
		}
		// **The body runs to endAbs, not to the end of the line.** `buildWindow`
		// let the *match* cross a soft line break; a code span or a link inside
		// the matched body has to be allowed to cross it too, or `*a `b\nc` d*`
		// finds no code span and falls back to literal text. The window is
		// contiguous in the source by construction, so this slice is exactly the
		// body text upstream matches against.
		// The body is read through the window too, so a blockquote's `> ` never
		// reaches the output while the indentation after it does.
		bw := buildWindow(block, segment.Start, endAbs)
		line := bw.text
		limit := len(line)
		if startAbs < 0 {
			startAbs = segment.Start
		}

		trigger := -1
		for i, c := range line {
			// A newline is a trigger only when it is `LINE_BREAK_RE`'s `'  \n'`.
			// A soft break is ordinary text and must stay inside the run below,
			// because the run is what carries a continuation line's stripped
			// indent — splitting at every newline dropped it. The two spaces are
			// read from the source rather than from `line`, since an earlier
			// trigger may already have split the run right before them.
			if c == '\n' {
				abs := segment.Start + i
				if abs < 2 || source[abs-1] != ' ' || source[abs-2] != ' ' {
					continue
				}
			}
			if c == '`' || c == '[' || c == '*' || c == '_' || c == '\\' || c == '!' || c == '<' || c == '\n' {
				trigger = i
				break
			}
		}
		if trigger < 0 {
			appendSpans(container, bw, 0, limit)
			advanceTo(block, bw.source(limit))
			continue
		}
		if trigger > 0 {
			appendSpans(container, bw, 0, trigger)
			advanceTo(block, bw.source(trigger))
			continue
		}

		switch line[0] {
		case '\n':
			// `LINE_BREAK_RE` is `'  \n'` at priority 100
			// (`inlinepatterns.py:97`), above `em_strong` at 60, so a hard break
			// resolves *inside* an emphasis body. This dispatcher emitted the
			// body as raw text and seeked past the newline, so goldmark's own
			// linebreak parser never ran and `*a  \nb*` kept two literal spaces
			// where upstream has a `<br />`.
			//
			// A single space is not a hard break: `*a \nb*` keeps its space and
			// its newline, which the literal path below already gets right.
			if trailing := trailingSpaces(container, source); trailing >= 2 {
				hardBreak(container, trailing)
				advanceTo(block, segment.Start+1)
				continue
			}
		case '`':
			if node, ok := buildBodyCodeSpan(block, bw); ok {
				container.AppendChild(container, node)
				continue
			}
		case '[':
			if !allowLink {
				break
			}
			if node, ok := buildBodyLink(block, pc, bw, endAbs); ok {
				container.AppendChild(container, node)
				continue
			}
		case '!':
			if node, ok := buildImage(block, bw); ok {
				container.AppendChild(container, node)
				continue
			}
		case '<':
			// Priority order: automail (250), autolink (300), raw HTML (400) —
			// the same order `pythonInlineParsers` registers them at the top
			// level (`html.go`), since all three share the `<` trigger.
			if node := (automailParser{}).Parse(container, block, pc); node != nil {
				container.AppendChild(container, node)
				continue
			}
			if node := bodyAutoLinkParser.Parse(container, block, pc); node != nil {
				container.AppendChild(container, node)
				continue
			}
			if node := bodyRawHTMLParser.Parse(container, block, pc); node != nil {
				container.AppendChild(container, node)
				continue
			}
		case '\\':
			// ESCAPE_RE `\(.)`, but `EscapeInlineProcessor.handleMatch` declines
			// unless the escaped byte is in `ESCAPED_CHARS`
			// (`inlinepatterns.py:350-360`) — so `**\alpha**` keeps its
			// backslash and only a real escape loses one.
			if len(line) > 1 && isEscapedChar(line[1]) {
				// **Both bytes, backslash included.** `pythonWriter.write`
				// already resolves `\X` in the text it emits
				// (`htmlescape.go:138`), so dropping the backslash here
				// unescapes twice — the same mistake the link destination made.
				// Worse, the bare escaped byte merges with whatever literal
				// follows it, and `\\` + `!` became the text `\!`, which the
				// writer then read as an escape and collapsed to `!`. Handing
				// the writer the untouched pair leaves exactly one round trip.
				appendSpans(container, bw, 0, 2)
				advanceTo(block, bw.source(2))
				continue
			}
		case '*', '_':
			nestedPatterns, nestedFloor := asteriskPatterns, prioEmStrong
			if line[0] == '_' {
				nestedPatterns, nestedFloor = underscorePatterns, prioEmStrong2
			}
			nestedCutoff := -1
			// Only the processor that produced the parent is cut off — the
			// same rule inline.go's fromDelim carries.
			if line[0] == delim {
				nestedCutoff = cutoff
			}
			// **The body is the lookbehind context**, exactly as at the top
			// level: `parse_sub_patterns` matches with `pattern.match(data,
			// pos)` where `data` is the whole group text
			// (`inlinepatterns.py:613-620`), so a `_` one byte into a body has
			// the byte before it to look at. Matching from the trigger made
			// `(?<!\w)` vacuous again, and `*__(_*` came out
			// `<em>_<em>(</em></em>` where upstream leaves `__(_` literal.
			nw := bw
			nestedPos := nw.index(segment.Start)
			if nestedPos < 0 {
				break
			}
			if index, end, fs, fe, ss, se, ok := matchEmphasis(nestedPatterns, string(nw.text), nestedPos, nestedCutoff, nestedFloor); ok {
				node := buildEmphasis(block, pc, index, line[0],
					nw.source(end), nw.source(fs), nw.source(fe), nw.source(ss), nw.source(se))
				container.AppendChild(container, node)
				continue
			}
		}

		// The trigger byte matched nothing: literal.
		appendSpans(container, bw, 0, 1)
		advanceTo(block, bw.source(1))
	}
}

// trailingSpaces counts the spaces at the end of the text already appended to
// container, which is what decides whether the newline about to be consumed is
// `LINE_BREAK_RE`'s hard break or an ordinary soft one.
func trailingSpaces(container ast.Node, source []byte) int {
	last, ok := container.LastChild().(*ast.Text)
	if !ok {
		return 0
	}
	n := 0
	for last.Segment.Stop-1-n >= last.Segment.Start && source[last.Segment.Stop-1-n] == ' ' {
		n++
	}
	return n
}

// hardBreak drops the spaces `LINE_BREAK_RE` consumed and marks the text node,
// which is how goldmark spells `<br />` — the newline itself is written by the
// renderer, so the caller seeks past it rather than emitting it.
func hardBreak(container ast.Node, trailing int) {
	last := container.LastChild().(*ast.Text)
	last.Segment = last.Segment.WithStop(last.Segment.Stop - trailing)
	last.SetHardLineBreak(true)
}

// bodyAutoLinkParser and bodyRawHTMLParser are goldmark's own stock parsers,
// reused as-is inside an emphasis body or a link label: both are stateless
// single-shot matchers (unlike the link-label bracket state machine goldmark's
// own `NewLinkParser` carries), so there is nothing to reimplement — and
// `pc` is threaded through to them for real, not `nil`, since `rawHTMLParser`
// uses it for a multi-line comment/processing-instruction's continuation state.
var (
	bodyAutoLinkParser = parser.NewAutoLinkParser()
	bodyRawHTMLParser  = parser.NewRawHTMLParser()
)

// buildBodyCodeSpan is `BacktickInlineProcessor` (spec 008), reused unchanged:
// content between a run of N backticks and the next run of exactly N, stripped
// at both ends by the renderer (`codespan.go`'s `codeSpanRenderer`), which is
// why this hands back the raw, unstripped segment.
func buildBodyCodeSpan(block text.Reader, w window) (ast.Node, bool) {
	line := w.text
	width := 0
	for width < len(line) && line[width] == '`' {
		width++
	}
	_, after := matchBackticks(string(line), width, width)
	if after < 0 {
		return nil, false
	}
	span := ast.NewCodeSpan()
	appendSpans(span, w, width, after-width)
	advanceTo(block, w.source(after))
	return span, true
}

// buildBodyLink is `[label](destination)` inside an emphasis body, sharing
// `linkParser`'s label handling (`link.go`): the label is parsed as inline
// Markdown through `parseEmphasisBody`, not stashed as raw text, so
// `*a [b **c** d](u) e*` nests a strong inside the link exactly as upstream's
// own recursive `self.parser.parseChunk` does. It declines anything
// reference-style, unbalanced, or reaching past endAbs, the same narrowness
// `imageParser` already documents.
func buildBodyLink(block text.Reader, pc parser.Context, w window, endAbs int) (ast.Node, bool) {
	line := w.text
	if len(line) < 2 || line[0] != '[' {
		return nil, false
	}
	scan := []byte(maskAbove(string(line), prioLink))
	_, after := matchBracketed(scan, 1, '[', ']')
	if after < 0 || after >= len(scan) || scan[after] != '(' {
		return nil, false
	}
	href, title, hasTitle, parenEnd, ok := getLink(scan, line, after)
	if !ok || w.source(parenEnd) > endAbs {
		return nil, false
	}

	link := ast.NewLink()
	link.Destination = href
	if hasTitle {
		link.Title = title
	}

	advanceTo(block, w.source(1)) // the opening `[`
	parseEmphasisBody(link, block, pc, w.source(after-1), -1, noCutoffDelim, false)
	advanceTo(block, w.source(parenEnd)) // the closing `]` through the `)`

	return link, true
}
