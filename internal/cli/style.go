package cli

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// colorKind says how a colour is written, and therefore how it downgrades.
type colorKind uint8

const (
	colorUnset colorKind = iota
	// colorStandard is one of the sixteen ANSI colours, written as its own SGR
	// parameter — 30-37 and 90-97. It never downgrades.
	colorStandardKind
	// colorPalette is an entry in the 256-colour palette, written `38;5;N` and
	// **downgraded to a standard colour** when the terminal has only sixteen.
	colorPaletteKind
)

// Color is a foreground colour, in the two forms RenderCV's styles use.
type Color struct {
	kind colorKind
	// code is the SGR parameter for a standard colour, or the palette index for
	// a palette one.
	code int
	// downgrade is the standard-colour SGR parameter this palette colour
	// collapses to on a sixteen-colour terminal.
	//
	// **It is a per-colour constant rather than a computed nearest-neighbour.**
	// Rich converts the palette entry to RGB and searches the sixteen
	// (`rich/color.py`, `Color.downgrade`), which needs both palettes; the port
	// carries the answer for the three palette colours RenderCV actually names,
	// each measured. A fourth palette colour must arrive with its own measured
	// value rather than be guessed.
	downgrade int
}

// The colours RenderCV and typer name, with the SGR each resolves to.
// Every value in this block was measured through the vendored Rich, one process
// per colour system — spec 012 delta §2.2, and §2.3 for why one process per
// system is not optional.
var (
	colorRed         = Color{kind: colorStandardKind, code: 31}
	colorGreen       = Color{kind: colorStandardKind, code: 32}
	colorYellow      = Color{kind: colorStandardKind, code: 33}
	colorBlue        = Color{kind: colorStandardKind, code: 34}
	colorMagenta     = Color{kind: colorStandardKind, code: 35}
	colorCyan        = Color{kind: colorStandardKind, code: 36}
	colorBrightBlack = Color{kind: colorStandardKind, code: 90}

	// `purple` is palette 129 and collapses to magenta.
	colorPurple = Color{kind: colorPaletteKind, code: 129, downgrade: 35}
	// `orange4` is palette 94 and collapses to yellow.
	colorOrange4 = Color{kind: colorPaletteKind, code: 94, downgrade: 33}
	// `dodger_blue3` is palette 26 and collapses to **bright** blue.
	colorDodgerBlue3 = Color{kind: colorPaletteKind, code: 26, downgrade: 94}
)

// Style is a Rich style: the attributes, the foreground colour and the
// hyperlink a run of text carries. Only the attributes RenderCV and typer
// actually use are modelled.
type Style struct {
	Bold  bool
	Dim   bool
	Color Color

	// Link is the URL an OSC 8 hyperlink points at — Rich's `Style(link=…)`,
	// which `[link=…]` markup builds (`cli/new_command/print_welcome.py:22`).
	//
	// **It is not SGR at all**, and it is the one styled element whose exact
	// bytes are not reproducible between two runs (spec 012 delta §2.5): the
	// sequence carries an id, and Rich's is `randint(0, 999999)`
	// (`rich/style.py:197`).
	Link string
	// LinkID is that id. It belongs to the style rather than to the run because
	// Rich stores it on the `Style` object, so a link split across two lines
	// writes the **same** id twice.
	LinkID string
}

