package cli

import (
	"os"
	"strconv"
	"strings"
	"unicode"
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
	// rather than a table — the validation table's lines. When it is set the
	// other fields are ignored.
	Text string
	// IsText marks a row as free text even when Text is empty, which is how a
	// blank separator row inside a panel is spelled.
	IsText bool

	// Body is the row as **styled** text, and it may carry newlines.
	//
	// It is how the panels upstream builds from one markup string reach the
	// box: `new` and `create-theme` both pass a single `"\n".join(lines)` to
	// `rich.panel.Panel` (`new_command.py:171-178`,
	// `create_theme_command.py:57-64`), so their whole body is one renderable
	// whose spans cross the line breaks — `create-theme`'s outer `[purple]`
	// runs over five lines. Splitting it into a row per line first would cut
	// those spans at the wrong places.
	//
	// When it is set the other fields are ignored.
	Body Text
}

// body is the row as styled text — the line `print_progress_panel` assembles,
// markup and all (`cli/render_command/progress_panel.py:104-107`):
//
//	f"[green]✓[/green] {timing} {message:<26} {paths_display}"
//
// with `timing` `[bold green]{timing_ms + ' ms':<8}[/bold green]` and
// `paths_display` `[purple]{paths}[/purple]`.
//
// Three things the plain string this replaces cannot say, each measured:
//
//  1. **The timing field's padding is inside its run.** `bold green` covers
//     `11 ms   ` to the full eight columns, because the padding is applied to
//     the field before the markup closes around it.
//  2. **The message and its padding are unstyled**, and so is the single space
//     that separates each field from the next.
//  3. **The trailing padding after the paths is outside the `purple` run** — it
//     belongs to the panel, not to the field.
//
// The styles are attached unconditionally rather than behind a flag: a row with
// a `Mark` is the progress panel's row and no other surface builds one, and on
// a terminal with no colour every one of these renders as the plain string it
// used to be.
func (r PanelRow) body() Text {
	if r.Body.Plain != "" || len(r.Body.Spans) > 0 {
		return r.Body
	}
	if r.IsText || r.Text != "" {
		return PlainText(r.Text)
	}
	return StyledText(r.Mark, StyleGreen).
		Append(PlainText(" ")).
		Append(StyledText(pad(r.Timing, timingWidth-1), StyleBoldGreen)).
		Append(PlainText(" " + pad(r.Label, labelWidth))).
		Append(StyledText(r.Value, StylePurple))
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
//
// **The nine columns are eight plus a separator, and the seam is visible on a
// terminal**: upstream pads the field to eight *inside* the `bold green` markup
// (`{step.timing_ms + ' ms':<8}`, `progress_panel.py:104`) and then writes one
// unstyled space after it, so `body` styles `timingWidth-1` and leaves the last
// column plain.
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
	return renderPanel(PlainText(title), rows, Style{}, Terminal{})
}

// StyledPanel is Panel with a border style, written for one terminal — the
// `border_style` argument every `rich.panel.Panel` in RenderCV passes
// (`cli/error_handler.py:46`, `cli/render_command/progress_panel.py:132`).
//
// **It is a separate entry point rather than two more parameters on `Panel`.**
// `Panel` is called from the surfaces whose styling has not landed yet, and its
// callers' tests assert whole panels as plain strings; a styled panel on a
// terminal that has no colour is byte-identical to a plain one, so the two
// converge exactly where they must.
//
// **The title is a `Text`, not a string, because its markup changes the bytes.**
// `title="[bold red]Error[/bold red]"` puts span boundaries either side of the
// word, and Rich opens a run per boundary — so the `Error` panel's band is three
// runs where the progress panel's plain `Your CV is ready` is one, even though
// both are styled edge to edge. Both were measured.
func StyledPanel(title Text, rows []PanelRow, border Style, terminal Terminal) string {
	return renderPanel(title, rows, border, terminal)
}

func renderPanel(title Text, rows []PanelRow, border Style, terminal Terminal) string {
	var out strings.Builder
	for _, line := range panelLines(title, rows, border) {
		out.WriteString(renderSegments(line, terminal))
		out.WriteString("\n")
	}
	return out.String()
}

