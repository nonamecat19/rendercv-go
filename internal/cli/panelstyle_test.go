package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// sgrPattern matches the only escape form these panels emit, which is what
// makes "the same geometry with the colour taken back out" a thing this file
// can assert.
var sgrPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// spellEscapes writes the escape byte as `ESC` so a failure is legible. It is
// `readable` from `style_test.go`, which lives in the external test package and
// so cannot be called from this one.
func spellEscapes(text string) string {
	return strings.ReplaceAll(text, "\x1b", "ESC")
}

// The `Error` panel exactly as the vendored Python writes it on a pty at
// `COLUMNS=80`, escape byte spelled out.
//
// **Measured, not derived.** Captured by driving
// `third_party/rendercv/.venv/bin/rendercv render CV.yaml` over an empty
// document through a pty with `TERM=xterm-256color COLORTERM=truecolor`, and
// gated end to end by `TestErrorPanelColour` (`ptydiff_conformance_test.go`),
// which compares the same two captures byte for byte. This copy exists so the
// shape is pinned on a machine with no submodule checked out.
const upstreamErrorPanel = "ESC[1;31m╭─ESC[0mESC[1;31m ESC[0mESC[1;31mErrorESC[0mESC[1;31m ESC[0m" +
	"ESC[1;31m─────────────────────────────────────────────────────────────────────ESC[0mESC[1;31m─╮ESC[0m\n" +
	"ESC[1;31m│ESC[0m The input file is empty!                                                     ESC[1;31m│ESC[0m\n" +
	"ESC[1;31m╰──────────────────────────────────────────────────────────────────────────────╯ESC[0m\n"

// TestStyledPanelMatchesUpstreamsBytes pins spec 012 delta §2.4's run structure
// on the `Error` panel: six runs on the top line where a single string would
// look identical to a reader, one unstyled run of padding between the two
// border characters of the body, and the whole closing border as one run.
func TestStyledPanelMatchesUpstreamsBytes(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	rows := []PanelRow{{Text: "The input file is empty!"}}
	for _, terminal := range []struct {
		name string
		term Terminal
	}{
		// `bold red` carries no palette colour, so every colour system writes
		// it the same way — which is what makes the three rows one expectation.
		{"truecolor", Terminal{IsTerminal: true, System: ColorTruecolor}},
		{"256 colour", Terminal{IsTerminal: true, System: ColorEightBit}},
		{"standard", Terminal{IsTerminal: true, System: ColorStandard}},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			got := spellEscapes(StyledPanel("Error", rows, StyleBoldRed, terminal.term))
			if got != upstreamErrorPanel {
				t.Errorf("StyledPanel() =\n  %q\nwant\n  %q", got, upstreamErrorPanel)
			}
		})
	}
}

// TestStyledPanelIsPlainWithoutColour is the property every one of the 42
// goldens depends on: in the environment they are captured in — a pipe, and
// `NO_COLOR=1 TERM=dumb` besides — the styled panel is byte-identical to the
// unstyled one.
//
// `NO_COLOR` is in the table for the opposite reason: it is the one switch that
// leaves a sequence behind on a real terminal, so it must *not* pass by
// accident, and the row below asserts that it does not reach this rule.
func TestStyledPanelIsPlainWithoutColour(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	rows := []PanelRow{{Text: "The input file is empty!"}}
	plainPanel := Panel("Error", rows)

	for _, terminal := range []struct {
		name string
		term Terminal
		want bool
	}{
		{name: "a pipe", term: Terminal{}, want: true},
		{name: "a dumb terminal", term: Terminal{IsTerminal: true, System: ColorNone}, want: true},
		{
			name: "NO_COLOR on a real terminal keeps the bold",
			term: Terminal{IsTerminal: true, System: ColorEightBit, NoColor: true},
			want: false,
		},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			got := StyledPanel("Error", rows, StyleBoldRed, terminal.term)
			if (got == plainPanel) != terminal.want {
				t.Errorf("StyledPanel() =\n  %q\nplain is\n  %q", spellEscapes(got), plainPanel)
			}
		})
	}
}

// TestStyledPanelCropsTheTitleWithItsStyle pins delta §7.1.2: the title is
// cropped **before** it is styled, so a narrow console never cuts an escape
// sequence in half and the styled panel breaks in exactly the column the plain
// one does.
func TestStyledPanelCropsTheTitleWithItsStyle(t *testing.T) {
	tests := []struct {
		columns int
		want    string
	}{
		// Room for ` Erro` and no more: the trailing pad and the fill are gone,
		// and what is left is still opened and closed.
		{columns: 9, want: "ESC[1;31m╭─ESC[0mESC[1;31m ESC[0mESC[1;31mErroESC[0mESC[1;31m─╮ESC[0m"},
		// One column short of the whole title.
		{columns: 10, want: "ESC[1;31m╭─ESC[0mESC[1;31m ESC[0mESC[1;31mErrorESC[0mESC[1;31m─╮ESC[0m"},
	}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.columns), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(test.columns))

			terminal := Terminal{IsTerminal: true, System: ColorTruecolor}
			got := StyledPanel("Error", nil, StyleBoldRed, terminal)
			top, _, _ := strings.Cut(got, "\n")
			if spelled := spellEscapes(top); spelled != test.want {
				t.Errorf("top border = %q, want %q", spelled, test.want)
			}

			// The plain panel is what the crop has to agree with once the
			// sequences are removed: colour may not move the geometry by a
			// column.
			plainTop, _, _ := strings.Cut(Panel("Error", nil), "\n")
			if stripped := sgrPattern.ReplaceAllString(top, ""); stripped != plainTop {
				t.Errorf("the styled crop lands at %q, the plain one at %q", stripped, plainTop)
			}
		})
	}
}
