package yamlreader

import "fmt"

// NonPrintableError is ruamel's reader refusing a character outside YAML's
// printable set — `Reader.check_printable` raising `ReaderError`
// (`ruamel/yaml/reader.py:216-227`).
//
// **goccy accepts every C0 and C1 control character, and `U+FFFE`/`U+FFFF`,
// anywhere in a document**, so the port rendered a complete CV at exit 0 for
// input upstream refuses at exit 1 (spec delta 002-P §0).
//
// ruamel carries neither a context mark nor a problem mark on this error
// (`reader.py:35-42`), so `get_yaml_error_location` returns `None` and the
// Location column is the bare source name with no line number.
type NonPrintableError struct {
	// Rune is the first offending character in source order. ruamel's two
	// detection paths — the ASCII fast path (`reader.py:193-200`) and the
	// regex fallback (`:203-207`) — both report the first one.
	Rune rune
}

// Error is the first line of `ReaderError.__str__` (`reader.py:52-56`) with the
// fixed reason of `check_printable` (`reader.py:221-227`). The second line, the
// stream name and position, is dropped by the model builder before the message
// reaches a panel, so it is not built here.
//
// The hex is lowercase and zero-padded to a minimum of four digits, a `%04x`.
// A codepoint above `U+FFFF` would print five or six, which the rule makes
// unreachable: ruamel's pattern admits the astral range whole.
func (e *NonPrintableError) Error() string {
	return fmt.Sprintf(
		"unacceptable character #x%04x: special characters are not allowed", e.Rune)
}

// checkPrintable mirrors `Reader.check_printable` (`reader.py:216-227`).
//
// It runs **before any scanning**, as it does upstream: `read_yaml` hands a
// `str` to `yaml.load` (`yaml_reader.py:53`) and the `Reader.stream` setter
// checks the whole document before assigning the buffer (`reader.py:105-108`).
// A document carrying both a forbidden character and a syntax error therefore
// reports the forbidden character.
//
// Bytes that are not valid UTF-8 decode to `U+FFFD` here, which the rule
// permits. That is deliberate: Python never reaches this check for such a file,
// because `read_text` raises `UnicodeDecodeError` first — the unhandled
// traceback class of D-011 — so there is no upstream message to match.
func checkPrintable(src string) error {
	for _, r := range src {
		if !isPrintable(r) {
			return &NonPrintableError{Rune: r}
		}
	}
	return nil
}

// isPrintable is the complement of ruamel's `NON_PRINTABLE` character class
// (`reader.py:187-189`):
//
//	[^\x09\x0A\x0D\x20-\x7E\x85\xA0-\uD7FF\uE000-\uFFFD\U00010000-\U0010FFFF]
//
// So TAB, LF, CR and NEL are permitted while DEL, the rest of C0, the rest of
// C1 and the two `U+FFFE`/`U+FFFF` non-characters are not — and every astral
// codepoint is permitted, non-characters included.
func isPrintable(r rune) bool {
	switch {
	case r == 0x09 || r == 0x0A || r == 0x0D || r == 0x85:
		return true
	case r >= 0x20 && r <= 0x7E:
		return true
	case r >= 0xA0 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}
