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
	styled := make([]helpItem, len(items))
	for i, item := range items {
		styled[i] = helpItem{Text: PlainText(item)}
	}
	return plainLines(helpColumnsOverflow(styled, width, false))
}

// helpColumnsOverflow is HelpColumns under an explicit overflow: fold splits an
// over-long word across lines, and the default cuts it with `…`. The choice
// belongs to the enclosing table column (`rich/table.py:834`), which is why it
// is a parameter here rather than a property of the layout.
func helpColumnsOverflow(items []helpItem, width int, fold bool) []Text {
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
		measured[i] = min(cellLen(item.Text.Plain), width)
	}

	count := columnCount(measured, width)

	var lines []Text
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
//
// **Every item is padded to its grid column, inside its own style**, the last
// one included — which the plain layout could leave to the outer table because
// trailing spaces are trailing spaces either way. On a terminal they are not:
// `[default: classic]` is the only item on its line and comes out `dim` across
// all thirty-three columns of the cell, padding and all (measured on
// `new --help`).
func columnsRow(row []helpItem, measured []int, count, width int, fold bool) []Text {
	widths := gridColumnWidths(measured, count, width)

	cells := make([][]Text, len(row))
	height := 0
	for i, item := range row {
		cells[i] = overflowLines(item, widths[i], fold)
		height = max(height, len(cells[i]))
	}

	lines := make([]Text, 0, height)
	for line := range height {
		var out Text
		for i := range row {
			if i > 0 {
				out = out.Append(PlainText(strings.Repeat(" ", columnsPadding)))
			}
			if line >= len(cells[i]) {
				// A grid cell shorter than its neighbour is padded by the grid,
				// which carries no style of its own.
				out = out.Append(PlainText(strings.Repeat(" ", widths[i])))
				continue
			}
			out = out.Append(cells[i][line])
		}
		lines = append(lines, out)
	}
	return lines
}

// gridColumnWidths is each grid column's width: the widest item that lands in
// it (`rich/columns.py:128-142`, whose `widths` the grid is built from).
//
// **A single column is the widest item, not the whole cell.** The two agree
// whenever an item fills the cell, which is every case at `COLUMNS=80` and is
// why the plain layout could take the cell — and they part at a wider console:
// measured at `COLUMNS=200`, `new --help`'s `[default: classic]` is `dim` across
// 138 columns, the width of the prose above it, with the cell's remaining 16
// left plain by the table.
func gridColumnWidths(measured []int, count, width int) []int {
	if count <= 1 {
		widest := 0
		for _, itemWidth := range measured {
			widest = max(widest, itemWidth)
		}
		return []int{min(widest, width)}
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
//
// Each line comes back padded to the column inside the item's own style, which
// is where `Text.justify` puts it.
func overflowLines(item helpItem, width int, fold bool) []Text {
	var lines []Text
	if fold {
		lines = wrapText(item.Text, width)
	} else {
		lines = wrapTextKeepingWords(item.Text, width)
		for i, line := range lines {
			lines[i] = line.TruncateEllipsis(width)
		}
	}
	for i, line := range lines {
		lines[i] = line.PadRight(width, item.Base)
	}
	return lines
}

// plainLines is the styled layout as the strings the plain callers expect, with
// the padding this layout now keeps trimmed back off.
func plainLines(lines []Text) []string {
	if lines == nil {
		return nil
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = strings.TrimRight(line.Plain, " ")
	}
	return plain
}
