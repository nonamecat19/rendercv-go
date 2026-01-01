package process

import "strings"

// The inline pattern priorities of `build_inlinepatterns`
// (`markdown/inlinepatterns.py:73-95`). Only the ones this file resolves are
// named; `reference` (170) and the three shortcut/reference forms need the
// link-definition map, which neither path builds.
const (
	prioBacktick  = 190
	prioEscape    = 180
	prioLink      = 160
	prioImage     = 150
	prioAutolink  = 120
	prioAutomail  = 110
	prioHTML      = 90
	prioEntity    = 80
	prioNotStrong = 70
	prioEmStrong  = 60
)

// escapedChars is `Markdown.ESCAPED_CHARS` (`markdown/core.py:111`).
//
// **A backslash before anything else is not an escape at all.**
// `EscapeInlineProcessor.handleMatch` returns `None` for a character outside
// this set (`inlinepatterns.py:350-360`), which makes the pattern decline and
// leaves the backslash as literal text — so `**\alpha**` keeps its backslash
// and `*C:\temp*` keeps its path separator. Treating `\` + any byte as an
// escape ate both.
const escapedChars = "\\`*_{}[]()>#+-.!"

func isEscapedChar(b byte) bool { return strings.IndexByte(escapedChars, b) >= 0 }

// maskedByte is what a resolved construct's bytes become. It stands in for
// python-markdown's stash placeholder — `util.STX + "wzxhzdk:%d" + util.ETX`
// (`markdown/util.py`) — and like that placeholder it is not a word character,
// not whitespace, and not any pattern's delimiter, so every lookaround that
// runs into one behaves the same on both sides.
const maskedByte = 0x00

// maskAbove returns a copy of data in which every inline construct registered
// **above** the floor priority has been blanked to `maskedByte`, length
// preserved.
//
// # Why the port needs this at all
//
// python-markdown's inline pass is not one left-to-right scan. It walks the
// pattern registry in priority order and gives each pattern a full pass over
// the whole block, replacing what it matches with an opaque stash placeholder
// before the next pattern ever runs (`treeprocessors.py:120-140`, and the
// module docstring at `inlinepatterns.py:60-70`: "backticks and escaped
// characters have to be handled before everything else so that we can preempt
// any markdown patterns by escaping them"). By the time `em_strong` (60) sees
// the text, a code span, an escape, a link, an autolink, an inline tag and a
// whitespace-isolated delimiter run have all stopped being the characters the
// author wrote.
//
// Both of this port's paths scan the raw string instead, so every one of those
// pre-emptions has to be reconstructed or the emphasis matchers trip over
// bytes upstream never showed them. Four measured examples, all of them
// ordinary CV prose:
//
//	**bold ** text        not_strong (70) eats the second `**`  → no strong
//	Used *the `*` op*     backtick (190) hides the inner `*`    → one em, not two
//	*a<!-- * -->b*        html (90) hides the comment's `*`      → one em
//	*a\*b*                escape (180) hides the escaped `*`     → closes on the real one
//
// Blanking the **whole** construct rather than only its delimiters is both
// simpler and closer to upstream, whose placeholder replaces the whole match.
// Length is preserved so that every offset the matchers return still indexes
// the original data, which is what the callers slice.
//
// Passes run in descending priority, each over the regions the earlier passes
// left alone, mirroring the registry order rather than a single scan — a
// single scan would let a `[` claim text that upstream's backtick pass had
// already taken.
func maskAbove(data string, floor int) string {
	if data == "" {
		return data
	}
	b := []byte(data)

	if floor < prioBacktick {
		maskPass(b, func(rest []byte, _ int) int {
			if rest[0] != '`' {
				return 0
			}
			width := 0
			for width < len(rest) && rest[width] == '`' {
				width++
			}
			if _, after := matchBackticks(rest, width, width); after >= 0 {
				return after
			}
			return 0
		})
	}
	if floor < prioEscape {
		maskPass(b, func(rest []byte, _ int) int {
			if rest[0] == '\\' && len(rest) > 1 && isEscapedChar(rest[1]) {
				return 2
			}
			return 0
		})
	}
	if floor < prioLink {
		maskPass(b, func(rest []byte, at int) int {
			if rest[0] != '[' || (at > 0 && b[at-1] == '!') {
				return 0 // NOIMG (`inlinepatterns.py:100`)
			}
			return spanOfBracketedLink(rest, 1)
		})
	}
	if floor < prioImage {
		maskPass(b, func(rest []byte, _ int) int {
			if len(rest) < 2 || rest[0] != '!' || rest[1] != '[' {
				return 0
			}
			return spanOfBracketedLink(rest, 2)
		})
	}
	if floor < prioAutolink {
		maskPass(b, func(rest []byte, _ int) int {
			if rest[0] != '<' {
				return 0
			}
			if end, _, ok := matchAutolink(string(rest)); ok {
				return end
			}
			return 0
		})
	}
	if floor < prioHTML {
		maskPass(b, func(rest []byte, _ int) int {
			if rest[0] != '<' && rest[0] != '&' {
				return 0
			}
			if end, ok := matchRawHTML(string(rest)); ok {
				return end
			}
			return 0
		})
	}
	if floor < prioNotStrong {
		maskPass(b, maskNotStrong(b))
	}

	return string(b)
}

