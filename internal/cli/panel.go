package cli

import (
	"strings"
	"unicode/utf8"
)

// PanelWidth is Rich's default console width when stdout is not a terminal,
// which is what every golden was captured with.
const PanelWidth = 80

// PanelRow is one line inside the panel.
type PanelRow struct {
	// Mark is the leading glyph — `✓` for a generated artifact.
	Mark string
	// Label and Value are the two columns; the label is padded to LabelWidth.
	Label string
	Value string
	// Timing is the duration Rich prints between the mark and the label. The
	// conformance harness normalizes it to `<duration>`, so its content is not
	// part of the contract but its position is.
	Timing string

	// Text is a whole row of free text, used by the panels that are prose
	// rather than a table — `new`'s two. When it is set the other fields are
	// ignored.
	Text string
	// IsText marks a row as free text even when Text is empty, which is how a
	// blank separator row inside a panel is spelled.
	IsText bool
}

// labelWidth is the column the value starts in, measured from two goldens
// rather than one: `Generated Typst:` (16 characters) is followed by 11 spaces
// and `Generated Markdown:` (19) by 8, so both paths begin at 27.
const labelWidth = 27

// timingWidth is the fixed column the duration sits in.
//
// **It is recoverable from the golden even though the duration itself is
// erased.** The harness replaces `<number><unit>` *and the spaces after it* with
// `<duration> `, so the normalized line is longer than the real one by a known
// amount — 2 characters on `render_typst_only`, which puts the real field at 9.
// A timing printed without that padding shifts every column to its right.
const timingWidth = 9

// Panel renders Rich's bordered box (`rich.panel.Panel`).
//
// **The geometry is the contract**, and it is all measured from
// `render_typst_only`'s stdout rather than derived from Rich's source: the box
// is exactly PanelWidth columns of *display* width, the title sits after
// `╭─ ` with a space before the fill, and the fill is `─` to the closing corner.
//
// Width here is counted in runes, not bytes. Every character these panels use is
// single-width, so runes and columns agree; a CJK name in a path would break
// that, and no golden has one.
func Panel(title string, rows []PanelRow) string {
	var out strings.Builder

	head := "╭─ " + title + " "
	out.WriteString(head)
	out.WriteString(strings.Repeat("─", PanelWidth-utf8.RuneCountInString(head)-1))
	out.WriteString("╮\n")

	for _, row := range rows {
		body := row.Mark + " " + pad(row.Timing, timingWidth) + pad(row.Label, labelWidth) + row.Value
		if row.IsText || row.Text != "" {
			body = row.Text
		}
		// The inner width is the panel minus the two borders and their padding
		// spaces.
		inner := PanelWidth - 4
		out.WriteString("│ ")
		out.WriteString(pad(body, inner))
		out.WriteString(" │\n")
	}

	out.WriteString("╰")
	out.WriteString(strings.Repeat("─", PanelWidth-2))
	out.WriteString("╯\n")
	return out.String()
}

func pad(text string, width int) string {
	if missing := width - utf8.RuneCountInString(text); missing > 0 {
		return text + strings.Repeat(" ", missing)
	}
	return text
}
