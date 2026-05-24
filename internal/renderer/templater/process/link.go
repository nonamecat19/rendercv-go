package process

import (
	"bytes"

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

	// **The scan is the block's text, not the line's** (spec 011 §9.3).
	// `getLink` is handed `data` — the whole text the tree processor is
	// walking (`inlinepatterns.py:698, 731`) — and python-markdown's block
	// text is the block's lines joined by `\n`, so a destination may cross
	// one: `[t](a\nb)` is `<a href="a\nb">` upstream.
	tail := readBlockTail(block)
	if tail.text == nil {
		return nil
	}

	// getText's own bracket-balance scan (`inlinepatterns.py:832-850`), run
	// over the escape-masked text so a `\]` in the label is not a closing
	// bracket: `[a\](b)` is not a link at all upstream.
	scan := []byte(maskAbove(string(tail.text), prioLink))
	_, after := matchBracketed(scan, 1, '[', ']')
	if after < 0 || after >= len(scan) || scan[after] != '(' {
		return nil
	}
	// **A label that crosses a line is left to goldmark**, which already
	// agrees with upstream on one and whose parser this one only outranks for
	// the shapes it adds. `parseEmphasisBody` takes source offsets on the
	// current line, so claiming a multi-line label here would need the label
	// side of the same mapping for no measured gain.
	if after > len(line) {
		return nil
	}
	href, title, hasTitle, parenEnd, ok := getLink(scan, tail.text, after)
	if !ok {
		return nil
	}

	link := ast.NewLink()
	link.Destination = href
	if hasTitle {
		link.Title = title
	}

	advanceTo(block, segment.Start+1) // the opening `[`
	parseEmphasisBody(link, block, pc, segment.Start+after-1, -1, noCutoffDelim, false)
	advanceTo(block, tail.sourceOffset(parenEnd)) // the closing `]` through the `)`

	return link
}

// blockTail is the block's text from the reader's position to the end of the
// block, with the mapping back to source offsets that advancing needs.
//
// The text is the lines joined as the reader hands them over, which is how
// python-markdown's block text is built too: a container's prefix is already
// gone from both, so `> [t](a\n> b)` scans as `[t](a\nb)` on both sides.
type blockTail struct {
	text []byte
	// lines is one entry per line: where it starts in text, and where the same
	// byte is in the source.
	lines []tailLine
}

type tailLine struct{ inText, inSource int }

// readBlockTail reads the rest of the block without moving the reader.
//
// It returns a nil `text` for a line the mapping cannot describe — one goldmark
// has given synthetic padding, which stands in for a tab it expanded. Nothing
// reaches that here, because `normalizeWhitespace` has already expanded every
// tab in the document (`html.go`), but a wrong offset would be a silently
// mangled link rather than a declined one.
func readBlockTail(block text.Reader) blockTail {
	line, segment := block.Position()
	defer block.SetPosition(line, segment)

	source := block.Source()
	var tail blockTail
	for first := true; ; first = false {
		value, current := block.PeekLine()
		if len(value) == 0 {
			break
		}
		if current.Padding != 0 {
			return blockTail{}
		}
		start := current.Start
		if !first {
			start = indentedLineStart(source, current.Start)
		}
		tail.lines = append(tail.lines, tailLine{inText: len(tail.text), inSource: start})
		tail.text = append(tail.text, source[start:current.Start]...)
		tail.text = append(tail.text, value...)
		block.AdvanceLine()
	}
	return tail
}

// indentedLineStart gives back the indentation goldmark trimmed off a
// continuation line, which upstream's block text still has.
//
// A paragraph's second and later lines are stored `TrimLeftSpace`d
// (`parser/paragraph.go`), where python-markdown `lstrip`s the **block**
// (`blockprocessors.py`, `ParagraphProcessor.run`) and so strips the first line
// only: `[t](a\n  b)` is `href="a\n  b"` upstream, two spaces and all.
//
// What stands between the line head and the segment is the container prefix
// plus that indentation, and only the second is upstream's to keep. A blockquote
// marker is `>` and **one** optional space on both sides —
// `BlockQuoteProcessor.RE` is `(^|\n)[ ]{0,3}>[ ]?` (`blockprocessors.py`), and
// goldmark's parser consumes the same — so the last `>` on the line, and one
// space after it, is where upstream's own text begins:
//
//	[t](a\n  b)        href="a\n  b"
//	> [t](a\n>   b)    href="a\n  b"   the `> ` goes, the two spaces stay
//
// A list item's continuation line carries no marker of its own, so its whole
// gap is indentation.
func indentedLineStart(source []byte, start int) int {
	head := bytes.LastIndexByte(source[:start], '\n') + 1
	if marker := bytes.LastIndexByte(source[head:start], '>'); marker >= 0 {
		head += marker + 1
		if head < start && source[head] == ' ' {
			head++
		}
	}
	if len(bytes.Trim(source[head:start], " \t")) != 0 {
		return start
	}
	return head
}

// sourceOffset is the source position of one offset into the tail's text.
func (t blockTail) sourceOffset(offset int) int {
	at := t.lines[0]
	for _, l := range t.lines {
		if l.inText > offset {
			break
		}
		at = l
	}
	return at.inSource + offset - at.inText
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
// It scans `scan` but slices `src`, which are the same length and differ only
// in that `scan` has had its higher-priority constructs blanked by `maskAbove`
// — upstream's `escape` (180) runs ahead of `link` (160), so by the time this
// scanner sees the text a `\)` the author wrote is a stash placeholder and not
// a closing parenthesis. `[t](a\)b)` is one link to `a)b` upstream and was two
// pieces of wreckage here. Callers that have no mask pass the same slice twice.
//
// Python's `dequote` (redundant-quote-stripping on an already-extracted title)
// is not reproduced: nothing in spec 011's corpus reaches it.
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
		// `href = data[start_index:last_bracket - 1]` / `index = last_bracket`
		// (`inlinepatterns.py:817-819`) — and `last_bracket` is still its
		// initial `-1` whenever no backtracked `)` was ever found. **Python
		// slices that from the end**, so upstream quietly takes `data[:-2]` and
		// keeps the block's final byte; a straight index panics.
		// `[]("(` is a five-byte input that reaches it, renders
		// `<p><a href=""></a>(</p>` upstream, and crashed the renderer here —
		// `rendercv-go render` dumped a goroutine stack on a CV highlight
		// containing it.
		hrefEnd := pyIndex(lastBracket-1, len(src))
		if hrefEnd < startIndex {
			hrefEnd = startIndex
		}
		href = src[startIndex:hrefEnd]
		return trimSpaceBytes(href), nil, false, pyIndex(lastBracket, len(src)), true
	}

	return nil, nil, false, 0, false
}

// pyIndex resolves one Python sequence index against a length: a negative
// index counts from the end, and the result is clamped to [0, length] the way
// a Python *slice* bound is (an out-of-range slice bound is not an error there,
// unlike an out-of-range element index).
func pyIndex(i, length int) int {
	if i < 0 {
		i += length
	}
	if i < 0 {
		return 0
	}
	if i > length {
		return length
	}
	return i
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
