package cli

import (
	"math"
	"strings"
)

// TableColumn is one column of the validation-error table
// (`cli/render_command/progress_panel.py:148-151`).
type TableColumn struct {
	// Header is the column's title row.
	Header string
	// NoWrap is upstream's `no_wrap=True`: the cell is never folded, only
	// truncated. `Location` and `Input Value` carry it; `Explanation` does not.
	NoWrap bool

	// Style is `add_column(style=…)` — `cyan`, `magenta` and `orange4` on the
	// three validation columns (`progress_panel.py:149-151`).
	//
	// **It covers the cells and not the header.** Rich renders a header cell
	// under `header_style` alone (`rich/table.py:671-674`), and the column's own
	// style is passed to the *cell* renderer (`:838-841`) — measured on a pty,
	// `Location`'s header is `ESC[1m` and never `ESC[1;36m`.
	Style Style
}

// tableHeaderStyle is Rich's `table.header` (`rich/default_styles.py:110`),
// which `Table(header_style="table.header")` resolves by default
// (`rich/table.py:208`). RenderCV overrides neither.
var tableHeaderStyle = StyleBold

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
	var out strings.Builder
	for _, line := range StyledTable(columns, rows, maxWidth) {
		out.WriteString(line.Plain)
		out.WriteString("\n")
	}
	return out.String()
}

// StyledTable is Table as **styled lines**, one `Text` per line, for the caller
// that nests it in a panel on a terminal.
//
// It is where the table's three column styles live, and it is a separate entry
// point for the same reason `StyledPanel` is: the plain callers assert whole
// tables as strings, and on a terminal with no colour the two forms are
// byte-identical — `Table` is literally this function's `Plain`.
//
// **The box's own characters are unstyled.** Only the cells carry a colour, and
// only the enclosing panel's border carries `bold red` — measured on a pty.
func StyledTable(columns []TableColumn, rows [][]string, maxWidth int) []Text {
	// Every string Rich renders becomes a `Text`, whose constructor sanitizes
	// it (`rich/text.py:156`). Do it once, before anything is measured, so the
	// stripped characters cannot occupy a column.
	columns = append([]TableColumn(nil), columns...)
	for i := range columns {
		columns[i].Header = stripControlCodes(columns[i].Header)
	}
	sanitized := make([][]string, len(rows))
	for i, row := range rows {
		sanitized[i] = make([]string, len(row))
		for j, cell := range row {
			sanitized[i][j] = stripControlCodes(cell)
		}
	}
	rows = sanitized

	// `box=ROUNDED` with the default edges: one divider per column plus the
	// closing one.
	widths := columnWidths(columns, rows, maxWidth, len(columns)+1)

	lines := []Text{PlainText(rule(widths, "╭", "┬", "╮"))}
	lines = append(lines, tableRow(columns, headerCells(columns), widths, true)...)
	lines = append(lines, PlainText(rule(widths, "├", "┼", "┤")))
	for i, row := range rows {
		if i > 0 {
			// `show_lines=True` puts a separator between every pair of rows and
			// nowhere else.
			lines = append(lines, PlainText(rule(widths, "├", "┼", "┤")))
		}
		lines = append(lines, tableRow(columns, row, widths, false)...)
	}
	return append(lines, PlainText(rule(widths, "╰", "┴", "╯")))
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
	// **The table's own width bounds every column before anything is reduced.**
	// `_measure_column` ends in `.with_maximum(max_width)`
	// (`rich/table.py:748`), and `max_width` there is the space left after the
	// box's dividers (`:489`). The port measured the natural width instead, so
	// `collapse` and the even reduction below divided a different excess and
	// laid out every column of a narrow validation table one column off.
	available := maxWidth - extra

	widths := make([]int, len(columns))
	for i, column := range columns {
		widest := cellLen(column.Header)
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			for line := range strings.Lines(row[i]) {
				widest = max(widest, cellLen(strings.TrimRight(line, "\n")))
			}
		}
		// `if max_width < 1: return Measurement(0, 0)` (`:725-726`), and then
		// `_range.maximum or 1` (`:532`) turns that zero back into one — so a
		// table with no room at all still starts from a column apiece.
		measured := 0
		if available >= 1 {
			measured = min(widest+tablePadding, available)
		}
		widths[i] = max(measured, 1)
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

// rule draws one horizontal border of the ROUNDED box. It carries no style:
// the box characters are plain and only the cells are coloured.
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
	return out.String()
}

// tableRow renders one row, which may occupy several lines when a wrappable
// cell folds. Cells shorter than the tallest one are padded with blanks.
//
// **A cell is three runs and a blank continuation is one**, and the difference
// is not cosmetic — it is what the bytes say. A rendered line is a `Padding`
// around the cell, so Rich emits the left pad, the padded content and the right
// pad separately, each opened and closed:
// `ESC[36m ESC[0mESC[36mlocale  ESC[0mESC[36m ESC[0m`. A line that exists only
// because a taller cell in the same row demanded it is `Segment(" " * width,
// style)` — **one** run spanning the whole column, padding included
// (`Segment.align_top`, `rich/segment.py:489`): `ESC[36m          ESC[0m`.
// Both were measured on a pty.
func tableRow(columns []TableColumn, cells []string, widths []int, header bool) []Text {
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

	lines := make([]Text, 0, height)
	for line := range height {
		row := PlainText("│")
		for i, width := range widths {
			style := columns[i].Style
			if header {
				style = tableHeaderStyle
			}
			content := max(width-tablePadding, 0)
			switch {
			case line >= len(rendered[i]), width < tablePadding:
				// The blank continuation, and the column too narrow to hold its
				// own padding: one run over the whole column.
				row = row.Append(StyledText(strings.Repeat(" ", width), style))
			default:
				row = row.Append(StyledText(" ", style)).
					Append(StyledText(pad(rendered[i][line], content), style)).
					Append(StyledText(" ", style))
			}
			row = row.Append(PlainText("│"))
		}
		lines = append(lines, row)
	}
	return lines
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
		// **`no_wrap` skips the wrapping, not the tab expansion**: `Text.wrap`
		// expands each line before it looks at the flag (`rich/text.py:1231`),
		// so a tab in a `Location` or `Input Value` cell becomes spaces here
		// too. `fold` does it for the wrappable branch below.
		return []string{truncate(expandTabs(text), width)}
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
	if cellLen(text) <= width {
		return text
	}
	if width <= 1 {
		return strings.Repeat(ellipsis, width)
	}
	// `set_cell_size(plain, max_width - 1) + "…"` (`rich/text.py:875`): the
	// ellipsis costs one column, and a cut that lands inside a double-width
	// character leaves a space in its place rather than half a glyph.
	head, _ := cutCells(text, width-1)
	return head + ellipsis
}

// stripControlCodes is Rich's `strip_control_codes`
// (`rich/control.py:181-192`), which `Text.__init__` applies to every string
// Rich is asked to render (`rich/text.py:156`).
//
// The set it removes is exactly five codepoints — `STRIP_CONTROL_CODES`
// (`rich/control.py:9-15`) — and no others: a `\x01`, `\x1b` or `\x7f` in an
// input value reaches upstream's terminal untouched, so it must reach ours too.
// Tab and newline survive as well; Rich handles those elsewhere.
func stripControlCodes(text string) string {
	if !strings.ContainsAny(text, "\a\b\v\f\r") {
		return text
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '\a', '\b', '\v', '\f', '\r':
			return -1
		default:
			return r
		}
	}, text)
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
