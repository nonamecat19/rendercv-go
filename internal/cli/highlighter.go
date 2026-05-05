package cli

import "unicode"

// Rich's `ReprHighlighter` (`rich/highlighter.py:80-103`), which `rich.print`
// applies to every **bare string** it is given.
//
// **Only the `number` rule is implemented, because only it is reachable.**
// RenderCV prints exactly one bare string through `rich.print`
// (`cli/new_command/print_welcome.py:14`), and of the highlighter's fifteen
// named groups the greeting matches one: the digit at the end of the version.
// Measured, and it is visible in the bytes —
// `Welcome to ESC[38;5;26mRenderCV v2.ESC[0mESC[1;38;5;26m8ESC[0m!` — where `8`
// is bold and `v2.` is not (spec 012 delta §2.6). Panel *contents* are not
// highlighted at all: a `Panel` is a renderable, not a string, so `render_str`
// never sees it.
//
// The other rules belong to `--help`, whose three highlighters are typer's and
// not this one (delta §4).

// HighlightRepr lays the highlighter's spans **underneath** the text's own —
// `render_str` builds the highlighted text first and then copies the markup's
// styles onto it (`rich/console.py:1468-1472`).
//
// The order is the whole point: the number's span comes first in the list and
// the markup's `dodger_blue3` comes second, so the colour of the later span
// wins and the digit is rendered bold *blue* rather than the highlighter's own
// bold cyan.
func HighlightRepr(text Text) Text {
	spans := numberSpans([]rune(text.Plain))
	if len(spans) == 0 {
		return text
	}
	return Text{Plain: text.Plain, Spans: append(spans, text.Spans...)}
}

// numberSpans is the `number` alternative of the highlighter's combined regex
// (`rich/highlighter.py:98`):
//
//	(?<!\w)\-?[0-9]+\.?[0-9]*(e[-+]?\d+?)?\b|0x[0-9a-fA-F]*
//
// It is scanned by hand rather than by `regexp` because Go has no lookbehind,
// and dropping the `(?<!\w)` would change the answer rather than merely
// widen it: in `v2.8` Python rejects a match at `2` and then **retries at the
// next position**, ending up with `8`, where a Go pattern without the
// lookbehind matches `2.8` in one go and a post-hoc rejection loses the `8`
// entirely.
func numberSpans(runes []rune) []Span {
	var spans []Span
	for start := 0; start < len(runes); {
		end, ok := matchNumber(runes, start)
		if !ok {
			start++
			continue
		}
		spans = append(spans, Span{Start: start, End: end, Style: StyleReprNumber})
		start = end
	}
	return spans
}

// matchNumber is one attempt of the pattern at one offset, in the alternation's
// own order: the decimal form first, then the hexadecimal one.
func matchNumber(runes []rune, start int) (end int, ok bool) {
	if end, ok = matchDecimal(runes, start); ok {
		return end, true
	}
	return matchHex(runes, start)
}

func matchDecimal(runes []rune, start int) (end int, ok bool) {
	// `(?<!\w)`.
	if start > 0 && isWordRune(runes[start-1]) {
		return 0, false
	}

	position := start
	if position < len(runes) && runes[position] == '-' {
		position++
	}
	digits := position
	for position < len(runes) && isDigit(runes[position]) {
		position++
	}
	if position == digits {
		return 0, false
	}
	if position < len(runes) && runes[position] == '.' {
		position++
	}
	for position < len(runes) && isDigit(runes[position]) {
		position++
	}
	// `(e[-+]?\d+?)?` — one digit is enough, the `+?` being lazy.
	if exponent, matched := matchExponent(runes, position); matched {
		position = exponent
	}
	// `\b`: the last matched rune is a word rune by construction, so the
	// boundary holds unless the next one is a word rune too.
	if position < len(runes) && isWordRune(runes[position]) {
		return 0, false
	}
	return position, true
}

func matchExponent(runes []rune, start int) (end int, ok bool) {
	position := start
	if position >= len(runes) || runes[position] != 'e' {
		return 0, false
	}
	position++
	if position < len(runes) && (runes[position] == '-' || runes[position] == '+') {
		position++
	}
	if position >= len(runes) || !isDigit(runes[position]) {
		return 0, false
	}
	return position + 1, true
}

// matchHex is `0x[0-9a-fA-F]*`, which carries no lookbehind and no boundary.
func matchHex(runes []rune, start int) (end int, ok bool) {
	if start+1 >= len(runes) || runes[start] != '0' || runes[start+1] != 'x' {
		return 0, false
	}
	position := start + 2
	for position < len(runes) && isHexDigit(runes[position]) {
		position++
	}
	return position, true
}

// isWordRune is Python's `\w` for a `str` pattern: a letter, a digit or an
// underscore, and **Unicode-aware** — Python's `\w` matches `é` and `٣`, which
// an ASCII-only test would let start a number.
func isWordRune(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
}

func isDigit(character rune) bool {
	return character >= '0' && character <= '9'
}

func isHexDigit(character rune) bool {
	switch {
	case isDigit(character), character >= 'a' && character <= 'f', character >= 'A' && character <= 'F':
		return true
	}
	return false
}