// panelLines is the box as lines of segments, before any terminal has been
// consulted.
func panelLines(title Text, rows []PanelRow, border Style) [][]Segment {
	width := ConsoleWidth()

	// The inner width is the panel minus the two borders and their padding
	// spaces.
	inner := width - 4

	lines := [][]Segment{panelTop(title, width, border)}

	// **At four columns or fewer there is no body at all.** The child renders
	// at `width - 4`, and Rich yields no lines for a width of zero
	// (`rich/panel.py:225`), so the box is its two borders and nothing else.
	if inner <= 0 {
		return append(lines, []Segment{{Text: panelRule("╰", "╯", width), Style: border}})
	}

	for _, row := range rows {
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
		for _, piece := range row.body().SplitLines() {
			for _, line := range wrapText(piece, inner) {
				// **The padding either side of the body is unstyled**, and only
				// the two border characters carry the style — measured on the
				// `Error` panel, whose row is
				// `S(│)` + the text and its padding + `S(│)`, and again on the
				// progress panel, where the padding that follows a `purple`
				// path is outside the run.
				body := line.Segments()
				segments := make([]Segment, 0, len(body)+4)
				segments = append(segments, Segment{Text: "│", Style: border}, Segment{Text: " "})
				segments = append(segments, body...)
				segments = append(segments,
					Segment{Text: strings.Repeat(" ", max(inner-cellLen(line.Plain), 0)) + " "},
					Segment{Text: "│", Style: border},
				)
				lines = append(lines, segments)
			}
		}
	}

	// The closing border is **one** segment, corners included
	// (`rich/panel.py:250-256`).
	return append(lines, []Segment{{Text: panelRule("╰", "╯", width), Style: border}})
}

// panelTop draws the bordered line the title sits in
// (`rich/panel.py:234-246`).
//
// **The title is cropped, not ellipsized, and it never widens the box.** Rich
// renders it at exactly `width - 4` under the console's default overflow,
// `"fold"` (`rich/text.py:36`), and `truncate` spends a column on `…` only for
// `"ellipsis"` — so `Error` at nine columns is `╭─ Erro─╮`. `align_text` then
// pads whatever is left over with the box's own `─` (`:176-186`).
//
// The port instead wrote `╭─ ` + title + ` ` and filled the remainder, which is
// a negative count once the title no longer fits: `COLUMNS=30 rendercv-go
// render CV.yaml` on a document with any validation error died with `panic:
// strings: negative Repeat count` rather than printing upstream's panel.
// **How many runs the line is depends on the title's own markup**, which is why
// the title arrives as a `Text`. `[bold red]Error[/bold red]` is six —
// `╭─`, the leading pad, the word, the trailing pad, the fill and `─╮` — because
// the markup's span ends where the padding begins and Rich opens a run per span
// boundary. A title with no markup is four, its band a single run: measured,
// `ESC[90m╭─ESC[0mESC[90m Your CV is ready ESC[0m…`. Concatenated either way they
// are the string this function used to return, which is what keeps every plain
// golden unmoved.
func panelTop(title Text, width int, border Style) []Segment {
	// `title_text is None or width <= 4` (`:234`): no title, just the box's own
	// top edge.
	if width <= 4 {
		return []Segment{{Text: panelRule("╭", "╮", width), Style: border}}
	}

	// `title_text.pad(1)` (`rich/panel.py:121`): one space either side, added
	// before the crop, so a title cropped to nothing still leaves its leading
	// space. The border style then goes **underneath** the title's own
	// (`stylize_before`, `:205-206`), over the padding as well.
	padded := PlainText(" ").Append(title).Append(PlainText(" "))
	padded = padded.StylizeBefore(0, len([]rune(padded.Plain)), border)

	inner := width - 4
	band := padded.Truncate(inner).Segments()
	if excess := inner - cellLen(padded.Plain); excess > 0 {
		// `Text.assemble(text, (character * excess_space, style))`
		// (`rich/panel.py:181-186`): the fill is a span of its own, and
		// therefore a run of its own.
		band = append(band, Segment{Text: strings.Repeat("─", excess), Style: border})
	}

	line := make([]Segment, 0, len(band)+2)
	line = append(line, Segment{Text: "╭─", Style: border})
	line = append(line, band...)
	return append(line, Segment{Text: "─╮", Style: border})
}

