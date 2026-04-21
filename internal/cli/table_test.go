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
