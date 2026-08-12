package process

import (
	"unicode"
	"unicode/utf8"
)

// Python's `\w` and `\s` as the emphasis patterns see them.
//
// Every emphasis pattern in `inlinepatterns.py` is compiled `re.DOTALL |
// re.UNICODE`, so its `\w` and `\s` are the Unicode classes, not the ASCII
// ones: **`\s` is 29 codepoints and `\w` is 137,936**. Six `\w` occurrences and
// three `\s` occurrences across `SMART_*_RE` and `NOT_STRONG_RE` decide whether
// a delimiter run may open or close a span at all, so the classes are the
// behavior and not a detail of it.
//
// The port's older byte predicates cannot express either. `isSpaceByte`
// (`inline.go`) is the six ASCII spaces, and `isWordByte` (`emphasis.go`)
// counts **every** byte ≥ 0x80 as a word byte, which is right for a UTF-8
// continuation byte and wrong for `\xa0`, `·` and `—`. The two push opposite
// ways — the first emphasises where upstream does not, the second refuses where
// upstream does — and between them they moved 98 of 103 probed shapes on both
// backends. Spec `011-markdown-and-html/spec-delta-emphasis.md` §3 measures it;
// §3.3 is the reachable case: `**\xa0Senior Engineer**`, which is what pasting
// from a word processor produces, is four literal asterisk runs upstream and
// was `#strong[…]` here.
//
// Both predicates are swept against `re.match(r'\w', …)` and `re.match(r'\s',
// …)` over every non-surrogate codepoint by `TestPyClassesMatchPython`; the
// sweep is what says they are these functions and not merely near them.

// isPyWordRune is Python's `\w` under `re.UNICODE`: an underscore, any letter,
// or any number.
//
// **`unicode.IsNumber`, not `unicode.IsDigit`.** `\w` takes all of `N*` — `Nl`
// (236 codepoints, `Ⅷ`) and `No` (915, `²`) as well as `Nd` — and `IsDigit` is
// `Nd` alone.
func isPyWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// isPySpaceRune is Python's `\s` under `re.UNICODE`.
//
// Go's `unicode.IsSpace` is the same set **minus `U+001C`–`U+001F`**, the four
// information separators, which Python counts as whitespace and the Unicode
// `White_Space` property does not.
func isPySpaceRune(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// wordBefore is `(?<!\w)` inverted: whether the rune ending at `at` is a word
// rune. A position at the start of the text has no rune before it and so is not
// preceded by one, which is what the lookbehind says.
func wordBefore(text string, at int) bool {
	r, size := utf8.DecodeLastRuneInString(text[:at])
	return size > 0 && isPyWordRune(r)
}

// wordAt is `(?=\w)`: whether a word rune starts at `at`. The end of the text
// is not one, so `(?!\w)` holds there.
func wordAt(text string, at int) bool {
	if at >= len(text) {
		return false
	}
	r, size := utf8.DecodeRuneInString(text[at:])
	return size > 0 && isPyWordRune(r)
}

// spaceBefore is `(?<=\s)` over bytes, for `NOT_STRONG_RE`'s left anchor.
func spaceBefore(text []byte, at int) bool {
	r, size := utf8.DecodeLastRune(text[:at])
	return size > 0 && isPySpaceRune(r)
}

// spaceAt is `(?=\s)` over bytes. `NOT_STRONG_RE` spells its right anchor
// `(?=\s|$)`, and the caller supplies the `$`.
func spaceAt(text []byte, at int) bool {
	if at >= len(text) {
		return false
	}
	r, size := utf8.DecodeRune(text[at:])
	return size > 0 && isPySpaceRune(r)
}