// The styles RenderCV and typer name, as values.
//
// **Values rather than a name lookup at the call site.** The names are
// upstream's markup spellings and belong in one place; a typo in a call site
// would otherwise be a runtime surprise, and package `cli` may not panic
// (`exitcode_test.go`'s `TestNoPanicInPackageCLI`).
var (
	StyleBold        = Style{Bold: true}
	StyleDim         = Style{Dim: true}
	StyleRed         = Style{Color: colorRed}
	StyleGreen       = Style{Color: colorGreen}
	StyleYellow      = Style{Color: colorYellow}
	StyleBlue        = Style{Color: colorBlue}
	StyleMagenta     = Style{Color: colorMagenta}
	StyleCyan        = Style{Color: colorCyan}
	StyleBrightBlack = Style{Color: colorBrightBlack}
	StylePurple      = Style{Color: colorPurple}
	StyleOrange4     = Style{Color: colorOrange4}
	StyleDodgerBlue3 = Style{Color: colorDodgerBlue3}
	StyleBoldRed     = Style{Bold: true, Color: colorRed}
	StyleBoldGreen   = Style{Bold: true, Color: colorGreen}
	StyleBoldYellow  = Style{Bold: true, Color: colorYellow}
	StyleBoldMagenta = Style{Bold: true, Color: colorMagenta}
	StyleBoldCyan    = Style{Bold: true, Color: colorCyan}
	StyleDimRed      = Style{Dim: true, Color: colorRed}
	StyleDimYellow   = Style{Dim: true, Color: colorYellow}
	StyleDimBlue     = Style{Dim: true, Color: colorBlue}

	// StyleReprNumber is Rich's `repr.number`
	// (`rich/default_styles.py:81`), the one highlighter style reachable from
	// RenderCV's own output — the digit in `RenderCV v2.8` (delta §2.6).
	//
	// Its `italic=False` is not modelled because Rich emits a code only for an
	// attribute that is both set **and** true (`Style._make_ansi_codes`,
	// `rich/style.py:354`), so a false italic writes nothing.
	StyleReprNumber = Style{Bold: true, Color: colorCyan}
)

// LinkStyle is Rich's `Style(link=url)`: a style that carries no colour and no
// attribute, only the OSC 8 hyperlink.
//
// **The id is random, exactly as upstream's is** (`rich/style.py:197`). It is
// not a value a test can pin, and the pty differential normalizes it away
// (delta §2.5).
func LinkStyle(url string) Style {
	return Style{Link: url, LinkID: strconv.Itoa(rand.IntN(1_000_000))}
}

// namedStyles maps each markup spelling upstream writes to its style.
//
// It is a closed set on purpose: a name that is not here is a name nobody
// measured, and guessing its SGR is exactly the kind of plausible-looking wrong
// answer the parity contract is meant to exclude. `cyan bold` is here as well
// as `bold cyan` because `render_command.py:195` writes it in that order.
var namedStyles = map[string]Style{
	"":             {},
	"bold":         StyleBold,
	"dim":          StyleDim,
	"red":          StyleRed,
	"green":        StyleGreen,
	"yellow":       StyleYellow,
	"blue":         StyleBlue,
	"magenta":      StyleMagenta,
	"cyan":         StyleCyan,
	"bright_black": StyleBrightBlack,
	"purple":       StylePurple,
	"orange4":      StyleOrange4,
	"dodger_blue3": StyleDodgerBlue3,
	"bold red":     StyleBoldRed,
	"bold green":   StyleBoldGreen,
	"bold yellow":  StyleBoldYellow,
	"bold magenta": StyleBoldMagenta,
	"bold cyan":    StyleBoldCyan,
	"cyan bold":    StyleBoldCyan,
	"dim red":      StyleDimRed,
	"dim yellow":   StyleDimYellow,
	"dim blue":     StyleDimBlue,
}

// ParseStyle resolves a Rich style name — `bold red`, `bright_black` — to the
// style it names, reporting whether the name is one the port has measured.
//
// It exists for the markup upstream embeds in message text; code in this
// package uses the values above.
func ParseStyle(name string) (Style, bool) {
	style, ok := namedStyles[name]
	return style, ok
}

// IsZero reports whether the style would emit nothing.
func (s Style) IsZero() bool {
	return !s.Bold && !s.Dim && s.Color.kind == colorUnset && s.Link == ""
}

// Render writes one run of text in this style — `Style.render`
// (`rich/style.py:690-714`), which is where every escape sequence RenderCV
// emits is finally assembled.
//
// The order is upstream's and is visible in the bytes: the SGR sequence wraps
// the text, and the **hyperlink wraps that**, so a coloured link reads
// `OSC8 open`, `ESC[…m`, text, `ESC[0m`, `OSC8 close`.
//
// **A terminal with no colour system gets the bare text**, hyperlink included:
// `render` returns early on `color_system is None` (`:706`), which is why
// `TERM=dumb` produces no OSC 8 either.
func (s Style) Render(text string, terminal Terminal) string {
	if text == "" || terminal.System == ColorNone {
		return text
	}
	rendered := text
	if sequence := s.SGR(terminal); sequence != "" {
		rendered = sequence + text + reset
	}
	if s.Link != "" {
		// `f"\x1b]8;id={id};{link}\x1b\\{rendered}\x1b]8;;\x1b\\"` (`:711-713`).
		// **`NO_COLOR` does not remove it**: `Style.without_color` keeps the
		// link and only drops the colour (`rich/style.py:474-490`).
		rendered = "\x1b]8;id=" + s.LinkID + ";" + s.Link + "\x1b\\" + rendered + "\x1b]8;;\x1b\\"
	}
	return rendered
}

