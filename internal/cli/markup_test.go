package cli_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// linkID normalizes the random id Rich puts in an OSC 8 sequence
// (`rich/style.py:197`), which differs between two runs of the same binary and
// is the one styled element whose exact bytes are not reproducible at all
// (spec 012 delta §2.5).
var linkID = regexp.MustCompile(`id=\d+;`)

func normalizedLinks(text string) string {
	return linkID.ReplaceAllString(text, "id=<n>;")
}

// render is a markup string as it reaches the terminal.
//
// Two things it has to do in the right order, both of which the bytes show.
// **The text is split on its newlines first** — Rich breaks a `Text` at every
// `\n` before it wraps or renders anything (`rich/text.py:1231`), so a span
// crossing a line break is opened and closed once per line. And each line is
// then cut into **one run per span boundary** (`rich/text.py:742-774`), not one
// per distinct style, which is why `Segments` and not `Render`.
func render(markup string, terminal cli.Terminal) string {
	var lines []string
	for _, line := range cli.Markup(markup).SplitLines() {
		lines = append(lines, line.RenderSegments(terminal))
	}
	return normalizedLinks(readable(strings.Join(lines, "\n")))
}

// TestMarkupRuns pins the run structure Rich produces for the markup RenderCV
// writes — every expectation measured from the vendored CLI on a pty at
// `COLUMNS=80`, `TERM=xterm-256color`.
//
// **The runs are the contract, not just the colours.** Three of these cases
// come out as several separately opened and closed runs of the *same* style,
// because a span boundary ends a run whether or not the style changes.
func TestMarkupRuns(t *testing.T) {
	tests := []struct {
		name   string
		markup string
		want   string
	}{{
		name:   "a closed tag is one run",
		markup: "[green]✓[/green] Created your YAML input file: [purple]./JohnDoe_CV.yaml[/purple]",
		want: "ESC[32m✓ESC[0m Created your YAML input file: " +
			"ESC[38;5;129m./JohnDoe_CV.yamlESC[0m",
	}, {
		// `create_theme_command.py:46-47` opens `[purple]` on one line and
		// closes it on the next, so `pop_style` matches the *nearest* open tag
		// and the outer one keeps running. Measured: `2. Edit`, the path and
		// ` to:` are three separate purple runs.
		name:   "an unclosed tag runs to the end of the text",
		markup: "in [purple]./t/\n2. Edit [purple]./t/x[/purple] to:",
		want: "in ESC[38;5;129m./t/ESC[0m\nESC[38;5;129m2. Edit ESC[0m" +
			"ESC[38;5;129m./t/xESC[0mESC[38;5;129m to:ESC[0m",
	}, {
		// The last span covering a character wins its colour
		// (`rich/markup.py:230` decides the order, `rich/text.py:758-766` the
		// combination), so `[cyan]` inside an open `[purple]` reads cyan.
		name:   "a later tag wins the colour without closing the earlier one",
		markup: "[purple]set this:\n[cyan]  design:",
		want:   "ESC[38;5;129mset this:ESC[0m\nESC[36m  design:ESC[0m",
	}, {
		name:   "a name nobody measured styles nothing",
		markup: "[qq]John[/qq]_CV.yaml",
		want:   "John_CV.yaml",
	}, {
		name:   "an escaped brace is written through",
		markup: `\[green]John`,
		want:   "[green]John",
	}, {
		name:   "a bracket that opens no tag is text",
		markup: "[Bold]John",
		want:   "[Bold]John",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := render(test.markup, eightBitTerminal); got != test.want {
				t.Errorf("Markup(%q) =\n %q\nwant %q", test.markup, got, test.want)
			}
		})
	}
}

