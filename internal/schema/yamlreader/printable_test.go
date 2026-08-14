package yamlreader_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// document is spec delta 002-P §4's probe: one character dropped into a scalar
// of the smallest CV that renders.
func document(ch rune) string {
	return fmt.Sprintf("cv:\n  name: %cA\n", ch)
}

// TestNonPrintableCharactersAreRejected is spec delta 002-P §1.1 — the
// characters ruamel's reader refuses, each with the message its `ReaderError`
// prints (§2).
//
// The rows below `U+007F` are the ones that were live: the port rendered five
// artifacts at exit 0 for every one of them.
func TestNonPrintableCharactersAreRejected(t *testing.T) {
	cases := []struct {
		name string
		ch   rune
		want string
	}{
		{"NUL", 0x00, "unacceptable character #x0000: special characters are not allowed"},
		{"SOH", 0x01, "unacceptable character #x0001: special characters are not allowed"},
		{"BS", 0x08, "unacceptable character #x0008: special characters are not allowed"},
		{"VT", 0x0B, "unacceptable character #x000b: special characters are not allowed"},
		{"FF", 0x0C, "unacceptable character #x000c: special characters are not allowed"},
		{"SO", 0x0E, "unacceptable character #x000e: special characters are not allowed"},
		{"US", 0x1F, "unacceptable character #x001f: special characters are not allowed"},
		{"DEL", 0x7F, "unacceptable character #x007f: special characters are not allowed"},
		{"C1 low", 0x84, "unacceptable character #x0084: special characters are not allowed"},
		{"C1 high", 0x86, "unacceptable character #x0086: special characters are not allowed"},
		{"APC", 0x9F, "unacceptable character #x009f: special characters are not allowed"},
		{"U+FFFE", 0xFFFE, "unacceptable character #xfffe: special characters are not allowed"},
		{"U+FFFF", 0xFFFF, "unacceptable character #xffff: special characters are not allowed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(document(tc.ch))

			var nonPrintable *yamlreader.NonPrintableError
			if !errors.As(err, &nonPrintable) {
				t.Fatalf("err = %v (%T), want *yamlreader.NonPrintableError", err, err)
			}
			if got := nonPrintable.Error(); got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
			if nonPrintable.Rune != tc.ch {
				t.Errorf("rune = %U, want %U", nonPrintable.Rune, tc.ch)
			}
		})
	}
}

// TestPrintableCharactersAreAccepted is the other half of spec delta 002-P
// §1.1, and the half that makes the check safe to have: a rule that refused
// NEL, a tab or an astral emoji would refuse documents upstream renders, which
// is a defect of the opposite sign.
//
// "Accepted" here means only that the reader raises no NonPrintableError. TAB,
// CR and LF each still fail this document on a later rule — the tab check and
// the parser respectively — exactly as they do upstream, so the assertion is
// about which rule fires and not about whether the document parses.
func TestPrintableCharactersAreAccepted(t *testing.T) {
	cases := []struct {
		name string
		ch   rune
	}{
		{"TAB", 0x09},
		{"LF", 0x0A},
		{"CR", 0x0D},
		{"space", 0x20},
		{"tilde", 0x7E},
		{"NEL", 0x85},
		{"NBSP", 0xA0},
		{"U+D7FF", 0xD7FF},
		{"U+E000", 0xE000},
		{"U+FFFD", 0xFFFD},
		{"emoji U+1F600", 0x1F600},
		{"U+10FFFF", 0x10FFFF},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(document(tc.ch))

			var nonPrintable *yamlreader.NonPrintableError
			if errors.As(err, &nonPrintable) {
				t.Fatalf("rune %U was rejected as %v", tc.ch, nonPrintable)
			}
		})
	}
}

// TestFirstNonPrintableCharacterWins pins the choice ruamel's two detection
// paths agree on (`reader.py:193-214`): the first offender in source order,
// whichever path found it. The second document forces the regex fallback by
// carrying a non-ASCII character, so both paths are exercised.
func TestFirstNonPrintableCharacterWins(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want rune
	}{
		{"ascii path", "cv:\n  name: \x08x\x01\n", 0x08},
		{"regex path", "cv:\n  name: é\x08x\x01\n", 0x08},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(tc.src)

			var nonPrintable *yamlreader.NonPrintableError
			if !errors.As(err, &nonPrintable) {
				t.Fatalf("err = %v (%T), want *yamlreader.NonPrintableError", err, err)
			}
			if nonPrintable.Rune != tc.want {
				t.Errorf("rune = %U, want %U", nonPrintable.Rune, tc.want)
			}
		})
	}
}

// TestReaderCheckPrecedesTheScanner is spec delta 002-P §1: ruamel checks the
// whole string in the `Reader.stream` setter, before a single token is scanned,
// so a document carrying both a forbidden character and a syntax error reports
// the forbidden character.
func TestReaderCheckPrecedesTheScanner(t *testing.T) {
	// A tab after a colon is the TabError of build.go, and `{` unclosed is a
	// parser failure. Neither may win over the reader's rule.
	for name, src := range map[string]string{
		"before the tab rule":   "cv:\tname: \x01A\n",
		"before a parse error":  "cv: {\x01\n",
		"before the empty rule": "\x01",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := yamlreader.ReadString(src)

			var nonPrintable *yamlreader.NonPrintableError
			if !errors.As(err, &nonPrintable) {
				t.Fatalf("err = %v (%T), want *yamlreader.NonPrintableError", err, err)
			}
		})
	}
}

// TestInvalidUTF8IsNotRejectedHere is spec delta 002-P §6: Python never reaches
// this check for a file that is not valid UTF-8, because `read_text` raises
// `UnicodeDecodeError` first — the unhandled-traceback class of D-011. The
// bytes decode to `U+FFFD` here, which the rule permits, so the port invents no
// message for them.
func TestInvalidUTF8IsNotRejectedHere(t *testing.T) {
	_, err := yamlreader.ReadString("cv:\n  name: \xffA\n")

	var nonPrintable *yamlreader.NonPrintableError
	if errors.As(err, &nonPrintable) {
		t.Fatalf("invalid UTF-8 was reported as %v", nonPrintable)
	}
}
