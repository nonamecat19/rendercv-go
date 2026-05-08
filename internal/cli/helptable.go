package cli

import "strings"

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
	// Fold marks the one column that overflows by folding rather than by
	// ellipsis: the metavar, which typer builds as
	// `Text(style=STYLE_METAVAR, overflow="fold")`
	// (`typer/rich_utils.py:376`). A `Text`'s own overflow beats the render
	// options it is given (`rich/text.py:694`), so it keeps folding inside a
	// table whose columns all default to `"ellipsis"` (`rich/table.py:90`).
	Fold bool
	// Columns marks the column whose cell is a `rich.columns.Columns` rather
	// than a `Text`: the help column, and only it (`typer/rich_utils.py:318`).
	//
	// **The difference is where the padding stops.** A `Text` justifies itself to
	// the whole cell, so an empty metavar is four `bold yellow` spaces; a
	// `Columns` lays its items out at *their* width and leaves the rest of the
	// cell to the table, so `[default: classic]` is `dim` only as far as the
	// prose above it reaches. Both measured, and at `COLUMNS=80` they coincide —
	// which is why one path looked like enough.
	Columns bool
	// Style is the column's own style, which rich lays **under** every cell in
	// it, the padding cells included. Only one column declares one: the
	// `Commands` panel's first, `bold cyan` (`STYLE_COMMANDS_TABLE_FIRST_COLUMN`,
	// `typer/rich_utils.py:64`, applied at `:487-491`).
	Style Style
}

// helpItem is one renderable inside a cell: its text, and the style its own
// rich `Text` carries.
//
// **The two are separate because the base reaches further than the text does.**
// `Text.justify` pads the string and `Text.render` then covers the padding as
// well, so an empty metavar cell is four `bold yellow` spaces rather than four
// plain ones — the base has to survive the layout in order to be applied after
// it.
type helpItem struct {
	Text Text
	Base Style
}

// helpCell is one cell. Plain columns hold a single item; the flexible column
// holds the `Columns` items, which may share a line or stack.
type helpCell struct {
	items []helpItem
}

func plain(text string) helpCell {
	return helpCell{items: []helpItem{{Text: PlainText(text)}}}
}

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
					// Cells, not runes: a column's natural width is
					// `Measurement.get`'s, and that is `cell_len`.
					widest = max(widest, cellLen(item.Text.Plain))
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

// helpTable renders the boxless table inside a help panel as plain text: no
// borders, no dividers, no header, and no separator lines between rows.
//
// It is the styled layout with the runs discarded and the trailing padding
// trimmed, which is what the plain-text fixtures compare.
func helpTable(columns []helpColumn, rows [][]helpCell, maxWidth int) []string {
	lines := helpTableLines(columns, rows, maxWidth)
	plain := make([]string, len(lines))
	for i, line := range lines {
		var out strings.Builder
		for _, segment := range line {
			out.WriteString(segment.Text)
		}
		plain[i] = strings.TrimRight(out.String(), " ")
	}
	return plain
}

// helpTableLines is that table as the runs rich cuts it into.
func helpTableLines(columns []helpColumn, rows [][]helpCell, maxWidth int) [][]Segment {
	widths := helpTableWidths(columns, rows, maxWidth)

	var lines [][]Segment
	for _, row := range rows {
		lines = append(lines, helpTableRow(columns, row, widths)...)
	}
	return lines
}

// helpTableCell lays one cell out at its content width under the column's
// overflow, which `render_lines` takes from the column (`rich/table.py:834`).
//
// Three shapes, and every help panel uses all three:
//
//   - `no_wrap` skips the wrapping and truncates the one line it has
//     (`rich/text.py:1233-1237`, then `:1248`) — the `Commands` panel's first
//     column. The tab expansion still happens, because `Text.wrap` expands
//     before it consults the flag (`:1231`).
//   - `Fold` splits an over-long word across lines and never ellipsizes — the
//     metavar alone.
//   - Everything else leaves the word whole and cuts the line with `…`.
func helpTableCell(column helpColumn, cell helpCell, width int) []Text {
	if width <= 0 {
		return []Text{{}}
	}
	if column.NoWrap {
		joined := joinItems(cell.items)
		return []Text{joined.Text.ExpandTabs().TruncateEllipsis(width).PadRight(width, joined.Base)}
	}
	if column.Columns {
		return helpColumnsOverflow(cell.items, width, column.Fold)
	}
	// A `Text` cell justifies itself to the whole column.
	return overflowLines(joinItems(cell.items), width, column.Fold)
}

// joinItems is a cell's items as the one renderable a non-`Columns` cell holds.
// Every such cell in a help panel holds exactly one item; the join is what makes
// a second one visible rather than silently dropped.
func joinItems(items []helpItem) helpItem {
	if len(items) == 1 {
		return items[0]
	}
	var joined Text
	for i, item := range items {
		if i > 0 {
			joined = joined.Append(PlainText(" "))
		}
		joined = joined.Append(item.Text.StylizeBefore(0, len([]rune(item.Text.Plain)), item.Base))
	}
	return helpItem{Text: joined}
}

// helpTableRow lays one row out as the runs rich writes it in.
//
// **The padding cells are runs of their own, and they carry the column's
// style.** Measured on the `Commands` panel: `create-theme` and the single
// padding cell after it are two separately opened `bold cyan` runs, because the
// cell and its padding are different segments (`rich/padding.py`) and rich
// merges neither.
//
// **A line the cell does not reach is one run across the whole column.** That is
// `Segment.set_shape` filling the row out to its tallest cell with a single
// blank segment, and it is visible: the continuation lines of a `Commands` row
// are thirteen `bold cyan` spaces — content *and* padding in one run, where the
// first line of the same row is two.
func helpTableRow(columns []helpColumn, row []helpCell, widths []int) [][]Segment {
	cells := make([][]Text, len(columns))
	height := 0

	for i := range columns {
		content := widths[i] - helpPadding(i, len(columns))
		if i < len(row) {
			cells[i] = helpTableCell(columns[i], row[i], content)
		}
		height = max(height, len(cells[i]))
	}

	lines := make([][]Segment, 0, height)
	for line := range height {
		var segments []Segment
		for i := range columns {
			left, right := 0, 0
			if i > 0 {
				left = 1
			}
			if i < len(columns)-1 {
				right = 1
			}
			content := widths[i] - left - right
			style := columns[i].Style

			if line >= len(cells[i]) {
				segments = append(segments, Segment{
					Text:  strings.Repeat(" ", max(left+content+right, 0)),
					Style: style,
				})
				continue
			}
			if left > 0 {
				segments = append(segments, Segment{Text: " ", Style: style})
			}
			// The cell's own padding is already inside its style; the column's
			// goes underneath all of it.
			text := cells[i][line].PadRight(content, style)
			segments = append(segments, text.Segments()...)
			if right > 0 {
				segments = append(segments, Segment{Text: " ", Style: style})
			}
		}
		lines = append(lines, segments)
	}
	return lines
}
