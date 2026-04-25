package cli_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// style resolves a markup name for a test table, failing rather than panicking
// on a name the port has not measured.
func style(t *testing.T, name string) cli.Style {
	t.Helper()
	resolved, ok := cli.ParseStyle(name)
	if !ok {
		t.Fatalf("ParseStyle(%q) is unknown", name)
	}
	return resolved
}

// readable spells the escape byte out so a failure is legible.
func readable(text string) string {
	return strings.ReplaceAll(text, "\x1b", "ESC")
}

var (
	truecolorTerminal = cli.Terminal{IsTerminal: true, System: cli.ColorTruecolor}
	eightBitTerminal  = cli.Terminal{IsTerminal: true, System: cli.ColorEightBit}
	standardTerminal  = cli.Terminal{IsTerminal: true, System: cli.ColorStandard}
	noColorTerminal   = cli.Terminal{IsTerminal: true, System: cli.ColorEightBit, NoColor: true}
	dumbTerminal      = cli.Terminal{IsTerminal: true, System: cli.ColorNone}
	pipe              = cli.Terminal{}
)

// TestStyleSGR pins spec 012 delta §2.2: the sequence each style name Rich
// resolves to, per colour system.
//
// Every expectation was measured through the vendored Rich, **one process per
// colour system** — `Style.render` caches its SGR on the Style object after the
// first call (`rich/style.py:350`), so measuring several systems in one process
// reports the first one's answer for all of them and made `purple` read
// `ESC[35m` under truecolor (delta §2.3).
func TestStyleSGR(t *testing.T) {
	tests := []struct {
		style string
		// The three systems, in the order truecolor, 256, standard.
		truecolor, eightBit, standard string
	}{
		{style: "bright_black", truecolor: "ESC[90m", eightBit: "ESC[90m", standard: "ESC[90m"},
		{style: "green", truecolor: "ESC[32m", eightBit: "ESC[32m", standard: "ESC[32m"},
		{style: "bold green", truecolor: "ESC[1;32m", eightBit: "ESC[1;32m", standard: "ESC[1;32m"},
		{style: "magenta", truecolor: "ESC[35m", eightBit: "ESC[35m", standard: "ESC[35m"},
		{style: "cyan", truecolor: "ESC[36m", eightBit: "ESC[36m", standard: "ESC[36m"},
		{style: "bold cyan", truecolor: "ESC[1;36m", eightBit: "ESC[1;36m", standard: "ESC[1;36m"},
		{style: "red", truecolor: "ESC[31m", eightBit: "ESC[31m", standard: "ESC[31m"},
		{style: "bold red", truecolor: "ESC[1;31m", eightBit: "ESC[1;31m", standard: "ESC[1;31m"},
		{style: "bold yellow", truecolor: "ESC[1;33m", eightBit: "ESC[1;33m", standard: "ESC[1;33m"},
		{style: "dim", truecolor: "ESC[2m", eightBit: "ESC[2m", standard: "ESC[2m"},
		{style: "dim red", truecolor: "ESC[2;31m", eightBit: "ESC[2;31m", standard: "ESC[2;31m"},
		{style: "dim yellow", truecolor: "ESC[2;33m", eightBit: "ESC[2;33m", standard: "ESC[2;33m"},
		{style: "dim blue", truecolor: "ESC[2;34m", eightBit: "ESC[2;34m", standard: "ESC[2;34m"},

		// The three palette colours, which are the only styles that **move**
		// when the terminal has sixteen colours instead of 256.
		{style: "purple", truecolor: "ESC[38;5;129m", eightBit: "ESC[38;5;129m", standard: "ESC[35m"},
		{style: "orange4", truecolor: "ESC[38;5;94m", eightBit: "ESC[38;5;94m", standard: "ESC[33m"},
		{style: "dodger_blue3", truecolor: "ESC[38;5;26m", eightBit: "ESC[38;5;26m", standard: "ESC[94m"},
	}

	for _, test := range tests {
		t.Run(test.style, func(t *testing.T) {
			resolved := style(t, test.style)
			for _, system := range []struct {
				name     string
				terminal cli.Terminal
				want     string
			}{
				{"truecolor", truecolorTerminal, test.truecolor},
				{"256", eightBitTerminal, test.eightBit},
				{"standard", standardTerminal, test.standard},
			} {
				if got := readable(resolved.SGR(system.terminal)); got != system.want {
					t.Errorf("%s: SGR = %q, want %q", system.name, got, system.want)
				}
			}
		})
	}
}

