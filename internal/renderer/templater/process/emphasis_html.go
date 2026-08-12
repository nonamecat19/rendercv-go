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
	// A block prefix a block parser already stripped (a list marker, a
	// blockquote `>`) ends in whitespace, so it is indistinguishable from the
	// start of a block to every guard here.
	source := block.Source()
	lineStart := segment.Start
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}
	// **A pattern's body may cross a soft line break.** Every emphasis regex is
	// compiled with `re.DOTALL` (`inlinepatterns.py:546-552`) and matched
	// against the whole block, so `*a\nb*` in a wrapped paragraph is one
	// emphasis upstream. goldmark's own parser handled that before this one
	// replaced it, which made line-bounded matching a regression rather than a
	// standing limitation.
	//
	// The window extends across following lines whose only gap from the
	// previous one is whitespace goldmark stripped, which is exactly where
	// absolute source offsets stay linear. A gap containing anything else is a
	// stripped block marker; joining across one would need an offset map this
	// parser does not carry, so the window stops there.
	dataStop := blockWindow(block, segment)
	data := string(source[lineStart:dataStop])
	pos := segment.Start - lineStart

	index, end, firstStart, firstEnd, secondStart, secondEnd, ok := matchEmphasis(patterns, data, pos, -1, floor)
	if !ok {
		return nil
	}

	return buildEmphasis(block, pc, lineStart, pos, index, delim, end, firstStart, firstEnd, secondStart, secondEnd)
}

// buildEmphasis builds the node tree for one matched pattern and advances
// block exactly to the end of the match, consuming every delimiter run along
// the way — the opening one, the middle one for a double pattern, and the
// closing one — none of which are literal content.
//
// It assumes block is positioned at `dataStart + pos`; every offset parameter
// is an index into the same data `matchEmphasis` was handed, whose first byte
// sits at absolute source offset `dataStart`.
func buildEmphasis(block text.Reader, pc parser.Context, dataStart, pos, index int, delim byte, end, firstStart, firstEnd, secondStart, secondEnd int) ast.Node {
	block.Advance(firstStart - pos)

	switch index {
	case 0: // EM_STRONG_RE / EM_STRONG2_RE: strong[ em[first] second ]
		strong := ast.NewEmphasis(2)
		em := ast.NewEmphasis(1)
		strong.AppendChild(strong, em)
		parseEmphasisBody(em, block, pc, dataStart+firstEnd, index, delim, true)
		block.Advance(secondStart - firstEnd)
		parseEmphasisBody(strong, block, pc, dataStart+secondEnd, index, delim, true)
		block.Advance(end - secondEnd)
		return strong
	case 1: // STRONG_EM_RE / STRONG_EM2_RE: em[ strong[first] second ]
		em := ast.NewEmphasis(1)
		strong := ast.NewEmphasis(2)
		em.AppendChild(em, strong)
		parseEmphasisBody(strong, block, pc, dataStart+firstEnd, index, delim, true)
		block.Advance(secondStart - firstEnd)
		parseEmphasisBody(em, block, pc, dataStart+secondEnd, index, delim, true)
		block.Advance(end - secondEnd)
		return em
	case 2: // STRONG_EM3_RE / SMART_STRONG_EM_RE: strong[ first em[second] ]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, pc, dataStart+firstEnd, index, delim, true)
		block.Advance(secondStart - firstEnd)
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, pc, dataStart+secondEnd, index, delim, true)
		strong.AppendChild(strong, em)
		block.Advance(end - secondEnd)
		return strong
	case 3: // STRONG_RE / SMART_STRONG_RE: strong[first]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, pc, dataStart+firstEnd, index, delim, true)
		block.Advance(end - firstEnd)
		return strong
	default: // 4, EMPHASIS_RE / SMART_EMPHASIS_RE: em[first]
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, pc, dataStart+firstEnd, index, delim, true)
		block.Advance(end - firstEnd)
		return em
	}
}

// blockWindow is the source offset one past the last line of this block that
// is contiguous with `segment` — the span the emphasis matchers may look at.
//
// It walks the reader forward and puts it back, which is the only way to ask a
// `text.Reader` how far its block reaches; `Position`/`SetPosition` are exact,
// so the walk has no effect on the caller.
func blockWindow(block text.Reader, segment text.Segment) int {
	line, seg := block.Position()
	defer block.SetPosition(line, seg)

	source := block.Source()
	stop := segment.Stop
	for {
		block.AdvanceLine()
		next, nextSeg := block.PeekLine()
		if len(next) == 0 || nextSeg.Start < stop {
			return stop
		}
		// A continuation line goldmark has stripped leading whitespace from
		// starts *after* the previous line's stop. Upstream keeps that
		// whitespace — `*a\n b*` is `<em>a\n b</em>`, space and all — and the
		// gap is still contiguous source, so the window may span it as long as
		// the skipped bytes really are only whitespace. Anything else is a
		// stripped block marker, where offsets stop being linear.
		if !isAllSpace(source[stop:nextSeg.Start]) {
			return stop
		}
		stop = nextSeg.Stop
	}
}

