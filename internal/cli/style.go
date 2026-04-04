package cli

import (
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

// Style is a Rich style: the attributes and the foreground colour a run of text
// carries. Only the attributes RenderCV and typer actually use are modelled.
type Style struct {
	Bold  bool
	Dim   bool
	Color Color
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
)

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
	return !s.Bold && !s.Dim && s.Color.kind == colorUnset
}

// SGR is the escape sequence that opens a run in this style, or the empty
// string when the style has nothing to say on this terminal.
//
// The parameter order is Rich's: attributes first, then the colour
// (`rich/style.py:345-380`), which is what makes `bold green` read `ESC[1;32m`
// and not `ESC[32;1m`.
func (s Style) SGR(terminal Terminal) string {
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
		segment := string(runes[runStart:end])
		if sequence := runStyle.SGR(terminal); sequence != "" {
			out.WriteString(sequence)
			out.WriteString(segment)
			out.WriteString(reset)
			return
		}
		out.WriteString(segment)
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
	}
	return style
}
