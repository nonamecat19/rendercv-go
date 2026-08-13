package cli

import (
	"strings"
	"testing"
)

// validationColumns is `print_validation_errors`'s table, minus the middle
// column, at a width small enough to pin on one screen
// (`cli/render_command/progress_panel.py:148-151`).
var validationColumns = []TableColumn{
	{Header: "Location", NoWrap: true, Style: StyleCyan},
	{Header: "Explanation", Style: StyleOrange4},
}

// The same table as the vendored Rich writes it, escape byte spelled out.
//
// **Measured, not derived.** Captured from
// `third_party/rendercv/.venv/bin/python` constructing the identical
// `rich.table.Table(expand=True, show_lines=True, box=ROUNDED)` at width 30
// with `force_terminal=True`, once per colour system — the memoization hazard of
// spec 012 delta §2.3 makes measuring two systems in one process report the
// first one's answer for both. The whole surface is gated end to end by
// `TestValidationTableColour` (`ptydiff_conformance_test.go`), which compares
// the real command's bytes against upstream's on a pty; this copy pins the run
// structure on a machine with no submodule checked out.
//
// Three rules are visible in it, and a plausible implementation gets each wrong:
//
//  1. **A cell is three runs** — left pad, padded content, right pad — each
//     opened and closed, because the cell is rendered as a `Padding` and Rich
//     merges nothing.
//  2. **A blank continuation cell is one run** spanning the whole column,
//     padding included: `Segment(" " * width, style)` (`rich/segment.py:489`).
//  3. **The box's own characters are unstyled.** Only the cells carry a colour;
//     the `bold red` belongs to the enclosing panel's border alone.
const upstreamValidationTable256 = "" +
	"╭──────────┬─────────────────╮\n" +
	"│ESC[1m ESC[0mESC[1mLocationESC[0mESC[1m ESC[0m│ESC[1m ESC[0mESC[1mExplanation    ESC[0mESC[1m ESC[0m│\n" +
	"├──────────┼─────────────────┤\n" +
	"│ESC[36m ESC[0mESC[36mlocale  ESC[0mESC[36m ESC[0m│ESC[38;5;94m ESC[0mESC[38;5;94mone two three  ESC[0mESC[38;5;94m ESC[0m│\n" +
	"│ESC[36m          ESC[0m│ESC[38;5;94m ESC[0mESC[38;5;94mfour           ESC[0mESC[38;5;94m ESC[0m│\n" +
	"╰──────────┴─────────────────╯"

// The same capture on an eight-colour terminal. `cyan` does not move and
// `orange4` is palette 94, which collapses to `ESC[33m` (delta §2.2) — so this
// is the expectation that catches a port hard-coding the palette form.
const upstreamValidationTableStandard = "" +
	"╭──────────┬─────────────────╮\n" +
	"│ESC[1m ESC[0mESC[1mLocationESC[0mESC[1m ESC[0m│ESC[1m ESC[0mESC[1mExplanation    ESC[0mESC[1m ESC[0m│\n" +
	"├──────────┼─────────────────┤\n" +
	"│ESC[36m ESC[0mESC[36mlocale  ESC[0mESC[36m ESC[0m│ESC[33m ESC[0mESC[33mone two three  ESC[0mESC[33m ESC[0m│\n" +
	"│ESC[36m          ESC[0m│ESC[33m ESC[0mESC[33mfour           ESC[0mESC[33m ESC[0m│\n" +
	"╰──────────┴─────────────────╯"

// renderStyledTable is the table as one string, the way the validation panel
// puts it on the screen: a line of segments at a time.
func renderStyledTable(rows [][]string, width int, terminal Terminal) string {
	lines := make([]string, 0, len(rows)+5)
	for _, line := range StyledTable(validationColumns, rows, width) {
		lines = append(lines, line.RenderSegments(terminal))
	}
	return strings.Join(lines, "\n")
}

// TestStyledTableMatchesUpstreamsBytes pins the run structure of the validation
// table against the two captures above.
func TestStyledTableMatchesUpstreamsBytes(t *testing.T) {
	rows := [][]string{{"locale", "one two three four"}}

	for _, test := range []struct {
		name string
		term Terminal
		want string
	}{
		{"truecolor", Terminal{IsTerminal: true, System: ColorTruecolor}, upstreamValidationTable256},
		{"256 colour", Terminal{IsTerminal: true, System: ColorEightBit}, upstreamValidationTable256},
		{"standard", Terminal{IsTerminal: true, System: ColorStandard}, upstreamValidationTableStandard},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := spellEscapes(renderStyledTable(rows, 30, test.term))
			if got != test.want {
				t.Errorf("StyledTable() =\n  %q\nwant\n  %q", got, test.want)
			}
		})
	}
}

// TestStyledTableHeaderCarriesNoColumnStyle is the rule most easily got wrong by
// a port that applies the column's style to the whole column: Rich renders a
// header cell under `header_style` alone (`rich/table.py:671-674`), which is
// `table.header`, which is `bold`. Measured — `Location`'s header is `ESC[1m`
// and never `ESC[1;36m`.
func TestStyledTableHeaderCarriesNoColumnStyle(t *testing.T) {
	lines := StyledTable(validationColumns, [][]string{{"locale", "why"}}, 30)
	header := lines[1].RenderSegments(Terminal{IsTerminal: true, System: ColorEightBit})

	if strings.Contains(header, "\x1b[36m") || strings.Contains(header, "\x1b[38;5;94m") {
		t.Errorf("the header carries a column colour: %q", spellEscapes(header))
	}
	if !strings.Contains(header, "\x1b[1m") {
		t.Errorf("the header is not bold: %q", spellEscapes(header))
	}
}

// TestStyledTableIsPlainWithoutColour is the property the 42 goldens depend on:
// in the environment they are captured in — a pipe — the styled table is
// byte-identical to `Table`, which is what every table test in this package
// asserts.
func TestStyledTableIsPlainWithoutColour(t *testing.T) {
	rows := [][]string{{"locale", "one two three four"}}
	want := strings.TrimRight(Table(validationColumns, rows, 30), "\n")

	for _, test := range []struct {
		name string
		term Terminal
	}{
		{"a pipe", Terminal{}},
		{"a dumb terminal", Terminal{IsTerminal: true, System: ColorNone}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := renderStyledTable(rows, 30, test.term); got != want {
				t.Errorf("StyledTable() =\n  %q\nwant\n  %q", spellEscapes(got), want)
			}
		})
	}
}

// TestStyledTableKeepsBoldUnderNoColor is `NO_COLOR`'s rule on this surface
// (delta §3.3): it calls `Segment.remove_color`, which drops the colour and
// keeps every other attribute — so the header's bold survives and the cells
// lose their colour entirely, runs and all.
func TestStyledTableKeepsBoldUnderNoColor(t *testing.T) {
	terminal := Terminal{IsTerminal: true, System: ColorEightBit, NoColor: true}
	rendered := renderStyledTable([][]string{{"locale", "why"}}, 30, terminal)

	if !strings.Contains(rendered, "\x1b[1m") {
		t.Errorf("the header lost its bold under NO_COLOR: %q", spellEscapes(rendered))
	}
	if strings.Contains(rendered, "\x1b[36m") || strings.Contains(rendered, "\x1b[38;5;94m") {
		t.Errorf("a cell kept its colour under NO_COLOR: %q", spellEscapes(rendered))
	}
}