// SGR is the escape sequence that opens a run in this style, or the empty
// string when the style has nothing to say on this terminal.
//
// The parameter order is Rich's: attributes first, then the colour
// (`rich/style.py:345-380`), which is what makes `bold green` read `ESC[1;32m`
// and not `ESC[32;1m`.
func (s Style) SGR(terminal Terminal) string {
	// **A terminal with no colour system emits no sequence at all, attributes
	// included.** `Style.render` returns the text untouched when
	// `color_system is None` (`rich/style.py:346-349`), and that is the only
	// gate: `_render_buffer` hands the console's system straight to it
	// (`rich/console.py:2119-2136`). So `bold` does *not* survive a dumb
	// terminal, where it does survive `NO_COLOR` below.
	//
	// Measured: `Console(force_terminal=True, color_system=None)` printing
	// `[bold green]hi` writes `hi`, and upstream's whole CLI emits zero
	// sequences under `TERM=dumb` on a pty — which is what
	// `TestTerminalDetection`'s colourless rows already pin.
	if terminal.System == ColorNone {
		return ""
	}

	parameters := make([]string, 0, 3)
	if s.Bold {
		parameters = append(parameters, "1")
	}
	if s.Dim {
		parameters = append(parameters, "2")
	}
	// `no_color` calls `Segment.remove_color` (`rich/console.py:2127`), which
	// drops the colour and keeps the attributes — so this is a colour switch,
	// not a styling one, and bold survives it.
	if colour := s.colorSGR(terminal); colour != "" && !terminal.NoColor {
		parameters = append(parameters, colour)
	}
	if len(parameters) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parameters, ";") + "m"
}

func (s Style) colorSGR(terminal Terminal) string {
	switch {
	case terminal.System == ColorNone, s.Color.kind == colorUnset:
		return ""
	case s.Color.kind == colorStandardKind:
		return strconv.Itoa(s.Color.code)
	case terminal.System == ColorStandard:
		return strconv.Itoa(s.Color.downgrade)
	default:
		return "38;5;" + strconv.Itoa(s.Color.code)
	}
}

// reset closes every run Rich opens (`ESC[0m`); it emits no other reset form.
const reset = "\x1b[0m"

// Span is a style applied to a half-open range of the plain text, in **rune**
// offsets — Rich's `Span` (`rich/text.py`).
type Span struct {
	Start, End int
	Style      Style
}

// Text is Rich's `Text`: a plain string plus the styles laid over it.
//
// **The style is never inside the string.** Every width the port computes —
// `cellLen`, `pad`, `cutCells`, `chopCells`, `columnWidths` — runs on `Plain`,
// and the spans are carried alongside and moved to follow it. A design that
// threaded escape sequences through the measured string would get every width
// wrong, and cropping such a string would cut an escape sequence in half
// (spec 012 delta §7.1).
type Text struct {
	Plain string
	Spans []Span
}

// StyledText is a whole string under one style, the common case.
func StyledText(text string, style Style) Text {
	if style.IsZero() || text == "" {
		return Text{Plain: text}
	}
	return Text{Plain: text, Spans: []Span{{Start: 0, End: len([]rune(text)), Style: style}}}
}

// PlainText is a string with no styling.
func PlainText(text string) Text {
	return Text{Plain: text}
}

// Append concatenates, shifting the appended spans by the current length.
func (t Text) Append(other Text) Text {
	offset := len([]rune(t.Plain))
	spans := append([]Span(nil), t.Spans...)
	for _, span := range other.Spans {
		spans = append(spans, Span{Start: span.Start + offset, End: span.End + offset, Style: span.Style})
	}
	return Text{Plain: t.Plain + other.Plain, Spans: spans}
}