// isAllSpace reports whether every byte is one python-markdown's `\s` matches.
func isAllSpace(b []byte) bool {
	for _, c := range b {
		if !isSpaceByte(c) {
			return false
		}
	}
	return true
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
	for {
		peek, segment := block.PeekLine()
		if len(peek) == 0 || segment.Start >= endAbs {
			return
		}
		// **The body runs to endAbs, not to the end of the line.** `blockWindow`
		// let the *match* cross a soft line break; a code span or a link inside
		// the matched body has to be allowed to cross it too, or `*a `b\nc` d*`
		// finds no code span and falls back to literal text. The window is
		// contiguous in the source by construction, so this slice is exactly the
		// body text upstream matches against.
		line := source[segment.Start:endAbs]
		limit := len(line)

		trigger := -1
		for i, c := range line {
			if c == '`' || c == '[' || c == '*' || c == '_' || c == '\\' || c == '!' || c == '<' {
				trigger = i
				break
			}
		}
		if trigger < 0 {
			ast.MergeOrAppendTextSegment(container, segment.WithStop(segment.Start+limit))
			block.Advance(limit)
			continue
		}
		if trigger > 0 {
			ast.MergeOrAppendTextSegment(container, segment.WithStop(segment.Start+trigger))
			block.Advance(trigger)
			continue
		}

		switch line[0] {
		case '`':
			if node, ok := buildBodyCodeSpan(block, line, segment.Start); ok {
				container.AppendChild(container, node)
				continue
			}
		case '[':
			if !allowLink {
				break
			}
			if node, ok := buildBodyLink(block, pc, line, segment.Start, endAbs); ok {
				container.AppendChild(container, node)
				continue
			}
		case '!':
			if node, ok := buildImage(block, line, segment.Start); ok {
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
				escaped := segment.WithStart(segment.Start + 1)
				ast.MergeOrAppendTextSegment(container, escaped.WithStop(segment.Start+2))
				block.Advance(2)
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
			data := string(line)
			if index, end, fs, fe, ss, se, ok := matchEmphasis(nestedPatterns, data, 0, nestedCutoff, nestedFloor); ok {
				node := buildEmphasis(block, pc, segment.Start, 0, index, line[0], end, fs, fe, ss, se)
				container.AppendChild(container, node)
				continue
			}
		}

		// The trigger byte matched nothing: literal.
		ast.MergeOrAppendTextSegment(container, segment.WithStop(segment.Start+1))
		block.Advance(1)
	}
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
func buildBodyCodeSpan(block text.Reader, line []byte, start int) (ast.Node, bool) {
	segment := text.NewSegment(start, start+len(line))
	width := 0
	for width < len(line) && line[width] == '`' {
		width++
	}
	_, after := matchBackticks(string(line), width, width)
	if after < 0 {
		return nil, false
	}
	span := ast.NewCodeSpan()
	contentSegment := segment.WithStop(segment.Start + after - width)
	contentSegment = contentSegment.WithStart(segment.Start + width)
	span.AppendChild(span, ast.NewTextSegment(contentSegment))
	block.Advance(after)
	return span, true
}

// buildBodyLink is `[label](destination)` inside an emphasis body, sharing
// `linkParser`'s label handling (`link.go`): the label is parsed as inline
// Markdown through `parseEmphasisBody`, not stashed as raw text, so
// `*a [b **c** d](u) e*` nests a strong inside the link exactly as upstream's
// own recursive `self.parser.parseChunk` does. It declines anything
// reference-style, unbalanced, or reaching past endAbs, the same narrowness
// `imageParser` already documents.
func buildBodyLink(block text.Reader, pc parser.Context, line []byte, start, endAbs int) (ast.Node, bool) {
	segment := text.NewSegment(start, start+len(line))
	if len(line) < 2 || line[0] != '[' {
		return nil, false
	}
	scan := []byte(maskAbove(string(line), prioLink))
	_, after := matchBracketed(scan, 1, '[', ']')
	if after < 0 || after >= len(scan) || scan[after] != '(' {
		return nil, false
	}
	href, title, hasTitle, parenEnd, ok := getLink(scan, line, after)
	if !ok || segment.Start+parenEnd > endAbs {
		return nil, false
	}

	link := ast.NewLink()
	link.Destination = href
	if hasTitle {
		link.Title = title
	}

	block.Advance(1) // the opening `[`
	parseEmphasisBody(link, block, pc, segment.Start+after-1, -1, noCutoffDelim, false)
	block.Advance(parenEnd - (after - 1)) // the closing `]` through the `)`

	return link, true
}
