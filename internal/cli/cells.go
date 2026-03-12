package cli

import "strings"

// This file mirrors `rich/cells.py`, the module every Rich layout measures with.
// The width table it reads lives in celltable.go and is captured from the
// vendored Rich by `just cellprobe` — reproducing Rich's own data rather than
// borrowing a third-party width table, whose Unicode revision would drift from
// upstream's and change a panel's padding by a column.
//
// **Rich resolves the revision at run time**: `cell_len(text, "auto")` honours
// a `UNICODE_VERSION` environment variable and falls back to the latest
// (`rich/_unicode_data/__init__.py`). The port carries the latest only, so a
// user who sets `UNICODE_VERSION` to an old revision gets upstream's *current*
// widths here. Nothing in RenderCV sets that variable.

const (
	// zeroWidthJoiner glues emoji into one grapheme and occupies no columns.
	zeroWidthJoiner = '‍'
	// variationSelector16 asks for the emoji presentation of the character
	// before it, which for a narrow character makes it two columns wide.
	variationSelector16 = '️'
)

// cellSpan is one row of Rich's width table: the codepoints from start to end
// inclusive each occupy width columns.
type cellSpan struct {
	start, end rune
	width      int
}

// cellLen is Rich's `cell_len` (`rich/cells.py`): the number of terminal
// columns a string occupies, which is what every border, pad and fold in a Rich
// layout is measured in.
//
// It is not the rune count. An East-Asian wide character is two columns, a
// combining mark is none, and a zero-width joiner sequence is measured as its
// parts less the joiners.
func cellLen(text string) int {
	// The fast path in Rich is `_is_single_cell_widths`, a set membership test
	// over the common ranges; it is an optimization, not a different rule, and
	// the loop below gives the same answer.
	if !strings.ContainsRune(text, zeroWidthJoiner) && !strings.ContainsRune(text, variationSelector16) {
		total := 0
		for _, character := range text {
			total += characterCellSize(character)
		}
		return total
	}

	// The joiner/selector path. **A joiner consumes the character after it as
	// well as itself**, which is why an emoji family measures as its first
	// member alone: `cell_len` skips two codepoints there, not one.
	total := 0
	runes := []rune(text)
	var lastMeasured rune
	haveLastMeasured := false
	for index := 0; index < len(runes); index++ {
		character := runes[index]
		switch character {
		case zeroWidthJoiner:
			index++
		case variationSelector16:
			if haveLastMeasured {
				if _, wide := narrowToWide[lastMeasured]; wide {
					total++
				}
				haveLastMeasured = false
			}
		default:
			if width := characterCellSize(character); width != 0 {
				lastMeasured = character
				haveLastMeasured = true
				total += width
			}
		}
	}
	return total
}

// characterCellSize is Rich's `get_character_cell_size`: 0, 1 or 2 columns.
//
// **The control-character rule is upstream's, quirk included.** Everything
// below U+0020 except NUL — and everything from U+007F to U+009F — scores 0,
// so an unstripped `\x01` in an input value makes upstream's own panel one
// column wider than the box it draws. `strip_control_codes` removes five other
// codepoints and not that one (`rich/control.py:9-15`), so the quirk is
// reachable, and matching upstream means keeping it.
func characterCellSize(character rune) int {
	if (character != 0 && character < 32) || (character >= 0x7F && character < 0xA0) {
		return 0
	}
	// Past the end of the table everything is one column wide.
	if character > cellWidths[len(cellWidths)-1].end {
		return 1
	}

	lower, upper := 0, len(cellWidths)-1
	for lower <= upper {
		index := (lower + upper) >> 1
		span := cellWidths[index]
		switch {
		case character < span.start:
			upper = index - 1
		case character > span.end:
			lower = index + 1
		default:
			return span.width
		}
	}
	return 1
}

// cellGrapheme is one unit of `splitGraphemes`: the byte range of a character
// together with the zero-width marks and joiners that belong to it, and the
// columns the whole thing occupies.
type cellGrapheme struct {
	start, end int
	width      int
}

// splitGraphemes is Rich's `split_graphemes`: the spans a string may be cut at
// without splitting a character from the marks that belong to it.
//
// The associations are Rich's: a zero-width character joins the span before it,
// a **joiner swallows the character after it** — which is why an emoji family
// is one span and not five — and a variation selector may widen the span it
// lands on.
//
// **Two inputs make upstream loop forever**: a string that opens with a
// variation selector, and one that opens with a combining mark, both of which
// reach a branch that `continue`s without advancing. Verified against the
// vendored Rich, which hangs on `split_graphemes("️abc")`. The port
// advances instead. Nothing else about those strings differs, and neither is
// reachable from `new` today: the padding path never calls this unless the row
// overflows the panel.
func splitGraphemes(text string) []cellGrapheme {
	runes := []rune(text)
	offsets := make([]int, len(runes)+1)
	offset := 0
	for i, character := range runes {
		offsets[i] = offset
		offset += len(string(character))
	}
	offsets[len(runes)] = offset

	var spans []cellGrapheme
	var lastMeasured rune
	haveLastMeasured := false

	for index := 0; index < len(runes); {
		character := runes[index]
		switch character {
		case zeroWidthJoiner:
			// The joiner *and the character after it* are folded into the
			// previous span, unmeasured.
			index = min(index+2, len(runes))
			if len(spans) > 0 {
				spans[len(spans)-1].end = offsets[index]
			}
		case variationSelector16:
			index++
			if len(spans) > 0 {
				last := &spans[len(spans)-1]
				last.end = offsets[index]
				if _, wide := narrowToWide[lastMeasured]; haveLastMeasured && wide {
					last.width++
					haveLastMeasured = false
				}
			}
		default:
			width := characterCellSize(character)
			start := offsets[index]
			index++
			if width != 0 {
				lastMeasured = character
				haveLastMeasured = true
				spans = append(spans, cellGrapheme{start: start, end: offsets[index], width: width})
			} else if len(spans) > 0 {
				// A zero-width character belongs to the character before it.
				spans[len(spans)-1].end = offsets[index]
			}
		}
	}
	return spans
}

// cutCells splits text at a column offset, the way Rich's `_split_text` does.
//
// **A cut that lands inside a double-width character replaces it with two
// spaces**, one on each side, so neither half is half a glyph.
func cutCells(text string, columns int) (head, tail string) {
	if columns <= 0 {
		return "", text
	}
	if cellLen(text) <= columns {
		return text, ""
	}

	width := 0
	for _, span := range splitGraphemes(text) {
		if width+span.width > columns {
			// The cut lands inside this span, which can only be a
			// double-width one.
			return text[:span.start] + " ", " " + text[span.end:]
		}
		width += span.width
		if width == columns {
			return text[:span.end], text[span.end:]
		}
	}
	return text, ""
}

// chopCells is Rich's `chop_cells`: fold text into lines of at most width
// columns, breaking between graphemes rather than inside them.
func chopCells(text string, width int) []string {
	var lines []string
	lineWidth := 0
	offset := 0
	for _, span := range splitGraphemes(text) {
		if lineWidth+span.width > width {
			lines = append(lines, text[offset:span.start])
			offset = span.start
			lineWidth = 0
		}
		lineWidth += span.width
	}
	if lineWidth != 0 {
		lines = append(lines, text[offset:])
	}
	return lines
}
