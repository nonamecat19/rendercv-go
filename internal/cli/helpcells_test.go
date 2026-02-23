package cli

import (
	"slices"
	"strings"
	"testing"
)

// Rich measures every layout in display cells (`rich/cells.py`), which the port
// reproduces in `cellLen`. The help layout was the last place still counting
// runes — and `commandsPanel` the last counting *bytes* — so a wide character
// reserved too little room and a multi-byte one too much.
//
// **The reachability argument for leaving them was wrong in its premise.** The
// embedded capture is not ASCII: `new --help`'s locale list carries `å`
// (U+00E5) today, because upstream's locale names reach that string. What is
// true is weaker and accidental — measured with the vendored rich itself, all
// 205 strings in `helpdata/help.json` have `cell_len == len`, so nothing
// mismeasures *yet*. One locale named in kana would end that, and silently.
//
// Each case pairs a string whose cell width differs from its rune or byte count
// with an ASCII string of the same cell width. Measured in cells the two must
// lay out identically; that is the assertion, and it needs no knowledge of the
// padding rules to state.

// TestHelpColumnsMeasuresCells pins `columns.go`'s measurement. `日本` is two
// runes and four cells, so under a rune count its grid column is two cells wide
// and the shorter item below it is padded to the wrong width.
func TestHelpColumnsMeasuresCells(t *testing.T) {
	const width = 20
	wide := HelpColumns([]string{"日本", "x", "ab", "y"}, width)
	ascii := HelpColumns([]string{"abcd", "x", "ab", "y"}, width)

	if len(wide) != len(ascii) {
		t.Fatalf("wide laid out in %d lines, the same-width ASCII in %d:\n%q\nvs\n%q",
			len(wide), len(ascii), wide, ascii)
	}
	for i := range wide {
		if cellLen(wide[i]) != cellLen(ascii[i]) {
			t.Errorf("line %d measures %d cells, the same-width ASCII line %d\n  %q\n  %q",
				i, cellLen(wide[i]), cellLen(ascii[i]), wide[i], ascii[i])
		}
	}
}

// TestHelpTableWidthsMeasureCells pins `helptable.go`'s measurement of a
// natural-width column.
func TestHelpTableWidthsMeasureCells(t *testing.T) {
	columns := []helpColumn{{}, {Flexible: true}}
	widths := func(name string) []int {
		return helpTableWidths(columns, [][]helpCell{{plain(name), plain("help")}}, 80)
	}

	wide := widths("日本")
	ascii := widths("abcd")
	if !slices.Equal(wide, ascii) {
		t.Errorf("widths for a 4-cell wide name = %v, for 4-cell ASCII = %v", wide, ascii)
	}
}

// TestCommandsPanelMeasuresCells pins the third site, which counted *bytes*:
// `日本` is six bytes, four cells, so its column was reserved two cells too
// wide even before the rune question arose.
// **Comparing whole-line widths here would prove nothing**: the panel pads
// every line out to the console width, so both panels measure the same however
// wrong the column is. The observable is where the help text *starts* — the
// column the name reserved — so that is what this measures.
func TestCommandsPanelMeasuresCells(t *testing.T) {
	const help = "does a thing"

	helpOffset := func(name string) int {
		t.Helper()
		panel := commandsPanel([]helpSubcommand{{Name: name, Help: help}})
		for _, line := range splitLines(panel) {
			if before, _, found := strings.Cut(line, help); found {
				return cellLen(before)
			}
		}
		t.Fatalf("no line of the panel carries the help text:\n%s", panel)
		return 0
	}

	wide := helpOffset("日本")
	ascii := helpOffset("abcd")
	if wide != ascii {
		t.Errorf("help starts at cell %d after a 4-cell wide name, at %d after 4-cell ASCII",
			wide, ascii)
	}
}