// TestStyleSGRWhenColourIsOff pins the two switches that are **not** the same
// switch, delta §3.3. `NO_COLOR` drops the colour and keeps the attributes; a
// dumb terminal and a pipe emit nothing at all.
func TestStyleSGRWhenColourIsOff(t *testing.T) {
	tests := []struct {
		name     string
		style    string
		terminal cli.Terminal
		want     string
	}{
		{name: "NO_COLOR keeps bold", style: "bold green", terminal: noColorTerminal, want: "ESC[1m"},
		{name: "NO_COLOR keeps dim", style: "dim red", terminal: noColorTerminal, want: "ESC[2m"},
		{name: "NO_COLOR drops a bare colour", style: "green", terminal: noColorTerminal, want: ""},
		{name: "NO_COLOR drops a palette colour", style: "purple", terminal: noColorTerminal, want: ""},
		// **A dumb terminal drops the attributes too**, which is the one place
		// this table used to disagree with its own heading. `Style.render`
		// returns the text untouched when the colour system is `None`
		// (`rich/style.py:346-349`), so there is no sequence to keep the bold
		// in. Measured through the vendored Rich —
		// `Console(force_terminal=True, color_system=None)` printing
		// `[bold green]hi` writes `hi` — and end to end by
		// `TestTerminalDetection`'s colourless rows, which require upstream to
		// emit **zero** sequences under `TERM=dumb` on a pty.
		{name: "a dumb terminal drops bold as well", style: "bold green", terminal: dumbTerminal, want: ""},
		{name: "a dumb terminal has no colour", style: "green", terminal: dumbTerminal, want: ""},
		{name: "a pipe emits nothing at all", style: "bold red", terminal: pipe, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := style(t, test.style)
			if got := readable(resolved.SGR(test.terminal)); got != test.want {
				t.Errorf("SGR = %q, want %q", got, test.want)
			}
		})
	}
}

// TestParseStyleRefusesAnUnmeasuredName: the set of names is closed, because a
// name nobody measured has no SGR the port can claim parity on.
func TestParseStyleRefusesAnUnmeasuredName(t *testing.T) {
	for _, name := range []string{"chartreuse1", "bold", "italic", "green on red", "bold  green"} {
		_, ok := cli.ParseStyle(name)
		if name == "bold" {
			if !ok {
				t.Errorf("ParseStyle(%q) should be known", name)
			}
			continue
		}
		if ok {
			t.Errorf("ParseStyle(%q) = known, want unknown", name)
		}
	}
}

// TestTextRenderEmitsOneRunPerStyle pins delta §2.4: Rich opens a run per
// maximal stretch of identical style and closes each with `ESC[0m`.
func TestTextRenderEmitsOneRunPerStyle(t *testing.T) {
	tests := []struct {
		name string
		text cli.Text
		want string
	}{
		{
			name: "plain text emits nothing",
			text: cli.PlainText("Generated Typst:"),
			want: "Generated Typst:",
		},
		{
			name: "one styled run",
			text: cli.StyledText("│", style(t, "bright_black")),
			want: "ESC[90m│ESC[0m",
		},
		{
			name: "the trailing padding of a styled field is inside the run",
			text: cli.StyledText("16 ms   ", style(t, "bold green")),
			want: "ESC[1;32m16 ms   ESC[0m",
		},
		{
			name: "styled and unstyled alternate",
			text: cli.StyledText("✓", style(t, "green")).
				Append(cli.PlainText(" done ")).
				Append(cli.StyledText("./out.typ", style(t, "purple"))),
			want: "ESC[32m✓ESC[0m done ESC[38;5;129m./out.typESC[0m",
		},
		{
			// The welcome line, delta §2.6: Rich's own highlighter lays `bold`
			// over the digit inside an already-coloured span, and the two
			// combine into one sequence rather than nesting.
			name: "an overlapping span combines rather than nests",
			text: cli.StyledText("v2.8", style(t, "dodger_blue3")).
				Stylize(3, 4, style(t, "bold")),
			want: "ESC[38;5;26mv2.ESC[0mESC[1;38;5;26m8ESC[0m",
		},
		{
			name: "an empty text renders as nothing",
			text: cli.PlainText(""),
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := readable(test.text.Render(truecolorTerminal)); got != test.want {
				t.Errorf("Render() =\n  %q\nwant\n  %q", got, test.want)
			}
		})
	}
}