// panelRule draws a titleless border. The fill is `width - 2` columns, and the
// whole line is cropped to the console width, which is what makes a one-column
// panel a lone `╭`.
func panelRule(left, right string, width int) string {
	line := left + strings.Repeat("─", max(width-2, 0)) + right
	if cellLen(line) > width {
		line, _ = cutCells(line, width)
	}
	return line
}

// wrap folds one row to the panel's inner width, the way Rich's `divide_line`
// does: break before the word that would overflow and hard-split a single word
// that cannot fit on a line of its own.
//
// It always returns at least one line, so a blank separator row survives.
func wrap(text string, width int) []string {
	return fold(text, width, true)
}

// wrapText is wrap over styled text: **the same breaks, at the same offsets,
// with every span sliced to the line it fell in.**
//
// It re-uses `divideLine` rather than re-deriving anything, which is the whole
// reason the styled panel needs no wrapping logic of its own (spec 012 delta
// §7.1.3). `rstripEnd` and the fold crop only ever trim from the end of a line,
// so the surviving runes still start at the break offset and `Slice` places the
// spans without a second measurement.
func wrapText(text Text, width int) []Text {
	// `Text.wrap` expands tabs before it measures (`rich/text.py:1231-1233`),
	// and the spans have to follow the widening or every offset after the tab
	// points at the wrong rune.
	text = text.ExpandTabs()

	if width <= 0 || cellLen(text.Plain) <= width {
		return []Text{text}
	}

	runes := []rune(text.Plain)
	breaks := divideLine(runes, width, true)

	lines := make([]Text, 0, len(breaks)+1)
	previous := 0
	for _, offset := range append(breaks, len(runes)) {
		trimmed := rstripEnd(runes[previous:offset], width)
		lines = append(lines, text.Slice(previous, previous+len([]rune(trimmed))).Truncate(width))
		previous = offset
	}
	return lines
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
	// `Text.wrap` expands the line's tabs before it measures anything
	// (`rich/text.py:1231-1233`), so this is where the expansion belongs: a row
	// whose tab pushes it past the width wraps because of it.
	text = expandTabs(text)

	if width <= 0 || cellLen(text) <= width {
		return []string{text}
	}

	runes := []rune(text)
	breaks := divideLine(runes, width, splitLongWords)

	lines := make([]string, 0, len(breaks)+1)
	previous := 0
	for _, offset := range append(breaks, len(runes)) {
		line := rstripEnd(runes[previous:offset], width)
		if splitLongWords {
			// `line.truncate(width, overflow=wrap_overflow)`
			// (`rich/text.py:1249`) under the panel's `"fold"`, which crops
			// rather than ellipsizes. The ellipsizing callers truncate the
			// lines themselves, because their `…` costs a column this one does
			// not spend.
			line, _ = cutCells(line, width)
		}
		lines = append(lines, line)
		previous = offset
	}
	return lines
}

// wordSpan is one match of Rich's `re_word` (`rich/_wrap.py:9`).
type wordSpan struct {
	start, end int
}

// richWords is Rich's `words` (`rich/_wrap.py:12-23`), which scans
// `\s*\S+\s*`.
//
// **The whitespace after a word belongs to the word.** That single fact is what
// makes a run of padding fold into lines of its own instead of vanishing at the
// break, and the port's own splitter — which yielded the word and its gap
// separately, then dropped the gap whenever it did not fit — was a line short
// every time a padded field overflowed.
//
// Offsets are in runes, because `divide_line` advances by `len(line)` over a
// Python string, which counts codepoints.
func richWords(text []rune) []wordSpan {
	var spans []wordSpan
	position := 0
	for position < len(text) {
		start := position
		for position < len(text) && unicode.IsSpace(text[position]) {
			position++
		}
		if position == len(text) {
			// Trailing whitespace with no `\S+` after it is not a match at all.
			break
		}
		for position < len(text) && !unicode.IsSpace(text[position]) {
			position++
		}
		for position < len(text) && unicode.IsSpace(text[position]) {
			position++
		}
		spans = append(spans, wordSpan{start: start, end: position})
	}
	return spans
}

