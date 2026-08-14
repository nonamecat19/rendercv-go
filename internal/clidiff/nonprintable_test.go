//go:build conformance

package clidiff

import (
	"fmt"
	"testing"
)

// noGeneration turns every artifact off.
//
// It is what makes a *success* row comparable byte for byte: the completed
// panel prints one duration per generated artifact, and a duration is the one
// part of the output that legitimately differs between the two binaries. With
// all five off, both sides print the same `Rendering...` panel and nothing
// else — so the accepted characters are held to full byte equality, exactly
// like the rejected ones, instead of to a weaker contract of their own.
var noGeneration = []string{
	"--dont-generate-typst", "--dont-generate-pdf", "--dont-generate-png",
	"--dont-generate-markdown", "--dont-generate-html",
}

// TestPrintableCharacterRule is spec delta 002-P: ruamel's reader refuses a
// character outside YAML's printable set before scanning starts
// (`ruamel/yaml/reader.py:187-189, 216-227`), and goccy refuses none of them.
//
// Every row is full byte equality of both streams, the exit code and the file
// tree. The accepted rows are as load-bearing as the rejected ones: a rule that
// refused NEL, a tab or an astral emoji would refuse documents upstream
// renders.
//
// `U+000D` is deliberately absent. Upstream reads the input through
// `pathlib.Path.read_text` (`cli/render_command/run_rendercv.py:115,140`),
// whose universal-newline translation turns a lone `\r` into `\n` before ruamel
// sees it, and the port reads bytes; the two therefore disagree on the *span*
// of a later parser error for that document. CR is permitted by both — the unit
// table in `internal/schema/yamlreader/printable_test.go` pins that — and the
// remaining disagreement is a newline-translation defect this rule does not
// touch (spec delta 002-P §6).
func TestPrintableCharacterRule(t *testing.T) {
	// document is spec delta 002-P §4's probe.
	document := func(ch rune) string { return fmt.Sprintf("cv:\n  name: %cA\n", ch) }

	cases := []struct {
		name string
		yaml string
	}{
		{"U+0000 NUL", document(0x00)},
		{"U+0001 SOH", document(0x01)},
		{"U+0008 BS", document(0x08)},
		{"U+000B VT", document(0x0B)},
		{"U+000C FF", document(0x0C)},
		{"U+000E SO", document(0x0E)},
		{"U+001F US", document(0x1F)},
		{"U+007F DEL", document(0x7F)},
		{"U+0084 C1 low", document(0x84)},
		{"U+0086 C1 high", document(0x86)},
		{"U+009F APC", document(0x9F)},
		{"U+FFFE", document(0xFFFE)},
		{"U+FFFF", document(0xFFFF)},

		{"U+0009 TAB", document(0x09)},
		{"U+000A LF", document(0x0A)},
		{"U+0085 NEL", document(0x85)},
		{"U+00A0 NBSP", document(0xA0)},
		{"U+D7FF", document(0xD7FF)},
		{"U+E000", document(0xE000)},
		{"U+FFFD", document(0xFFFD)},
		{"U+1F600 emoji", document(0x1F600)},
		{"U+10FFFF", document(0x10FFFF)},

		// The reader runs before the scanner and before the parser, so each of
		// these reports the forbidden character and not the other fault.
		{"before the tab rule", "cv:\tname: \x01A\n"},
		{"before a parse error", "cv: {\x01\n"},
		{"before the empty-file rule", "\x01"},

		// ruamel's ASCII fast path and its regex fallback both report the
		// first offender in source order; the second document forces the
		// fallback by carrying a non-ASCII character.
		{"first offender, ascii path", "cv:\n  name: \x08x\x01\n"},
		{"first offender, regex path", "cv:\n  name: é\x08x\x01\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream, port := differential(t, invocation{
				args:  append([]string{"render", "CV.yaml"}, noGeneration...),
				files: map[string]string{"CV.yaml": tc.yaml},
			})

			t.Logf("upstream: %s\nport:     %s", upstream, port)

			compare(t, upstream, port)
			compareRaw(t, "stdout", upstream.stdout, port.stdout)
			compareRaw(t, "stderr", upstream.stderr, port.stderr)
		})
	}
}
