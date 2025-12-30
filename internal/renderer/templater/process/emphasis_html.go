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
// elements that still need to run inside it — code spans and links, `plan.md`
// §3.3's third bullet — while consuming the reader exactly, so it composes with
// whatever consumed it.
type emphasisParser struct{}

// Trigger is both delimiter characters python-markdown's two processors claim.
func (emphasisParser) Trigger() []byte { return []byte{'*', '_'} }

// Parse claims one full emphasis span — open delimiters to close — at the
// trigger position, or declines by returning nil.
func (emphasisParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) == 0 {
		return nil
	}
	delim := line[0]

	// NOT_STRONG_RE (`inlinepatterns.py:141`): a lone delimiter with
	// whitespace-or-edge on both sides is never emphasis, matching
	// `inline.go`'s `isolatedDelimiter` but read from the reader's absolute
	// position rather than a local string, since PeekLine only exposes what is
	// at and after the trigger.
	source := block.Source()
	beforeIsSpace := segment.Start == 0 || isSpaceByte(source[segment.Start-1])
	afterIsSpace := len(line) == 1 || isSpaceByte(line[1])
	if beforeIsSpace && afterIsSpace {
		return nil
	}

	patterns := asteriskPatterns
	if delim == '_' {
		patterns = underscorePatterns
	}

	data := string(line)
	index, end, firstStart, firstEnd, secondStart, secondEnd, ok := matchEmphasis(patterns, data, 0, -1)
	if !ok {
		return nil
	}

	return buildEmphasis(block, segment.Start, index, delim, end, firstStart, firstEnd, secondStart, secondEnd)
}

// buildEmphasis builds the node tree for one matched pattern and advances
// block exactly to the end of the match, consuming every delimiter run along
// the way — the opening one, the middle one for a double pattern, and the
// closing one — none of which are literal content.
//
// It assumes block is positioned at the match's start; every offset parameter
// is relative to that start, matching what `matchEmphasis` returns.
func buildEmphasis(block text.Reader, spanStart int, index int, delim byte, end, firstStart, firstEnd, secondStart, secondEnd int) ast.Node {
	block.Advance(firstStart)

	switch index {
	case 0: // EM_STRONG_RE / EM_STRONG2_RE: strong[ em[first] second ]
		strong := ast.NewEmphasis(2)
		em := ast.NewEmphasis(1)
		strong.AppendChild(strong, em)
		parseEmphasisBody(em, block, spanStart+firstEnd, index, delim)
		block.Advance(secondStart - firstEnd)
		parseEmphasisBody(strong, block, spanStart+secondEnd, index, delim)
		block.Advance(end - secondEnd)
		return strong
	case 1: // STRONG_EM_RE / STRONG_EM2_RE: em[ strong[first] second ]
		em := ast.NewEmphasis(1)
		strong := ast.NewEmphasis(2)
		em.AppendChild(em, strong)
		parseEmphasisBody(strong, block, spanStart+firstEnd, index, delim)
		block.Advance(secondStart - firstEnd)
		parseEmphasisBody(em, block, spanStart+secondEnd, index, delim)
		block.Advance(end - secondEnd)
		return em
	case 2: // STRONG_EM3_RE / SMART_STRONG_EM_RE: strong[ first em[second] ]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, spanStart+firstEnd, index, delim)
		block.Advance(secondStart - firstEnd)
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, spanStart+secondEnd, index, delim)
		strong.AppendChild(strong, em)
		block.Advance(end - secondEnd)
		return strong
	case 3: // STRONG_RE / SMART_STRONG_RE: strong[first]
		strong := ast.NewEmphasis(2)
		parseEmphasisBody(strong, block, spanStart+firstEnd, index, delim)
		block.Advance(end - firstEnd)
		return strong
	default: // 4, EMPHASIS_RE / SMART_EMPHASIS_RE: em[first]
		em := ast.NewEmphasis(1)
		parseEmphasisBody(em, block, spanStart+firstEnd, index, delim)
		block.Advance(end - firstEnd)
		return em
	}
}

// parseEmphasisBody walks block from its current position up to the absolute
// offset endAbs, appending children to container — literal text, a code span,
// a plain inline link, or a nested emphasis matched at the same cutoff
// `inline.go`'s `parseFrom` uses (spec 011 behavior 22; wired up in T10).
//
// It leaves block positioned at exactly endAbs.
func parseEmphasisBody(container ast.Node, block text.Reader, endAbs, cutoff int, delim byte) {
	for {
		line, segment := block.PeekLine()
		if segment.Start >= endAbs || len(line) == 0 {
			return
		}
		limit := endAbs - segment.Start
		if limit > len(line) {
			limit = len(line)
		}
		line = line[:limit]

		trigger := -1
		for i, c := range line {
			if c == '`' || c == '[' || c == '*' || c == '_' {
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
			if node, ok := buildBodyCodeSpan(block); ok {
				container.AppendChild(container, node)
				continue
			}
		case '[':
			if node, ok := buildBodyLink(block, endAbs); ok {
				container.AppendChild(container, node)
				continue
			}
		case '*', '_':
			nestedPatterns := asteriskPatterns
			if line[0] == '_' {
				nestedPatterns = underscorePatterns
			}
			nestedCutoff := -1
			// Only the processor that produced the parent is cut off — the
			// same rule inline.go's fromDelim carries.
			if line[0] == delim {
				nestedCutoff = cutoff
			}
			data := string(line)
			if index, end, fs, fe, ss, se, ok := matchEmphasis(nestedPatterns, data, 0, nestedCutoff); ok {
				node := buildEmphasis(block, segment.Start, index, line[0], end, fs, fe, ss, se)
				container.AppendChild(container, node)
				continue
			}
		}

		// The trigger byte matched nothing: literal.
		ast.MergeOrAppendTextSegment(container, segment.WithStop(segment.Start+1))
		block.Advance(1)
	}
}

// buildBodyCodeSpan is `BacktickInlineProcessor` (spec 008), reused unchanged:
// content between a run of N backticks and the next run of exactly N, stripped
// at both ends by the renderer (`codespan.go`'s `codeSpanRenderer`), which is
// why this hands back the raw, unstripped segment.
func buildBodyCodeSpan(block text.Reader) (ast.Node, bool) {
	line, segment := block.PeekLine()
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

// buildBodyLink is a minimal `[label](destination)` matcher for a link inside
// an emphasis body — python-markdown parses the label as plain text here
// (unlike a top-level link, which the label range is handed back to goldmark's
// own `linkParser` for at the top level, per `html.go`'s registration), because
// none of spec 011's shapes need a formatted label inside emphasis. It declines
// anything reference-style, unbalanced, or reaching past endAbs, the same
// narrowness `imageParser` already documents.
func buildBodyLink(block text.Reader, endAbs int) (ast.Node, bool) {
	line, segment := block.PeekLine()
	if len(line) < 2 || line[0] != '[' {
		return nil, false
	}
	_, after := matchBracketed(line, 1, '[', ']')
	if after < 0 || after >= len(line) || line[after] != '(' {
		return nil, false
	}
	href, title, hasTitle, end, ok := getLink(line, after)
	if !ok || segment.Start+end > endAbs {
		return nil, false
	}

	link := ast.NewLink()
	link.Destination = href
	if hasTitle {
		link.Title = title
	}
	labelSegment := segment.WithStop(segment.Start + after - 1)
	labelSegment = labelSegment.WithStart(segment.Start + 1)
	link.AppendChild(link, ast.NewTextSegment(labelSegment))

	block.Advance(end)
	return link, true
}
