package cli

import "strings"

// columnsPadding is `rich.columns.Columns`' default padding, and the gap it
// leaves between two items on the same line: `max(left, right)` of `(0, 1)`
// (`rich/columns.py:72-73`).
const columnsPadding = 1

// HelpColumns lays out a help cell the way `rich.columns.Columns` does
// (`rich/columns.py:62-170`), returning one string per line.
//
// **It is a flow layout, not a stack, and both outcomes appear in the goldens.**
// `_get_parameter_help` wraps up to four items — the prose, the env var, the
// default and `[required]` — in a `Columns` (`typer/rich_utils.py:318`), and
// whether they share a line depends on whether they fit:
//
//   - `render`'s `input_file_name` renders `The YAML input file. [required]` on
//     **one** line, because 20 + 1 + 10 fits the 44-cell column;
//   - `new`'s `--theme` wraps its prose over six lines and puts
//     `[default: classic]` on its **own** line, because the prose alone fills
//     the 33-cell column.
//
// So neither "join with a space" nor "one per line" is right, and each is right
// for one of the two.
func HelpColumns(items []string, width int) []string {
	return helpColumnsOverflow(items, width, false)
}

// helpColumnsOverflow is HelpColumns under an explicit overflow: fold splits an
// over-long word across lines, and the default cuts it with `…`. The choice
// belongs to the enclosing table column (`rich/table.py:834`), which is why it
// is a parameter here rather than a property of the layout.
func helpColumnsOverflow(items []string, width int, fold bool) []string {
	if len(items) == 0 {
		return nil
	}

	// **Each item measures at most the column width.** `Measurement.get` clamps
	// to `options.max_width`, which is why a 138-character help string measures
	// 33 in a 33-cell column and the loop below can terminate.
	//
	// **In cells, not runes**: `Measurement.get` measures with `cell_len`
	// (`rich/measure.py`), so a wide character claims the two columns it
	// occupies. Nothing in the captured help text differs between the two
	// counts today, which is why this stood; `å` in the locale list is already
	// proof that non-ASCII reaches here.
	measured := make([]int, len(items))
	for i, item := range items {
		measured[i] = min(cellLen(item), width)
	}

	count := columnCount(measured, width)

	var lines []string
	for start := 0; start < len(items); start += count {
		end := min(start+count, len(items))
		lines = append(lines, columnsRow(items[start:end], measured, count, width, fold)...)
	}
	return lines
}

// columnCount is the `while column_count > 1` loop of `rich/columns.py:128-142`:
// start with one column per item and drop one whenever the running total
// overflows.
func columnCount(measured []int, width int) int {
	count := len(measured)
	for count > 1 {
		widths := make([]int, 0, count)
		overflowed := false

		for i, itemWidth := range measured {
			column := i % count
			if column == len(widths) {
				widths = append(widths, 0)
			}
			widths[column] = max(widths[column], itemWidth)

			total := columnsPadding * (len(widths) - 1)
			for _, w := range widths {
				total += w
			}
			if total > width {
				// `column_count = len(widths) - 1`, then start over.
				count = len(widths) - 1
				overflowed = true
				break
			}
		}
		if !overflowed {
			break
		}
	}
	return max(count, 1)
}

// columnsRow renders one row of the flow, wrapping each cell to its column's
// width and padding the shorter cells so the row is rectangular.
func columnsRow(row []string, measured []int, count, width int, fold bool) []string {
	widths := gridColumnWidths(measured, count, width)

	cells := make([][]string, len(row))
	height := 0
	for i, item := range row {
		cells[i] = overflowLines(item, widths[i], fold)
		height = max(height, len(cells[i]))
	}

	lines := make([]string, 0, height)
	for line := range height {
		var out strings.Builder
		for i := range row {
			if i > 0 {
				out.WriteString(strings.Repeat(" ", columnsPadding))
			}
			text := ""
			if line < len(cells[i]) {
				text = cells[i][line]
			}
			// The last column is not padded: the outer table pads the cell.
			if i < len(row)-1 {
				text = pad(text, widths[i])
			}
			out.WriteString(text)
		}
		lines = append(lines, strings.TrimRight(out.String(), " "))
	}
	return lines
}

// gridColumnWidths is each grid column's width: the widest item that lands in it.
// A single column takes the whole cell, which is what makes the stacked case
// wrap to the full width.
func gridColumnWidths(measured []int, count, width int) []int {
	if count <= 1 {
		return []int{width}
	}
	widths := make([]int, count)
	for i, itemWidth := range measured {
		column := i % count
		widths[column] = max(widths[column], itemWidth)
	}
	return widths
}

// overflowLines wraps one item under an overflow. Folding is `divide_line`'s
// `fold=True` and needs no truncation afterwards, because no line it produces
// is wider than the column. The ellipsis path is the other half of
// `Text.wrap`: divide without folding, so a word too long for the column stays
// whole, then cut each line at the width (`rich/text.py:1239`, `:1248`).
func overflowLines(item string, width int, fold bool) []string {
	if fold {
		return wrap(item, width)
	}
	lines := wrapKeepingWords(item, width)
	for i, line := range lines {
		lines[i] = truncate(line, width)
	}
	return lines
}
