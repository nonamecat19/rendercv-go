package cli

import (
	"os"
	"strconv"
	"strings"
)

// PanelWidth is Rich's fallback console width — what it uses when stdout is not
// a terminal and nothing says otherwise. Every golden was captured at it.
const PanelWidth = 80

// ConsoleWidth is the width Rich would lay out to (G-11).
//
// **Rich honours `COLUMNS` even when stdout is a pipe**, and the port used to
// print 80 columns unconditionally. At `COLUMNS=100` that was 20 columns
// narrower than upstream; at `COLUMNS=60` it was 20 wider, so every panel
// overflowed the reader's terminal and wrapped. No golden could see either,
// because all of them are captured at 80.
//
// A value that is not a positive number is ignored, which is what Rich's own
// `int()` guard amounts to.
func ConsoleWidth() int {
	if raw := os.Getenv("COLUMNS"); raw != "" {
		if width, err := strconv.Atoi(raw); err == nil && width > 0 {
			return width
		}
	}
	return PanelWidth
}

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
// **Width here is counted in display columns**, by `cellLen` — Rich's
// `cell_len`, table and all. It used to be counted in runes, on the grounds
// that every character in the goldens is single-width; `rendercv new
// "Ольга Ковальчук 李雷"` is enough to break that, and the two wide characters
// pushed the row two columns past the border it was drawn inside.
func Panel(title string, rows []PanelRow) string {
	var out strings.Builder

	width := ConsoleWidth()

	head := "╭─ " + title + " "
	out.WriteString(head)
	out.WriteString(strings.Repeat("─", width-cellLen(head)-1))
	out.WriteString("╮\n")

	// The inner width is the panel minus the two borders and their padding
	// spaces.
	inner := width - 4

	for _, row := range rows {
		body := row.Mark + " " + pad(row.Timing, timingWidth) + pad(row.Label, labelWidth) + row.Value
		if row.IsText || row.Text != "" {
			body = row.Text
		}
		// **A newline inside a row is a hard break**, not a character: Rich's
		// `Text` splits on it before it wraps, so each segment gets its own
		// bordered line. `new "$(printf 'a\nb')"` reaches this — the file name
		// keeps the newline, because `new` sanitizes nothing but spaces
		// (`cli/new_command/new_command.py:81`) — and without the split the
		// newline lands raw inside the box and breaks the border.
		//
		// **A row wider than the panel wraps**, and its continuation is not
		// indented — measured on `theme_classic`, whose two PNG paths do not fit
		// on one line and whose second line starts at the panel's first column.
		for _, segment := range strings.Split(body, "\n") {
			for _, line := range wrap(segment, inner) {
				out.WriteString("│ ")
				out.WriteString(pad(line, inner))
				out.WriteString(" │\n")
			}
		}
	}

	out.WriteString("╰")
	out.WriteString(strings.Repeat("─", width-2))
	out.WriteString("╯\n")
	return out.String()
}

// wrap folds one row to the panel's inner width, the way Rich's `divide_line`
// does: break before the word that would overflow, drop the whitespace the break
// consumed, and hard-split a single word that cannot fit on a line of its own.
//
// It always returns at least one line, so a blank separator row survives.
func wrap(text string, width int) []string {
	return fold(text, width, true)
}

// wrapKeepingWords is wrap without the hard split: a word too long for a line
// is left whole for the caller to truncate.
//
// **The two differ, and a golden shows it.** A Rich Panel's overflow is "fold",
// so an over-long token continues on the next line; a table Column's is
// "ellipsis", so the same token is cut with `…` — `err_unknown_theme`'s absolute
// path ends in `…` rather than wrapping onto line seven.
func wrapKeepingWords(text string, width int) []string {
	return fold(text, width, false)
}

func fold(text string, width int, splitLongWords bool) []string {
	if width <= 0 || cellLen(text) <= width {
		return []string{text}
	}

	var lines []string
	var line strings.Builder
	lineWidth := 0

	flush := func() {
		lines = append(lines, line.String())
		line.Reset()
		lineWidth = 0
	}

	for word, gap := range words(text) {
		wordWidth := cellLen(word)

		// The word does not fit after what is already on the line. The gap that
		// preceded it is dropped rather than carried to the next line, which is
		// why a wrapped row's continuation starts flush left.
		if lineWidth > 0 && lineWidth+wordWidth > width {
			flush()
		}

		// A word too long for an empty line is broken at the width, which is
		// what Rich's `fold` overflow does. Callers that want it ellipsized
		// instead pass splitLongWords false and truncate the line themselves.
		//
		// The break is at a **column** boundary, and between graphemes:
		// `chop_cells` never cuts a double-width character in half, so a line
		// of wide characters holds half as many of them as a line of Latin
		// ones.
		if splitLongWords && wordWidth > width {
			chunks := chopCells(word, width)
			lines = append(lines, chunks[:len(chunks)-1]...)
			word = chunks[len(chunks)-1]
			wordWidth = cellLen(word)
		}

		line.WriteString(word)
		lineWidth += wordWidth

		if lineWidth+cellLen(gap) <= width {
			line.WriteString(gap)
			lineWidth += cellLen(gap)
		}
	}

	if lineWidth > 0 || len(lines) == 0 {
		// Trailing whitespace on the last line is padding, not content.
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return lines
}

// words splits a row into (word, following spaces) pairs. The label column is
// padded with runs of spaces, so the gap is kept rather than collapsed: it is
// what puts the value in its column.
func words(text string) func(func(string, string) bool) {
	return func(yield func(string, string) bool) {
		for i := 0; i < len(text); {
			word := i
			for i < len(text) && text[i] != ' ' {
				i++
			}
			end := i
			for i < len(text) && text[i] == ' ' {
				i++
			}
			if !yield(text[word:end], text[end:i]) {
				return
			}
		}
	}
}

func pad(text string, width int) string {
	if missing := width - cellLen(text); missing > 0 {
		return text + strings.Repeat(" ", missing)
	}
	return text
}