// maskPass runs one pattern's full left-to-right pass. `match` is handed the
// remainder from a position that is not already masked and returns the width
// it claims, or 0.
func maskPass(b []byte, match func(rest []byte, at int) int) {
	for i := 0; i < len(b); i++ {
		if b[i] == maskedByte {
			continue
		}
		width := match(b[i:], i)
		if width <= 0 {
			continue
		}
		for j := i; j < i+width; j++ {
			b[j] = maskedByte
		}
		i += width - 1
	}
}

// spanOfBracketedLink is the width of a whole `[label](dest)` starting at the
// bracket `labelStart` follows — `getText`'s balanced-bracket scan plus
// `getLink`'s (`inlinepatterns.py:716-850`). Reference and shortcut forms
// return 0, since neither is resolved here.
func spanOfBracketedLink(rest []byte, labelStart int) int {
	_, after := matchBracketed(rest, labelStart, '[', ']')
	if after < 0 || after >= len(rest) || rest[after] != '(' {
		return 0
	}
	if _, _, _, end, ok := getLink(rest, rest, after); ok {
		return end
	}
	return 0
}

// maskNotStrong is `SimpleTextInlineProcessor(NOT_STRONG_RE)` at priority 70,
// `((^|(?<=\s))(\*{1,3}|_{1,3})(?=\s|$))` (`inlinepatterns.py:150`).
//
// **This is the single largest divergence Wave C's first pass left open**, 124
// of 147 measured shapes. A delimiter run that begins the block or follows
// whitespace and is followed by whitespace or the end is claimed as literal
// text *before* `em_strong` runs, so it can never open or close anything:
//
//	**bold ** text   →  the second `**` is literal, and the first has no closer
//	a ** b ** c      →  both runs literal
//	_a _             →  the trailing `_` is literal
//
// The port checked only the trigger's own position, which cannot see that the
// run that would have *closed* the span is the disqualified one. The run
// length is tried longest-first at 3, and a run of four or more matches
// nothing at all — `\*{1,3}` cannot reach the closing lookahead past a fourth
// delimiter, and no shorter alternative starts at a whitespace boundary.
func maskNotStrong(b []byte) func(rest []byte, at int) int {
	return func(rest []byte, at int) int {
		if rest[0] != '*' && rest[0] != '_' {
			return 0
		}
		if at > 0 && !isSpaceByte(b[at-1]) {
			return 0 // `(^|(?<=\s))`
		}
		run := 0
		for run < len(rest) && rest[run] == rest[0] {
			run++
		}
		for width := min(run, 3); width >= 1; width-- {
			if width < run {
				continue // `(?=\s|$)` cannot hold with a delimiter next
			}
			if width >= len(rest) || isSpaceByte(rest[width]) {
				return width
			}
		}
		return 0
	}
}
