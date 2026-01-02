package process

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// linkRenderer exists because **python-markdown does not URL-escape a
// destination at all.**
//
// `LinkInlineProcessor.handleMatch` puts the href on the element as it was
// written (`inlinepatterns.py:706-711`) and the serializer's only job on an
// attribute is HTML escaping (`markdown/serializers.py`). goldmark runs
// `util.URLEscape` first, so a perfectly ordinary CV link came out percent-
// encoded where upstream left it alone:
//
//	[t](héllo.png)   →  href="héllo.png"     not  "h%C3%A9llo.png"
//	[t](<a b>)       →  href="a b"           not  "a%20b"
//	[t](a<b)         →  href="a&lt;b"        not  "a%3Cb"
//
// A non-ASCII filename and a path with a space are both things a user writes
// without thinking about it, so this is not an exotic shape.
type linkRenderer struct {
	writer pythonWriter
}

// RegisterFuncs claims the link node, and only that one.
func (r linkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
}

func (r linkRenderer) renderLink(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Link)

	_, _ = w.WriteString(`<a href="`)
	r.writer.writeAttribute(w, n.Destination)
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		r.writer.writeAttribute(w, n.Title)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')

	return ast.WalkContinue, nil
}

// linkParser is `LinkInlineProcessor` for the direct `[text](destination)`
// form (spec 011 §8). It claims that form outright — priority 190, ahead of
// goldmark's own `NewLinkParser` at 200 — because `getLink` is a strict
// superset of what goldmark's own parser accepts: every destination goldmark
// already matches, `getLink` matches identically, and it additionally accepts
// the unbracketed-space and mid-destination-quote shapes goldmark declines.
// Reference links (`[text][ref]`) and a shortcut form are not this shape and
// are declined, falling through to goldmark's own parser as before.
type linkParser struct{}

// Trigger is `[`. `!` belongs to imageParser; this declines right after one
// (`NOIMG`, `inlinepatterns.py:100`), the same rule `precededByBang` gives the
// Typst path.
func (linkParser) Trigger() []byte { return []byte{'['} }

// Parse claims one `[label](destination)`, or returns nil.
//
// The label is parsed as inline Markdown, not stashed as raw text — python's
// own `LinkInlineProcessor.handleMatch` recursively calls
// `self.parser.parseChunk(el, text)` on it (`inlinepatterns.py:698-699`), so
// `[**bold**](u)` and `[a ` + "`b`" + `](u)` nest a real `<strong>`/`<code>`
// inside the `<a>`, the same as goldmark's own link parser already did before
// this one replaced it for the direct-paren form. `buildBodyLink`
// (`emphasis_html.go`) is the same construction reused for a link inside an
// emphasis body.
//
// **A label never contains another link.** The reprocessing upstream does on a
// built element's text starts at `patternIndex + 1`
// (`treeprocessors.py:315`), one below the pattern that built it, so `link`
// (160) is out of reach from inside a link and `[a [b](c) d](u)` keeps its
// inner brackets literal. Everything below it — `image_link` at 150 and down —
// is still available, which is why `[![i](p.png)](u)` is an image inside a
// link. That is the `false` passed for `allowLink` here.
func (linkParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) == 0 || line[0] != '[' {
		return nil
	}
	if segment.Start > 0 && block.Source()[segment.Start-1] == '!' {
		return nil
	}

	// getText's own bracket-balance scan (`inlinepatterns.py:832-850`).
	_, after := matchBracketed(line, 1, '[', ']')
	if after < 0 || after >= len(line) || line[after] != '(' {
		return nil
	}
	href, title, hasTitle, parenEnd, ok := getLink(line, line, after)
	if !ok {
		return nil
	}

	link := ast.NewLink()
	link.Destination = href
	if hasTitle {
		link.Title = title
	}

	block.Advance(1) // the opening `[`
	parseEmphasisBody(link, block, pc, segment.Start+after-1, -1, noCutoffDelim, false)
	block.Advance(parenEnd - (after - 1)) // the closing `]` through the `)`

	return link
}

