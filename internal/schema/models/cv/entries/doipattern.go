package entries

import (
	"unicode"
	"unicode/utf8"
)

// matchDOIPattern reports whether value matches the pattern pydantic enforces on
// PublicationEntry.doi, `\b10\..*`
// (third_party/rendercv/src/rendercv/schema/models/cv/entries/publication.py:34).
// Search semantics, not anchored: `prefix 10.5/x` matches (spec §3.11 behavior 18).
//
// The pattern is deliberately not handed to Go's regexp. pydantic-core evaluates it
// with the Rust `regex` crate, whose `\b` is defined over a Unicode `\w` class, while
// Go's `\b` is ASCII-only. Verified against the vendored Python: `①10.5` matches
// (U+2461 is category No, outside `\w`) while `ü10.5` and `ß10.5` do not. Go's regexp
// with the literal pattern accepts both and diverges (spec §5.1); doipattern_test.go
// pins that disagreement.
func matchDOIPattern(value string) bool {
	for i := 0; i+3 <= len(value); i++ {
		if value[i:i+3] != "10." {
			continue
		}
		// '1' is always a word rune, so a boundary exists at i whenever the rune
		// ending at i is not one — including at the start of the value.
		if i == 0 {
			return true
		}
		prev, _ := utf8.DecodeLastRuneInString(value[:i])
		if !isWordRune(prev) {
			return true
		}
	}
	return false
}

// isWordRune reports whether r belongs to the Rust `regex` crate's `\w` class,
// `\p{Alphabetic} ∪ \p{M} ∪ \p{Nd} ∪ \p{Pc} ∪ \p{Join_Control}` (plan §5). Go has no
// Alphabetic table, so it is spelled out as letters plus Nl plus Other_Alphabetic.
func isWordRune(r rune) bool {
	switch {
	case unicode.IsLetter(r),
		unicode.Is(unicode.Nl, r),
		unicode.Is(unicode.Other_Alphabetic, r),
		unicode.Is(unicode.M, r),
		unicode.Is(unicode.Nd, r),
		unicode.Is(unicode.Pc, r):
		return true
	case r == 0x200C, r == 0x200D: // Join_Control
		return true
	default:
		return false
	}
}