// TestMarkupLink pins OSC 8, which is not SGR and does not go through the
// colour system's downgrade table.
//
// The shape is `Style.render`'s (`rich/style.py:710-713`): the hyperlink wraps
// whatever the SGR sequence produced, so a link with no colour carries no
// `ESC[` at all — measured on the welcome panel, whose URLs are plain text
// inside a hyperlink while their titles are `bold cyan` outside one.
func TestMarkupLink(t *testing.T) {
	const markup = "[bold cyan]RenderCV App:  [/bold cyan] [link=https://rendercv.com]https://rendercv.com[/link]"

	tests := []struct {
		name     string
		terminal cli.Terminal
		want     string
	}{{
		name:     "a link is not a colour",
		terminal: eightBitTerminal,
		want: "ESC[1;36mRenderCV App:  ESC[0m " +
			"ESC]8;id=<n>;https://rendercv.comESC\\https://rendercv.comESC]8;;ESC\\",
	}, {
		// `Style.without_color` keeps the link and drops only the colour
		// (`rich/style.py:474-490`), so `NO_COLOR=1` leaves both the bold and
		// the hyperlink standing.
		name:     "NO_COLOR keeps the link and the bold",
		terminal: noColorTerminal,
		want: "ESC[1mRenderCV App:  ESC[0m " +
			"ESC]8;id=<n>;https://rendercv.comESC\\https://rendercv.comESC]8;;ESC\\",
	}, {
		// `Style.render` returns the bare text when the colour system is None
		// (`:706`), before it ever reaches the hyperlink branch.
		name:     "a dumb terminal gets no link either",
		terminal: dumbTerminal,
		want:     "RenderCV App:   https://rendercv.com",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := render(markup, test.terminal); got != test.want {
				t.Errorf("link markup =\n %q\nwant %q", got, test.want)
			}
		})
	}
}

// TestMarkupLinkIDsDiffer records that the id is per link, not per binary: two
// hyperlinks in one panel carry different ids, exactly as upstream's do
// (measured: 924555, 895170, 269717, 344114 in one `new`).
func TestMarkupLinkIDsDiffer(t *testing.T) {
	text := cli.Markup("[link=https://a]a[/link] [link=https://b]b[/link]")
	rendered := text.RenderSegments(eightBitTerminal)

	ids := regexp.MustCompile(`id=(\d+);`).FindAllStringSubmatch(rendered, -1)
	if len(ids) != 2 {
		t.Fatalf("rendered %q with %d ids, want 2", readable(rendered), len(ids))
	}
	if ids[0][1] == ids[1][1] {
		t.Errorf("both links carry id %s; Rich draws a fresh randint per style", ids[0][1])
	}
}

// TestHighlightRepr pins delta §2.6: `rich.print` runs its `ReprHighlighter`
// over a bare string, and the one match RenderCV produces is the digit at the
// end of the version.
//
// **The bold lands on the number and the colour stays the markup's.**
// `render_str` highlights first and copies the markup's styles on top
// (`rich/console.py:1468-1472`), so the highlighter's own cyan loses to
// `dodger_blue3` while its bold survives — measured,
// `ESC[38;5;26mRenderCV v2.ESC[0mESC[1;38;5;26m8ESC[0m`.
func TestHighlightRepr(t *testing.T) {
	tests := []struct {
		name   string
		markup string
		want   string
	}{{
		name:   "the version's last digit is bold",
		markup: "Welcome to [dodger_blue3]RenderCV v2.8[/dodger_blue3]!",
		want:   "Welcome to ESC[38;5;26mRenderCV v2.ESC[0mESC[1;38;5;26m8ESC[0m!",
	}, {
		// `(?<!\w)` rejects a digit that follows a letter, and the engine then
		// **retries at the next position** rather than giving up on the line —
		// which is why `8` is highlighted and `2` is not.
		name:   "a digit glued to a word is not a number",
		markup: "v2 and 2",
		want:   "v2 and ESC[1;36m2ESC[0m",
	}, {
		name:   "nothing to highlight is left alone",
		markup: "Next steps:",
		want:   "Next steps:",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := readable(cli.HighlightRepr(cli.Markup(test.markup)).RenderSegments(eightBitTerminal))
			if got != test.want {
				t.Errorf("HighlightRepr(%q) =\n %q\nwant %q", test.markup, got, test.want)
			}
		})
	}
}

// TestMarkupLeavesThePlainTextAlone is the invariant the whole span model rests
// on: the tags are gone from the measured string, so every width the port
// computes still runs on what the reader sees.
func TestMarkupLeavesThePlainTextAlone(t *testing.T) {
	text := cli.Markup("[green]✓[/green] Created your custom theme: [purple]./mytheme[/purple]")
	const want = "✓ Created your custom theme: ./mytheme"

	if text.Plain != want {
		t.Errorf("Plain = %q, want %q", text.Plain, want)
	}
	if strings.Contains(text.Plain, "[") {
		t.Errorf("Plain still carries markup: %q", text.Plain)
	}
}
