package cli

import (
	"math"
	"strings"
	"unicode/utf8"
)

// TableColumn is one column of the validation-error table
// (`cli/render_command/progress_panel.py:148-151`).
type TableColumn struct {
	// Header is the column's title row.
	Header string
	// NoWrap is upstream's `no_wrap=True`: the cell is never folded, only
	// truncated. `Location` and `Input Value` carry it; `Explanation` does not.
	NoWrap bool
}

// tablePadding is Rich's default cell padding, one space either side.
const tablePadding = 2

// ellipsis is what Rich appends when it truncates. A Column's default overflow
// is "ellipsis", which is why an over-long path in the goldens ends in `…`
// rather than being folded onto the next line.
const ellipsis = "…"

// Table renders Rich's `rich.table.Table(expand=True, show_lines=True,
// box=rich.box.ROUNDED)` — the exact construction
// `print_validation_errors` uses (`progress_panel.py:147`).
//
// **The layout algorithm is reproduced, not approximated.** Rich decides column
// widths in three stages, and each one is visible in a different golden:
//
//   - every column takes its widest cell, and the leftover is *distributed* when
//     the table is narrower than the space it has (`render_override_scalar`);
//   - when it is wider, the wrappable columns are *collapsed*, tallest first
//     (`err_unknown_theme`, whose Explanation lands on exactly 49);
//   - when collapsing is not enough, every column is reduced evenly, which is how
//     `err_wrong_input` ends up with an Explanation column of width zero.
//
// A table that got any one of these wrong would still look plausible and would
// differ from upstream on every row.
func Table(columns []TableColumn, rows [][]string, maxWidth int) string {
	// `box=ROUNDED` with the default edges: one divider per column plus the
	// closing one.
	widths := columnWidths(columns, rows, maxWidth, len(columns)+1)

	var out strings.Builder
	out.WriteString(rule(widths, "╭", "┬", "╮"))
	out.WriteString(tableRow(columns, headerCells(columns), widths))
	out.WriteString(rule(widths, "├", "┼", "┤"))
	for i, row := range rows {
		if i > 0 {
			// `show_lines=True` puts a separator between every pair of rows and
			// nowhere else.
			out.WriteString(rule(widths, "├", "┼", "┤"))
		}
		out.WriteString(tableRow(columns, row, widths))
	}
	out.WriteString(rule(widths, "╰", "┴", "╯"))
	return out.String()
}

func headerCells(columns []TableColumn) []string {
	cells := make([]string, len(columns))
	for i, column := range columns {
		cells[i] = column.Header
	}
	return cells
}

// columnWidths is Rich's `Table._calculate_column_widths`. The widths it returns
// include each column's padding; the box's own dividers are the extra on top.
func columnWidths(columns []TableColumn, rows [][]string, maxWidth, extra int) []int {
	widths := make([]int, len(columns))
	for i, column := range columns {
		widest := utf8.RuneCountInString(column.Header)
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			for line := range strings.Lines(row[i]) {
				widest = max(widest, utf8.RuneCountInString(strings.TrimRight(line, "\n")))
			}
		}
		// `_range.maximum or 1`: an empty column still occupies a cell.
		widths[i] = max(widest+tablePadding, 1)
	}

	wrappable := make([]bool, len(columns))
	for i, column := range columns {
		wrappable[i] = !column.NoWrap
	}

	total := sum(widths) + extra
	switch {
	case total > maxWidth:
		widths = collapseWidths(widths, wrappable, maxWidth-extra)
		if sum(widths)+extra > maxWidth {
			// Last resort: reduce every column evenly, bounded by its own
			// width, which is what drives a column to zero.
			excess := sum(widths) + extra - maxWidth
			ones := make([]int, len(widths))
			for i := range ones {
				ones[i] = 1
			}
			widths = ratioReduce(excess, ones, widths, widths)
		}
	case total < maxWidth:
		// `expand=True`: the leftover is handed out in proportion to the
		// widths already chosen.
		for i, pad := range ratioDistribute(maxWidth-total, widths) {
			widths[i] += pad
		}
	}
	return widths
}

// collapseWidths is Rich's `Table._collapse_widths`: repeatedly shave the widest
// wrappable column down towards the second widest until the table fits.
func collapseWidths(widths []int, wrappable []bool, maxWidth int) []int {
	widths = append([]int(nil), widths...)
	total := sum(widths)
	excess := total - maxWidth

	if !anyTrue(wrappable) {
		return widths
	}

	for total > 0 && excess > 0 {
		widest := 0
		for i, width := range widths {
			if wrappable[i] {
				widest = max(widest, width)
			}
		}
		second := 0
		for i, width := range widths {
			if wrappable[i] && width != widest {
				second = max(second, width)
			}
		}
		difference := widest - second

		ratios := make([]int, len(widths))
		any := false
		for i, width := range widths {
			if wrappable[i] && width == widest {
				ratios[i] = 1
				any = true
			}
		}
		if !any || difference == 0 {
			break
		}

		maxReduce := make([]int, len(widths))
		for i := range maxReduce {
			maxReduce[i] = min(excess, difference)
		}
		widths = ratioReduce(excess, ratios, maxReduce, widths)
		total = sum(widths)
		excess = total - maxWidth
	}
	return widths
}

