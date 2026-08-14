package cli_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// The three columns `print_validation_errors` builds
// (`progress_panel.py:148-151`).
var errorColumns = []cli.TableColumn{
	{Header: "Location", NoWrap: true},
	{Header: "Input Value", NoWrap: true},
	{Header: "Explanation"},
}

// Each case below is a golden's table, and the expectation is the golden's own
// first line. The three stages of Rich's width algorithm are one case each:
// distribute, collapse, and the even reduction that drives a column to zero.
func TestTableMatchesTheGoldenGeometry(t *testing.T) {
	// The path `err_unknown_theme` actually reports, now that the corpus runs at
	// internal/conformance/workroot.Root rather than under whichever checkout
	// generated the goldens.
	longPath := "`/tmp/rendercv-go-conformance/run/err_unknown_theme/nosuchtheme` does not exist." +
		" It should be in the same directory as the input file."

	cases := []struct {
		name string
		rows [][]string
		want string
	}{
		{
			// render_override_scalar: the table is narrower than the space it
			// has, so the leftover is distributed.
			name: "distribute",
			rows: [][]string{{"cv.phone", "+1-555-555-5555", "This is not a valid phone number."}},
			want: "╭────────────┬────────────────────┬────────────────────────────────────────╮",
		},
		{
			// err_unknown_theme: one wrappable column is collapsed to make room.
			name: "collapse",
			rows: [][]string{{"design", "...", "The custom theme folder " + longPath}},
			want: "╭──────────┬─────────────┬─────────────────────────────────────────────────╮",
		},
		{
			// err_wrong_input: collapsing the wrappable column to zero is still
			// not enough, so every column is reduced evenly.
			name: "reduce evenly",
			// Both no-wrap columns exceed what is left once the wrappable one
			// is gone: 40 and 30 characters of content, which is the pair the
			// golden's 41 / 31 / 0 solves to.
			rows: [][]string{
				{"cv.sections.welcome_to_rendercv_tests_10", "I'm the author! But should thi", "x"},
				{"design.theme", "not_a_valid_theme", "y"},
			},
			want: "╭─────────────────────────────────────────┬───────────────────────────────┬╮",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 76 is the outer panel's inner width, which is what the table is
			// given (`PanelWidth` minus the two borders and their padding).
			got, _, _ := strings.Cut(cli.Table(errorColumns, c.rows, cli.PanelWidth-4), "\n")
			if got != c.want {
				t.Errorf("top border:\ngot  %s\nwant %s", got, c.want)
			}
		})
	}
}

// Every line of a table must be exactly the width it was given, whatever the
// content did — a row that is one column short breaks the panel around it.
func TestTableLinesAreAllTheSameWidth(t *testing.T) {
	rows := [][]string{
		{"main_yaml_file: line 1 to line 2", "...", "This is not a valid YAML file. while parsing a flow sequence."},
		{"cv.name", "…", ""},
	}

	table := cli.Table(errorColumns, rows, cli.PanelWidth-4)
	for i, line := range strings.Split(strings.TrimRight(table, "\n"), "\n") {
		if width := len([]rune(line)); width != cli.PanelWidth-4 {
			t.Errorf("line %d is %d wide, want %d: %q", i+1, width, cli.PanelWidth-4, line)
		}
	}
}

// Rich sanitizes every string it renders through `Text.__init__`
// (`rich/text.py:156` → `rich/control.py:181`), which deletes exactly five
// codepoints and leaves every other control character alone. Verified against
// upstream with `design.page.size` probes: `"a\a4"`, `"a\b4"`, `"a\v4"`,
// `"a\f4"` and `"a\r4"` all print `a4`, while `"a\x014"`, `"a\e4"` and
// `"a\x7f4"` print the byte raw.
func TestTableStripsTheControlCodesRichStrips(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"bell", "a\a4", "a4"},
		{"backspace", "a\b4", "a4"},
		{"vertical tab", "a\v4", "a4"},
		{"form feed", "a\f4", "a4"},
		{"carriage return", "a\r4", "a4"},
		{"all five at once", "\a\b\va\f4\r", "a4"},
		// Not in Rich's STRIP_CONTROL_CODES, so upstream emits them as-is.
		{"start of heading", "a\x014", "a\x014"},
		{"escape", "a\x1b4", "a\x1b4"},
		{"delete", "a\x7f4", "a\x7f4"},
		// **A tab survives the stripping and is then expanded**, which this
		// row used to get half right: `Text.wrap` turns it into spaces up to
		// the next eight-column stop (`rich/text.py:1231`) long after
		// `strip_control_codes` has let it through. Re-measured with the same
		// `design.page.size` probe as the rest — upstream's cell is
		// `a       4`, not `a\t4`.
		{"tab", "a\t4", "a       4"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := []string{"design.page.size", c.value, "Input should be 'a4', 'a5', 'us-letter'"}
			table := cli.Table(errorColumns, [][]string{row}, cli.PanelWidth-4)
			if !strings.Contains(table, "│ "+c.want+" ") {
				t.Errorf("cell %q did not render as %q:\n%s", c.value, c.want, table)
			}
		})
	}
}