// TestTextRenderIsPlainWithoutATerminal is the property the 42 goldens depend
// on: in the environment they are captured in — a pipe, `NO_COLOR=1`,
// `TERM=dumb` — a styled Text is byte-identical to its plain string.
func TestTextRenderIsPlainWithoutATerminal(t *testing.T) {
	text := cli.StyledText("│", style(t, "bright_black")).
		Append(cli.StyledText(" ✓ ", style(t, "green"))).
		Append(cli.StyledText("./out.typ", style(t, "purple")))

	for _, terminal := range []struct {
		name string
		term cli.Terminal
	}{
		{"a pipe", pipe},
		{"a dumb terminal", dumbTerminal},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			if got := text.Render(terminal.term); got != text.Plain {
				t.Errorf("Render() = %q, want the plain text %q", readable(got), text.Plain)
			}
		})
	}
}

// TestTextDivideSlicesSpans pins delta §7.1.3: `divideLine`'s rune offsets are
// the offsets a span is sliced at, so the styled panel reuses the wrapping the
// unstyled one already does.
func TestTextDivideSlicesSpans(t *testing.T) {
	// "abcdefghij", with `bold red` over "cdefg" — a span that crosses both
	// break points and so has to be clipped twice.
	text := cli.PlainText("abcdefghij").Stylize(2, 7, style(t, "bold red"))

	lines := text.Divide([]int{4, 8})
	want := []string{
		"abESC[1;31mcdESC[0m",
		"ESC[1;31mefgESC[0mh",
		"ij",
	}
	if len(lines) != len(want) {
		t.Fatalf("Divide() produced %d lines, want %d", len(lines), len(want))
	}
	for i, line := range lines {
		if got := readable(line.Render(truecolorTerminal)); got != want[i] {
			t.Errorf("line %d = %q, want %q", i, got, want[i])
		}
	}
}

// TestTextTruncateClipsTheSpanWithThePlainText pins delta §7.1.2, the panel
// title crop: `align_text` truncates the plain text and the span follows, which
// is what stops a crop from cutting an escape sequence in half.
func TestTextTruncateClipsTheSpanWithThePlainText(t *testing.T) {
	tests := []struct {
		name  string
		text  cli.Text
		width int
		want  string
	}{
		{
			name:  "a title cropped mid-word keeps its style",
			text:  cli.StyledText(" Error ", style(t, "bold red")),
			width: 5,
			want:  "ESC[1;31m ErroESC[0m",
		},
		{
			name:  "a title that fits is untouched",
			text:  cli.StyledText(" Error ", style(t, "bold red")),
			width: 20,
			want:  "ESC[1;31m Error ESC[0m",
		},
		{
			name:  "cropping to nothing emits nothing",
			text:  cli.StyledText(" Error ", style(t, "bold red")),
			width: 0,
			want:  "",
		},
		{
			// A cut inside a double-width character puts a space in its place,
			// and the style covers the space — measured on Rich itself, whose
			// `set_cell_size` pads and whose span clipping happens against the
			// padded length.
			name:  "a double width character is padded, not cut in half",
			text:  cli.StyledText("日本語", style(t, "cyan")),
			width: 3,
			want:  "ESC[36m日 ESC[0m",
		},
		{
			name:  "a clean double width boundary does not pad",
			text:  cli.StyledText("日本語", style(t, "cyan")),
			width: 2,
			want:  "ESC[36m日ESC[0m",
		},
		{
			name:  "the pad lands after two full characters",
			text:  cli.StyledText("日本語", style(t, "cyan")),
			width: 5,
			want:  "ESC[36m日本 ESC[0m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.text.Truncate(test.width)
			if rendered := readable(got.Render(truecolorTerminal)); rendered != test.want {
				t.Errorf("Truncate(%d).Render() = %q, want %q", test.width, rendered, test.want)
			}
		})
	}
}