// divideLine is Rich's `divide_line` (`rich/_wrap.py:26-78`): the rune offsets
// a line is broken at.
//
// A word is measured **without** its trailing whitespace when deciding whether
// it fits (`:45`) and **with** it when the cursor advances (`:51`, `:64`), so a
// line may end up wider than the width by exactly the whitespace it carries —
// which is what rstripEnd then trims back.
func divideLine(text []rune, width int, foldWords bool) []int {
	var breaks []int
	offset := 0

	for _, span := range richWords(text) {
		word := text[span.start:span.end]
		wordLength := cellLen(strings.TrimRightFunc(string(word), unicode.IsSpace))
		if width-offset >= wordLength {
			offset += cellLen(string(word))
			continue
		}

		start := span.start
		switch {
		case wordLength > width && foldWords:
			// The word does not fit on any line, so it is broken at a column
			// boundary — and between graphemes, so a double-width character is
			// never cut in half.
			chunks := chopCells(string(word), width)
			for i, chunk := range chunks {
				if start != 0 {
					breaks = append(breaks, start)
				}
				if i == len(chunks)-1 {
					offset = cellLen(chunk)
				} else {
					start += len([]rune(chunk))
				}
			}
		case wordLength > width:
			// Folding is not allowed, so the word starts a line and the caller
			// crops it.
			if start != 0 {
				breaks = append(breaks, start)
			}
			offset = cellLen(string(word))
		case offset != 0 && start != 0:
			// It fits on the next, empty line.
			breaks = append(breaks, start)
			offset = cellLen(string(word))
		}
	}
	return breaks
}

// rstripEnd is Rich's `Text.rstrip_end` (`rich/text.py:1015-1023`): trailing
// whitespace is removed **only as far as the width**, so a line of `ms` plus
// four spaces at width five keeps three of them.
//
// The comparison is against the rune count, not the cell count, exactly as
// upstream compares against `len(self)`.
func rstripEnd(line []rune, size int) string {
	if len(line) <= size {
		return string(line)
	}
	trailing := 0
	for i := len(line) - 1; i >= 0 && unicode.IsSpace(line[i]); i-- {
		trailing++
	}
	return string(line[:len(line)-min(trailing, len(line)-size)])
}

// tabSize is Rich's default `Console.tab_size` (`rich/console.py:643`), and
// nothing in RenderCV overrides it.
const tabSize = 8

// expandTabs is Rich's `Text.expand_tabs` (`rich/text.py:817-857`): a tab is
// replaced by **one space plus however many more it takes to reach the next
// eight-column stop**, and the stops are counted in display cells from the
// start of the line — so a tab after a wide character moves less far than a tab
// after a narrow one.
//
// The port wrote the tab byte through, which left the terminal to decide how
// wide the row was and the panel's border wherever that landed.
func expandTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}

	var out strings.Builder
	position := 0
	rest := line
	for {
		index := strings.IndexByte(rest, '\t')
		if index < 0 {
			out.WriteString(rest)
			return out.String()
		}

		// Rich rewrites the tab itself as a space and only then asks how far
		// the next stop is, which is why a tab that lands exactly on a stop
		// still advances a full eight columns.
		part := rest[:index] + " "
		out.WriteString(part)
		position += cellLen(part)
		if remainder := position % tabSize; remainder != 0 {
			spaces := tabSize - remainder
			out.WriteString(strings.Repeat(" ", spaces))
			position += spaces
		}
		rest = rest[index+1:]
	}
}

func pad(text string, width int) string {
	if missing := width - cellLen(text); missing > 0 {
		return text + strings.Repeat(" ", missing)
	}
	return text
}
