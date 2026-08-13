package cli

import (
	"strings"
	"testing"
)

// The progress panel exactly as the vendored Python writes it on a pty at
// `COLUMNS=80`, escape byte spelled out.
//
// **Measured, not derived.** Captured by driving
// `third_party/rendercv/.venv/bin/rendercv render CV.yaml -nopdf -nopng -nomd
// -nohtml` through a pty with `TERM=xterm-256color COLORTERM=truecolor`, and
// gated end to end by `TestProgressPanelColour`
// (`ptydiff_conformance_test.go`), which compares the same two captures. This
// copy exists so the shape is pinned on a machine with no submodule checked out.
//
// Four things it pins that a plausible implementation gets wrong, each visible
// only in the bytes:
//
//  1. the title band is **one** run, `ESC[90m Your CV is ready ESC[0m`, where
//     the `Error` panel's is three — that panel's title carries markup of its
//     own and this one does not;
//  2. `bold green` covers the timing's padding out to eight columns, and stops
//     before the space that separates it from the message;
//  3. the message and its padding are unstyled, between two styled fields;
//  4. the padding that follows the `purple` path is **outside** the run, and so
//     is the panel's own padding space either side of the body.
const upstreamProgressPanel = "ESC[90m╭─ESC[0mESC[90m Your CV is ready ESC[0m" +
	"ESC[90m──────────────────────────────────────────────────────────ESC[0mESC[90m─╮ESC[0m\n" +
	"ESC[90m│ESC[0m ESC[32m✓ESC[0m ESC[1;32m11 ms   ESC[0m Generated Typst:           " +
	"ESC[38;5;129m./rendercv_output/John_Doe_CV.typESC[0m      ESC[90m│ESC[0m\n" +
	"ESC[90m╰──────────────────────────────────────────────────────────────────────────────╯ESC[0m\n"

// progressRow is the row the capture above was taken from.
func progressRow() []PanelRow {
	return []PanelRow{{
		Mark:   "✓",
		Timing: "11 ms",
		Label:  "Generated Typst:",
		Value:  "./rendercv_output/John_Doe_CV.typ",
	}}
}

// TestProgressPanelMatchesUpstreamsBytes is spec 012 delta §8's unit C on a
// terminal that has the whole palette.
func TestProgressPanelMatchesUpstreamsBytes(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	for _, terminal := range []struct {
		name string
		term Terminal
	}{
		// `purple` is palette 129 on both, and every other style on this panel
		// is standard or bold-only — so the two systems agree here and part
		// company only at `ColorStandard`, which the row below pins.
		{"truecolor", Terminal{IsTerminal: true, System: ColorTruecolor}},
		{"256 colour", Terminal{IsTerminal: true, System: ColorEightBit}},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			got := spellEscapes(StyledPanel(PlainText("Your CV is ready"), progressRow(),
				StyleBrightBlack, terminal.term))
			if got != upstreamProgressPanel {
				t.Errorf("StyledPanel() =\n  %q\nwant\n  %q", got, upstreamProgressPanel)
			}
		})
	}
}

// TestProgressPanelDowngradesPurple pins the one style on this panel that moves
// with the colour system: `purple` is palette entry 129 and collapses to the
// standard `magenta` on a sixteen-colour terminal (delta §2.2).
//
// Measured through the vendored Python at `TERM=xterm` with `COLORTERM` unset,
// where the path comes out `ESC[35m…ESC[0m` and every other run on the line is
// unchanged.
func TestProgressPanelDowngradesPurple(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	terminal := Terminal{IsTerminal: true, System: ColorStandard}
	got := spellEscapes(StyledPanel(PlainText("Your CV is ready"), progressRow(),
		StyleBrightBlack, terminal))

	want := strings.ReplaceAll(upstreamProgressPanel, "ESC[38;5;129m", "ESC[35m")
	if got != want {
		t.Errorf("StyledPanel() =\n  %q\nwant\n  %q", got, want)
	}
}

// TestProgressPanelKeepsBoldUnderNoColor is delta §3.3 on this surface:
// `NO_COLOR` calls `Segment.remove_color`, which drops the colour and keeps
// every other attribute. So the `green` tick and the `purple` path lose their
// runs entirely — a style with nothing left to say opens no sequence — while
// `bold green` stays as a bare `ESC[1m`.
//
// Measured: upstream writes `│ ✓ ESC[1m15 ms   ESC[0m Generated Typst: …` with
// no other sequence on the line, borders included.
func TestProgressPanelKeepsBoldUnderNoColor(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	terminal := Terminal{IsTerminal: true, System: ColorEightBit, NoColor: true}
	got := spellEscapes(StyledPanel(PlainText("Your CV is ready"), progressRow(),
		StyleBrightBlack, terminal))

	want := "╭─ Your CV is ready ───────────────────────────────────────────────────────────╮\n" +
		"│ ✓ ESC[1m11 ms   ESC[0m Generated Typst:           ./rendercv_output/John_Doe_CV.typ      │\n" +
		"╰──────────────────────────────────────────────────────────────────────────────╯\n"
	if got != want {
		t.Errorf("StyledPanel() =\n  %q\nwant\n  %q", got, want)
	}
}

// TestProgressPanelIsPlainWithoutColour is the property the 42 goldens depend
// on: in the environment they are captured in the styled progress panel is
// byte-identical to the unstyled one, so attaching the styles moves no golden.
func TestProgressPanelIsPlainWithoutColour(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	plainPanel := Panel("Your CV is ready", progressRow())
	for _, terminal := range []struct {
		name string
		term Terminal
	}{
		{name: "a pipe", term: Terminal{}},
		{name: "a dumb terminal", term: Terminal{IsTerminal: true, System: ColorNone}},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			got := StyledPanel(PlainText("Your CV is ready"), progressRow(), StyleBrightBlack, terminal.term)
			if got != plainPanel {
				t.Errorf("StyledPanel() =\n  %q\nplain is\n  %q", spellEscapes(got), plainPanel)
			}
		})
	}
}

// TestProgressPanelWrapsAValueWithItsStyle pins delta §7.1.3 on the row that
// actually wraps: a two-page render's `Generated PNGs:` line, whose paths do not
// fit on one line.
//
// **The span follows the break.** Measured, upstream ends the first line inside
// the `purple` run — the trailing space of `"; "` is still purple — and opens a
// fresh `purple` run for the continuation, which starts at the panel's first
// column with no indent.
func TestProgressPanelWrapsAValueWithItsStyle(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	rows := []PanelRow{{
		Mark:   "✓",
		Timing: "18 ms",
		Label:  "Generated PNGs:",
		Value:  "./rendercv_output/JohnDoe_CV_1.png; ./rendercv_output/JohnDoe_CV_2.png",
	}}
	got := spellEscapes(StyledPanel(PlainText("Your CV is ready"), rows, StyleBrightBlack,
		Terminal{IsTerminal: true, System: ColorTruecolor}))

	want := "ESC[90m│ESC[0m ESC[32m✓ESC[0m ESC[1;32m18 ms   ESC[0m Generated PNGs:            " +
		"ESC[38;5;129m./rendercv_output/JohnDoe_CV_1.png; ESC[0m   ESC[90m│ESC[0m\n" +
		"ESC[90m│ESC[0m ESC[38;5;129m./rendercv_output/JohnDoe_CV_2.pngESC[0m" +
		"                                           ESC[90m│ESC[0m\n"
	if !strings.Contains(got, want) {
		t.Errorf("StyledPanel() =\n  %q\nwant it to contain\n  %q", got, want)
	}
}
