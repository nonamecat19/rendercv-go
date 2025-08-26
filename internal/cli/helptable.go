package cli

import (
	"strings"
	"unicode/utf8"
)

// helpColumn is one column of a help panel's table.
//
// **Every column of an `Options` or `Arguments` panel is a default one**:
// `_print_options_panel` adds no columns at all, so they come into being from
// the first `add_row` with no width, no ratio and `no_wrap` false
// (`typer/rich_utils.py:440-448`). Only the `Commands` panel declares its two
// (`:487-496`).
type helpColumn struct {
	// Flexible marks the help column. Its cell is a `rich.columns.Columns`,
	// which measures `(1, max_width)` rather than the length of its text — so
	// its natural width is the whole console, every panel's natural sum
	// overflows, and `expand=True` never expands anything. See
	// `specs/012-cli/help.md` §3.3.
	Flexible bool
	// Width is a declared fixed width, which only the `Commands` panel's first
	// column has. Zero means measure the cells.
	Width int
	// NoWrap is `no_wrap=True`, likewise the `Commands` panel's first column.
	NoWrap bool
}

// helpCell is one cell. Plain columns hold a single item; the flexible column
// holds the `Columns` items, which may share a line or stack.
type helpCell struct {
	items []string
}

func plain(text string) helpCell { return helpCell{items: []string{text}} }

// helpPadding is the padding of a column at index i in a table of n, under
// `padding=(0, 1)` with `pad_edge=False` (`typer/rich_utils.py:50-51`): one
// cell on each inner side, and none against either edge of the panel.
func helpPadding(i, n int) int {
	width := 2
	if i == 0 {
		width--
	}
	if i == n-1 {
		width--
	}
	return width
}

// helpTableWidths is `Table._calculate_column_widths` for the help panels.
//
// **The collapse branch is the only one that ever runs.** The flexible column
// measures the full width, so the natural sum always exceeds it; rich shaves
// the widest wrappable column towards the second widest, and since the help
// column is widest by a distance the whole excess comes off it alone. The seven
// panels of `specs/012-cli/help.md` §3.3 are the fixture.
func helpTableWidths(columns []helpColumn, rows [][]helpCell, maxWidth int) []int {
	widths := make([]int, len(columns))
	wrappable := make([]bool, len(columns))

	for i, column := range columns {
		padding := helpPadding(i, len(columns))

		switch {
		case column.Flexible:
			widths[i] = maxWidth
		case column.Width > 0:
			widths[i] = column.Width + padding
		default:
			widest := 0
			for _, row := range rows {
				if i >= len(row) {
					continue
				}
				for _, item := range row[i].items {
					widest = max(widest, utf8.RuneCountInString(item))
				}
			}
			widths[i] = widest + padding
		}
		wrappable[i] = column.Width == 0 && !column.NoWrap
	}

	if sum(widths) > maxWidth {
		widths = collapseWidths(widths, wrappable, maxWidth)
		if sum(widths) > maxWidth {
			excess := sum(widths) - maxWidth
			ones := make([]int, len(widths))
			for i := range ones {
				ones[i] = 1
			}
			widths = ratioReduce(excess, ones, widths, widths)
		}
	}
	return widths
}

// helpTable renders the boxless table inside a help panel: no borders, no
// dividers, no header, and no separator lines between rows.
func helpTable(columns []helpColumn, rows [][]helpCell, maxWidth int) []string {
	widths := helpTableWidths(columns, rows, maxWidth)

	var lines []string
	for _, row := range rows {
		lines = append(lines, helpTableRow(columns, row, widths)...)
	}
	return lines
}

func helpTableRow(columns []helpColumn, row []helpCell, widths []int) []string {
	cells := make([][]string, len(columns))
	height := 0

	for i := range columns {
		content := widths[i] - helpPadding(i, len(columns))
		if i < len(row) {
			cells[i] = HelpColumns(row[i].items, content)
		}
		height = max(height, len(cells[i]))
	}

	lines := make([]string, 0, height)
	for line := range height {
		var out strings.Builder
		for i := range columns {
			content := widths[i] - helpPadding(i, len(columns))
			if i > 0 {
				out.WriteString(" ")
			}
			text := ""
			if line < len(cells[i]) {
				text = cells[i][line]
			}
			out.WriteString(pad(text, content))
			if i < len(columns)-1 {
				out.WriteString(" ")
			}
		}
		lines = append(lines, strings.TrimRight(out.String(), " "))
	}
	return lines
}