// getLink is `LinkInlineProcessor.getLink` (`inlinepatterns.py:716-830`): the
// paren-and-quote-tracking scanner between a link's `(` and its balancing `)`.
// index must point at the `(` — every caller checks that first, matching
// image.go's `matchBracketed` convention.
//
// hasTitle distinguishes an absent title (python's `None`) from an empty one
// (`”`), which the angle-bracket form with an empty quoted title can produce;
// `linkRenderer` and `imageRenderer` only write a `title=""` attribute when
// `Title != nil`.
//
// It scans `scan` but slices `src`. The two are the same length and are the
// same bytes until a caller has something to hide from the scanner — an
// escape-masked copy, which the next commit's callers pass. Everything here
// passes the same slice twice, so this split is a no-op by construction.
//
// unescape and Python's own `dequote` (backslash-escape resolution and
// redundant-quote-stripping on an already-extracted title) are not
// reproduced: nothing in spec 011's corpus reaches either, and this file's own
// `linkRenderer` comment already documents that the destination is not
// further transformed once matched.
func getLink(scan, src []byte, index int) (href, title []byte, hasTitle bool, end int, ok bool) {
	data := scan
	if dest, ttl, hasTtl, angleEnd, matched := matchAngleLink(scan, src, index); matched {
		return trimSpaceBytes(dest), normalizeTitleWhitespace(ttl), hasTtl, angleEnd, true
	}

	bracketCount := 1
	backtrackCount := 1
	startIndex := index + 1
	lastBracket := -1

	var quote, altQuote byte
	startQuote, exitQuote := -1, -1
	startAltQuote, exitAltQuote := -1, -1
	ignoreMatches := false

	var last byte

	for pos := startIndex; pos < len(data); pos++ {
		c := data[pos]
		switch c {
		case '(':
			if !ignoreMatches {
				bracketCount++
			} else if backtrackCount > 0 {
				backtrackCount--
			}
		case ')':
			switch {
			case (exitQuote >= 0 && quote == last) || (exitAltQuote >= 0 && altQuote == last):
				bracketCount = 0
			case !ignoreMatches:
				bracketCount--
			case backtrackCount > 0:
				backtrackCount--
				if backtrackCount == 0 {
					lastBracket = pos + 1
				}
			}
		case '\'', '"':
			switch {
			case quote == 0:
				ignoreMatches = true
				backtrackCount = bracketCount
				bracketCount = 1
				startQuote = pos + 1
				quote = c
			case c != quote && altQuote == 0:
				startAltQuote = pos + 1
				altQuote = c
			case c == quote:
				exitQuote = pos + 1
			case altQuote != 0 && c == altQuote:
				exitAltQuote = pos + 1
			}
		}

		next := pos + 1

		if bracketCount == 0 {
			switch {
			case exitQuote >= 0 && quote == last:
				href = src[startIndex : startQuote-1]
				title = src[startQuote : exitQuote-1]
				hasTitle = true
			case exitAltQuote >= 0 && altQuote == last:
				href = src[startIndex : startAltQuote-1]
				title = src[startAltQuote : exitAltQuote-1]
				hasTitle = true
			default:
				href = src[startIndex : next-1]
			}
			return trimSpaceBytes(href), normalizeTitleWhitespace(title), hasTitle, next, true
		}

		if c != ' ' {
			last = c
		}
	}

	if bracketCount != 0 && backtrackCount == 0 {
		href = src[startIndex : lastBracket-1]
		return trimSpaceBytes(href), nil, false, lastBracket, true
	}

	return nil, nil, false, 0, false
}

// matchAngleLink is `RE_LINK`'s angle-bracket alternative,
// `\(\s*(<[^<>]*>)\s*(?:('[^']*'|"[^"]*")\s*)?\)`, index pointing at the `(`.
// It splits scanning from slicing for the reason `getLink` documents.
func matchAngleLink(data, src []byte, index int) (dest, title []byte, hasTitle bool, end int, ok bool) {
	pos := index + 1
	for pos < len(data) && isSpaceByte(data[pos]) {
		pos++
	}
	if pos >= len(data) || data[pos] != '<' {
		return nil, nil, false, 0, false
	}
	closeAngle := -1
	for i := pos + 1; i < len(data); i++ {
		if data[i] == '<' {
			return nil, nil, false, 0, false // [^<>]* forbids a nested <
		}
		if data[i] == '>' {
			closeAngle = i
			break
		}
	}
	if closeAngle < 0 {
		return nil, nil, false, 0, false
	}
	dest = src[pos+1 : closeAngle]

	next := closeAngle + 1
	for next < len(data) && isSpaceByte(data[next]) {
		next++
	}

	if next < len(data) && (data[next] == '\'' || data[next] == '"') {
		q := data[next]
		closeQuote := indexByteFrom(data, q, next+1)
		if closeQuote < 0 {
			return nil, nil, false, 0, false
		}
		title = src[next+1 : closeQuote]
		hasTitle = true
		next = closeQuote + 1
		for next < len(data) && isSpaceByte(data[next]) {
			next++
		}
	}

	if next >= len(data) || data[next] != ')' {
		return nil, nil, false, 0, false
	}
	return dest, title, hasTitle, next + 1, true
}

// normalizeTitleWhitespace is `RE_TITLE_CLEAN.sub(' ', title.strip())`
// (`inlinepatterns.py:826`): every whitespace byte becomes a plain space, but
// runs are **not** collapsed, so `"x y  z"` stays `x y  z` (spec 011 behavior
// 30). nil stays nil, so an absent title is not turned into a present, empty
// one.
func normalizeTitleWhitespace(title []byte) []byte {
	if title == nil {
		return nil
	}
	title = trimSpaceBytes(title)
	out := make([]byte, len(title))
	for i, b := range title {
		if isSpaceByte(b) {
			out[i] = ' '
		} else {
			out[i] = b
		}
	}
	return out
}