// StylizeBefore lays a style **underneath** the styles already there — Rich's
// `Text.stylize_before` (`rich/text.py`), which a panel applies to its title so
// the border's colour shows wherever the title's own markup says nothing
// (`rich/panel.py:205-206`).
//
// The span goes to the front of the list, and `styleAt` lets a later span win
// the colour, so `[bold red]Error[/bold red]` inside a `bright_black` border
// keeps its red.
func (t Text) StylizeBefore(start, end int, style Style) Text {
	if start >= end || style.IsZero() {
		return t
	}
	spans := make([]Span, 0, len(t.Spans)+1)
	spans = append(spans, Span{Start: start, End: end, Style: style})
	return Text{Plain: t.Plain, Spans: append(spans, t.Spans...)}
}

// Stylize lays a style over a rune range, the way `Text.stylize` does.
func (t Text) Stylize(start, end int, style Style) Text {
	if start >= end || style.IsZero() {
		return t
	}
	return Text{Plain: t.Plain, Spans: append(append([]Span(nil), t.Spans...), Span{Start: start, End: end, Style: style})}
}

// Slice is the rune range `[start, end)`, with every span clipped to it — the
// operation `Text.divide` performs for each line it produces
// (`rich/text.py:1106-1150`).
func (t Text) Slice(start, end int) Text {
	runes := []rune(t.Plain)
	start = min(max(start, 0), len(runes))
	end = min(max(end, start), len(runes))

	sliced := Text{Plain: string(runes[start:end])}
	for _, span := range t.Spans {
		spanStart, spanEnd := max(span.Start, start), min(span.End, end)
		if spanStart >= spanEnd {
			continue
		}
		sliced.Spans = append(sliced.Spans, Span{
			Start: spanStart - start,
			End:   spanEnd - start,
			Style: span.Style,
		})
	}
	return sliced
}

// Divide splits at the given rune offsets — **the offsets `divideLine` already
// returns** (`internal/cli/panel.go`), which is the whole reason the styled
// panel does not need its own wrapping logic. `Text.divide` slices the plain
// text at exactly these positions and re-attaches each span clipped to the line
// it fell in.
func (t Text) Divide(offsets []int) []Text {
	bounds := make([]int, 0, len(offsets)+2)
	bounds = append(bounds, 0)
	bounds = append(bounds, offsets...)
	bounds = append(bounds, len([]rune(t.Plain)))

	lines := make([]Text, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		lines = append(lines, t.Slice(bounds[i], bounds[i+1]))
	}
	return lines
}

// Truncate crops to a **cell** width, spans following the plain text — the
// order `align_text` uses for a panel title: copy, `truncate(width)`, then
// stylize (`rich/panel.py:174-178`).
//
// It shares `cutCells` with the unstyled path, so a double-width character is
// never cut in half and the styled title crops exactly where the plain one
// does.
func (t Text) Truncate(width int) Text {
	if cellLen(t.Plain) <= width {
		return t
	}
	// **The head is not always a prefix of the plain text.** When the cut lands
	// inside a double-width character, `cutCells` puts a space in its place so
	// neither half is half a glyph — Rich's `set_cell_size` does the same, and
	// the span covers the space, because clipping happens against the *new*
	// length. Measured: `Text("日本語", style="cyan").truncate(3)` renders
	// `ESC[36m日 ESC[0m`.
	head, _ := cutCells(t.Plain, width)
	truncated := t.Slice(0, len([]rune(head)))
	truncated.Plain = head
	return truncated
}

// TruncateEllipsis crops to a cell width and spends the last column on `…` —
// the `overflow="ellipsis"` half of `Text.truncate` (`rich/text.py:875`), where
// `Truncate` above is the `"fold"` half.
//
// **The ellipsis inherits the style of the text it displaced.** `truncate`
// assigns `self.plain`, and the setter only *trims* the spans to the new length
// (`rich/text.py:120-129`); it does not pull one back off the last character.
// Measured on `render --help` at `COLUMNS=24`, where the `dim red` `[required]`
// comes out as `[r…` with the ellipsis inside the run.
func (t Text) TruncateEllipsis(width int) Text {
	runes := []rune(t.Plain)
	if cellLen(t.Plain) <= width {
		return t
	}
	if width <= 1 {
		cut := t.Slice(0, min(max(width, 0), len(runes)))
		cut.Plain = strings.Repeat(ellipsis, max(width, 0))
		return cut
	}
	// `set_cell_size(plain, max_width - 1) + "…"`: the ellipsis costs one column,
	// and a cut inside a double-width character leaves a space in its place.
	head, _ := cutCells(t.Plain, width-1)
	cut := t.Slice(0, min(len([]rune(head))+1, len(runes)))
	cut.Plain = head + ellipsis
	return cut
}