// A stripped character must not reserve a column either: upstream's `a\a4` row
// is exactly as wide as every other, because `Text` sanitizes before Rich
// measures anything.
func TestTableMeasuresCellsAfterStripping(t *testing.T) {
	explanation := "Input should be 'a4', 'a5', 'us-letter'"
	stripped := cli.Table(errorColumns, [][]string{{"design.page.size", "a\a4", explanation}}, cli.PanelWidth-4)
	plain := cli.Table(errorColumns, [][]string{{"design.page.size", "a4", explanation}}, cli.PanelWidth-4)

	if stripped != plain {
		t.Errorf("a bell changed the table geometry:\ngot\n%s\nwant\n%s", stripped, plain)
	}
}

// A cell too long for a no-wrap column is cut with `…`, not folded — Rich's
// default Column overflow, and what every long location in err_wrong_input shows.
func TestNoWrapCellsAreTruncatedWithAnEllipsis(t *testing.T) {
	long := strings.Repeat("a", 200)
	table := cli.Table(errorColumns, [][]string{{long, "x", "y"}}, cli.PanelWidth-4)

	if !strings.Contains(table, "a…") {
		t.Errorf("a long no-wrap cell was not ellipsized:\n%s", table)
	}
	if strings.Count(table, "\n") != 5 {
		t.Errorf("table has %d lines, want 5 — the cell folded instead of truncating:\n%s",
			strings.Count(table, "\n"), table)
	}
}

// A newline inside a no-wrap cell starts a new line, and `no_wrap` does not
// change that.
//
// `Text.wrap` iterates `self.split(allow_blank=True)` and only *then* branches
// on the flag, replacing the fold with `Lines([line])`
// (`rich/text.py:1230-1237`). So `no_wrap` suppresses folding one long line; it
// never joins two source lines. `allow_blank=True` keeps the empty line a
// trailing separator produces (`rich/text.py:1101-1102`), which is why a value
// ending in a newline is one line taller than its content.
//
// The port collapsed the cell to a single line instead, which was two bugs at
// once: a long multi-line value lost everything after the first line, and a
// *short* one escaped the truncation altogether and emitted its newline raw
// into the frame, tearing the row open. Found by a fuzzer on a `highlights:`
// block scalar.
//
// Every want below was measured against the vendored Python at COLUMNS=100 with
// `highlights:` given the quoted scalar named, reading the cell verbatim.
func TestNoWrapCellsSplitOnNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "no newline", input: "AAA", want: []string{"AAA"}},
		{name: "empty", input: "", want: []string{""}},
		{name: "one newline", input: "AAA\nBBB", want: []string{"AAA", "BBB"}},
		{
			name:  "several newlines",
			input: "AAA\nBBB\nCCC\nDDD",
			want:  []string{"AAA", "BBB", "CCC", "DDD"},
		},
		// allow_blank=True: the trailing separator keeps its empty line.
		{name: "trailing newline", input: "AAA\n", want: []string{"AAA", ""}},
		{
			name:  "trailing newline after several",
			input: "AAA\nBBB\nCCC\n",
			want:  []string{"AAA", "BBB", "CCC", ""},
		},
		{name: "leading newline", input: "\nAAA\nBBB", want: []string{"", "AAA", "BBB"}},
		{name: "only a newline", input: "\n", want: []string{"", ""}},
		{name: "blank line between", input: "AAA\n\nBBB", want: []string{"AAA", "", "BBB"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := noWrapCellLines(t, tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("cell = %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("cell = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// A long line inside a multi-line no-wrap cell is truncated on its own, not
// merged with its neighbours — the two rules meet on the case the fuzzer found,
// where the first line fits and the second does not.
func TestNoWrapCellsTruncateEachLineSeparately(t *testing.T) {
	long := strings.Repeat("b", 200)
	got := noWrapCellLines(t, "AAA\n"+long)
	if len(got) != 2 {
		t.Fatalf("cell = %q, want two lines", got)
	}
	if got[0] != "AAA" {
		t.Errorf("first line = %q, want %q — a short line must not be cut", got[0], "AAA")
	}
	if !strings.HasSuffix(got[1], ellipsisRune) || !strings.HasPrefix(got[1], "bb") {
		t.Errorf("second line = %q, want the long line truncated with an ellipsis", got[1])
	}
}

// ellipsisRune is what Rich appends when it truncates a cell.
const ellipsisRune = "…"

// noWrapCellLines renders one row and returns the Input Value column's lines,
// trimmed of the padding the frame adds.
func noWrapCellLines(t *testing.T, input string) []string {
	t.Helper()
	table := cli.Table(errorColumns, [][]string{{"a.b", input, "x"}}, cli.PanelWidth-4)

	all := strings.Split(strings.TrimSuffix(table, "\n"), "\n")
	// A table is a top border, a header, a header rule, the body, and a bottom
	// border; only the body carries cells.
	if len(all) < 5 {
		t.Fatalf("table has too few lines to hold a body:\n%s", table)
	}
	body := all[3 : len(all)-1]

	lines := make([]string, 0, len(body))
	for _, line := range body {
		cells := strings.Split(line, "│")
		if len(cells) < 3 {
			t.Fatalf("body line %q has no second column", line)
		}
		lines = append(lines, strings.TrimSpace(cells[2]))
	}
	return lines
}
