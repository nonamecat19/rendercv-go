package cli

import "testing"

// TestPanelExpandsTabs pins Rich's tab handling, which is **expansion to the
// next eight-column stop**, not a raw `\t` written through to the terminal
// (`rich/text.py:817-857`, reached from `Text.wrap` at `rich/text.py:1231`).
//
// The port emitted the tab byte, so the row was as wide as the reader's
// terminal chose to make it and the panel's right border landed wherever that
// left it. `new` reaches this: the file name keeps every character of the name
// that is not a space (`cli/new_command/new_command.py:81`), so
// `rendercv new "$(printf 'a\tb')"` puts a tab inside the box.
//
// **The stops are counted in cells from the start of the row**, not in
// characters — the "tab after wide" case is the one that separates the two —
// and expansion happens *before* wrapping, so it moves the fold as well.
//
// Every `want` is the exact stdout of the vendored
// `rich.panel.Panel(body, title="Get started", title_align="left",
// border_style="bright_black")` at width 80.
func TestPanelExpandsTabs(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "tab",
			body: "A\tB",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ A       B                                                                    │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "tab stops",
			body: "a\tbb\tccc\tdddd\te",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ a       bb      ccc     dddd    e                                            │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "tab after wide",
			body: "李\tB",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ 李      B                                                                    │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "leading tab",
			body: "\tindented",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│         indented                                                             │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "tab at a stop",
			body: "12345678\tx",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ 12345678        x                                                            │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "two tabs in a row",
			body: "a\t\tb",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ a               b                                                            │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
		{
			name: "tab in a wrapped row",
			body: "start\tword word word word word word word word word word word word word word word word word word word word ",
			want: "╭─ Get started ────────────────────────────────────────────────────────────────╮\n" +
				"│ start   word word word word word word word word word word word word word     │\n" +
				"│ word word word word word word word                                           │\n" +
				"╰──────────────────────────────────────────────────────────────────────────────╯\n",
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			t.Setenv("COLUMNS", "80")
			if got := Panel("Get started", []PanelRow{{Text: row.body}}); got != row.want {
				t.Errorf("panel =\n%q\nwant\n%q", got, row.want)
			}
		})
	}
}

// TestTableExpandsTabs is the same rule one layer down: `Text.wrap` expands
// tabs before it decides whether to wrap at all, so a no-wrap column gets the
// spaces too (`rich/text.py:1231-1233`, above the `no_wrap` branch).
//
// Captured from the vendored `rich.table.Table(expand=True, show_lines=True,
// box=rich.box.ROUNDED)` with `print_validation_errors`'s three columns
// (`progress_panel.py:148-151`), printed at width 80.
func TestTableExpandsTabs(t *testing.T) {
	columns := []TableColumn{
		{Header: "Location", NoWrap: true},
		{Header: "Input Value", NoWrap: true},
		{Header: "Explanation"},
	}

	cases := []struct {
		name string
		row  []string
		want string
	}{
		{
			name: "tab in a wrappable cell",
			row:  []string{"cv.name", "value", "a\tb explanation"},
			want: "╭────────────────────┬──────────────────────────┬──────────────────────────────╮\n" +
				"│ Location           │ Input Value              │ Explanation                  │\n" +
				"├────────────────────┼──────────────────────────┼──────────────────────────────┤\n" +
				"│ cv.name            │ value                    │ a       b explanation        │\n" +
				"╰────────────────────┴──────────────────────────┴──────────────────────────────╯\n",
		},
		{
			name: "tab in a no-wrap cell",
			row:  []string{"cv.name", "a\tb", "explanation"},
			want: "╭──────────────────────┬───────────────────────────┬───────────────────────────╮\n" +
				"│ Location             │ Input Value               │ Explanation               │\n" +
				"├──────────────────────┼───────────────────────────┼───────────────────────────┤\n" +
				"│ cv.name              │ a       b                 │ explanation               │\n" +
				"╰──────────────────────┴───────────────────────────┴───────────────────────────╯\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Table(columns, [][]string{c.row}, 80); got != c.want {
				t.Errorf("table =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}