// PadRight extends the text to a cell width with spaces and lays base
// **underneath** the whole of it — the padding included.
//
// That is where a `Text`'s own style reaches: `Text.justify` pads the plain
// string (`rich/text.py:1015-1040`) and `Text.render` then covers everything
// with `self.style`. Measured twice on `--help`, and both would be wrong the
// other way round: an empty metavar cell is four `bold yellow` spaces, and
// `[default: classic]` is `dim` out to the full column.
func (t Text) PadRight(width int, base Style) Text {
	if missing := width - cellLen(t.Plain); missing > 0 {
		t = t.Append(PlainText(strings.Repeat(" ", missing)))
	}
	return t.StylizeBefore(0, len([]rune(t.Plain)), base)
}

// ReplaceAll rewrites every occurrence of old, moving the spans to follow the
// text — what D-009's binary-name rebinding needs once the text is styled.
//
// **A span that covered the replaced token grows with it**: `[yellow]rendercv
// render John_Doe_CV.yaml[/yellow]` stays yellow to its last character after
// `rendercv` becomes `rendercv-go`, and every span after it moves by the three
// columns the longer name took. Substituting the *rendered bytes* instead is the
// phantom of spec 012 delta §6.3.
func (t Text) ReplaceAll(old, replacement string) Text {
	if old == "" || !strings.Contains(t.Plain, old) {
		return t
	}

	runes := []rune(t.Plain)
	oldRunes, newRunes := []rune(old), []rune(replacement)
	var starts []int
	for index := 0; index+len(oldRunes) <= len(runes); {
		if string(runes[index:index+len(oldRunes)]) == old {
			starts = append(starts, index)
			index += len(oldRunes)
			continue
		}
		index++
	}

	// An offset *inside* a replaced token has no image in the new text, so a
	// boundary that lands there is pushed to the token's near edge: a start to
	// where the token begins, an end to where it now ends.
	move := func(offset int, isEnd bool) int {
		delta := 0
		for _, start := range starts {
			switch {
			case offset >= start+len(oldRunes):
				delta += len(newRunes) - len(oldRunes)
			case offset > start:
				if isEnd {
					return start + delta + len(newRunes)
				}
				return start + delta
			}
		}
		return offset + delta
	}

	moved := Text{Plain: strings.ReplaceAll(t.Plain, old, replacement)}
	for _, span := range t.Spans {
		moved.Spans = append(moved.Spans, Span{
			Start: move(span.Start, false),
			End:   move(span.End, true),
			Style: span.Style,
		})
	}
	return moved
}

// Render writes the text with its styles as escape sequences, or as plain text
// when the terminal has no colour and no attributes to carry.
//
// Runs are emitted the way Rich emits them: one opening sequence per maximal
// stretch of identical style, each closed with `ESC[0m`. Overlapping spans
// combine — attributes accumulate and the **last** span wins the colour — which
// is what produces `ESC[1;38;5;26m` for the bold digit inside the coloured
// version string of the welcome line.
func (t Text) Render(terminal Terminal) string {
	runes := []rune(t.Plain)
	if len(runes) == 0 {
		return ""
	}

	var out strings.Builder
	runStart, runStyle := 0, t.styleAt(0)
	flush := func(end int) {
		out.WriteString(runStyle.Render(string(runes[runStart:end]), terminal))
	}

	for i := 1; i < len(runes); i++ {
		style := t.styleAt(i)
		if style == runStyle {
			continue
		}
		flush(i)
		runStart, runStyle = i, style
	}
	flush(len(runes))
	return out.String()
}