// ratioDistribute is Rich's `ratio_distribute` — note the ceiling, and that the
// remaining total and ratio shrink as it goes, so the last column absorbs the
// rounding.
func ratioDistribute(total int, ratios []int) []int {
	totalRatio := sum(ratios)
	remaining := total
	out := make([]int, len(ratios))
	for i, ratio := range ratios {
		var distributed int
		if totalRatio > 0 {
			distributed = int(math.Ceil(float64(ratio) * float64(remaining) / float64(totalRatio)))
		} else {
			distributed = remaining
		}
		out[i] = distributed
		totalRatio -= ratio
		remaining -= distributed
	}
	return out
}

// ratioReduce is Rich's `ratio_reduce`. Python's `round` is banker's rounding,
// and so is the helper here: `round(0.5)` is 0, not 1.
func ratioReduce(total int, ratios, maximums, values []int) []int {
	effective := make([]int, len(ratios))
	for i, ratio := range ratios {
		if maximums[i] != 0 {
			effective[i] = ratio
		}
	}
	totalRatio := sum(effective)
	out := append([]int(nil), values...)
	if totalRatio == 0 {
		return out
	}

	remaining := total
	for i, ratio := range effective {
		if ratio == 0 || totalRatio <= 0 {
			continue
		}
		distributed := min(maximums[i], bankersRound(float64(ratio)*float64(remaining)/float64(totalRatio)))
		out[i] = values[i] - distributed
		remaining -= distributed
		totalRatio -= ratio
	}
	return out
}

// bankersRound is Python's `round`: halves go to the even neighbour.
func bankersRound(value float64) int {
	rounded := math.RoundToEven(value)
	return int(rounded)
}

// rule draws one horizontal border of the ROUNDED box.
func rule(widths []int, left, join, right string) string {
	var out strings.Builder
	out.WriteString(left)
	for i, width := range widths {
		if i > 0 {
			out.WriteString(join)
		}
		out.WriteString(strings.Repeat("─", width))
	}
	out.WriteString(right)
	out.WriteString("\n")
	return out.String()
}

// tableRow renders one row, which may occupy several lines when a wrappable
// cell folds. Cells shorter than the tallest one are padded with blanks.
func tableRow(columns []TableColumn, cells []string, widths []int) string {
	rendered := make([][]string, len(widths))
	height := 1
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		// The content width is the column minus its padding. A column reduced
		// to nothing renders as an empty cell rather than a negative one.
		content := max(widths[i]-tablePadding, 0)
		rendered[i] = renderCell(cell, content, columns[i].NoWrap)
		height = max(height, len(rendered[i]))
	}

	var out strings.Builder
	for line := range height {
		out.WriteString("│")
		for i, width := range widths {
			text := ""
			if line < len(rendered[i]) {
				text = rendered[i][line]
			}
			content := max(width-tablePadding, 0)
			if width >= tablePadding {
				out.WriteString(" " + pad(text, content) + " ")
			} else {
				out.WriteString(strings.Repeat(" ", width))
			}
			_ = i
			out.WriteString("│")
		}
		out.WriteString("\n")
	}
	return out.String()
}

// renderCell lays one cell out at the given content width.
//
// Both branches end in an ellipsis rather than a fold, because a Rich Column's
// default overflow is "ellipsis" — visible in `err_unknown_theme`, where a path
// too long for the column is cut with `…` instead of continuing on the next line.
func renderCell(text string, width int, noWrap bool) []string {
	if width <= 0 {
		return []string{""}
	}
	if noWrap {
		return []string{truncate(text, width)}
	}

	var lines []string
	for paragraph := range strings.SplitSeq(text, "\n") {
		for _, line := range wrapKeepingWords(paragraph, width) {
			lines = append(lines, truncate(line, width))
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// truncate cuts text to width, spending the last column on the ellipsis.
func truncate(text string, width int) string {
	if utf8.RuneCountInString(text) <= width {
		return text
	}
	if width <= 1 {
		return strings.Repeat(ellipsis, width)
	}
	return string([]rune(text)[:width-1]) + ellipsis
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func anyTrue(values []bool) bool {
	for _, value := range values {
		if value {
			return true
		}
	}
	return false
}