// RenderSegments writes the text the way a console writes a renderable: one
// run per span boundary, which is what `Segments` cuts and what every measured
// capture shows.
//
// It is `Render`'s counterpart and the one to reach for. `Render` merges two
// neighbours that resolve to the same style; Rich does not, and the difference
// is visible in the bytes.
func (t Text) RenderSegments(terminal Terminal) string {
	return renderSegments(t.Segments(), terminal)
}

// Segments cuts the text into the runs Rich's `Text.render` yields
// (`rich/text.py:684-748`): **one segment per interval between span
// boundaries**, not one per distinct style.
//
// The difference is visible in the bytes, which is why this is not `Render`.
// Two neighbours that resolve to the same style stay two runs: the `Error`
// panel's title band is `bold red` throughout and still comes out as three
// separately opened and closed runs, because `pad(1)` and the title's own
// markup put boundaries inside it (spec 012 delta §2.4). The progress panel's
// title carries no markup at all, so the same band is **one** run — measured,
// `ESC[90m Your CV is ready ESC[0m`.
func (t Text) Segments() []Segment {
	runes := []rune(t.Plain)
	if len(runes) == 0 {
		return nil
	}

	bounds := []int{0, len(runes)}
	for _, span := range t.Spans {
		bounds = append(bounds, min(max(span.Start, 0), len(runes)), min(max(span.End, 0), len(runes)))
	}
	slices.Sort(bounds)

	segments := make([]Segment, 0, len(bounds))
	for i := 0; i+1 < len(bounds); i++ {
		start, end := bounds[i], bounds[i+1]
		if start == end {
			continue
		}
		segments = append(segments, Segment{Text: string(runes[start:end]), Style: t.styleAt(start)})
	}
	return segments
}

// SplitLines breaks at every newline, which Rich does **before** it wraps
// anything (`rich/text.py:1231`): a newline inside a row is a hard break and
// each piece gets its own bordered line.
func (t Text) SplitLines() []Text {
	runes := []rune(t.Plain)
	var lines []Text
	start := 0
	for i, character := range runes {
		if character != '\n' {
			continue
		}
		lines = append(lines, t.Slice(start, i))
		start = i + 1
	}
	return append(lines, t.Slice(start, len(runes)))
}

// ExpandTabs is `expandTabs` with the spans moved to follow the text it widens
// — Rich's `Text.expand_tabs` (`rich/text.py:817-857`), which rebuilds the
// spans rather than leaving them pointing at the pre-expansion offsets.
//
// **It walks prefixes rather than expanding each span's text on its own.** A tab
// stop is counted in cells from the start of the line, so how far one tab moves
// depends on everything before it — including earlier tabs. Expanding the whole
// prefix each time reuses `expandTabs`'s arithmetic instead of restating it,
// which is what keeps the styled path and the plain one from drifting apart.
func (t Text) ExpandTabs() Text {
	if !strings.ContainsRune(t.Plain, '\t') {
		return t
	}

	runes := []rune(t.Plain)
	expanded := Text{}
	previous := 0
	for i, character := range runes {
		if character != '\t' {
			continue
		}
		expanded = expanded.Append(t.Slice(previous, i))
		// The fully expanded prefix through this tab. Everything before it is
		// already in `expanded`, so what is left over is this tab's own spaces.
		prefix := []rune(expandTabs(string(runes[:i+1])))
		expanded = expanded.Append(PlainText(string(prefix[len([]rune(expanded.Plain)):])))
		previous = i + 1
	}
	return expanded.Append(t.Slice(previous, len(runes)))
}

// styleAt combines every span covering one rune.
func (t Text) styleAt(index int) Style {
	var style Style
	for _, span := range t.Spans {
		if index < span.Start || index >= span.End {
			continue
		}
		style.Bold = style.Bold || span.Style.Bold
		style.Dim = style.Dim || span.Style.Dim
		if span.Style.Color.kind != colorUnset {
			style.Color = span.Style.Color
		}
		// `Style.__add__` is `new_style._link = style._link or self._link`
		// (`rich/style.py:743`), so the later span's link wins and an earlier
		// one survives underneath a span that names none.
		if span.Style.Link != "" {
			style.Link, style.LinkID = span.Style.Link, span.Style.LinkID
		}
	}
	return style
}
