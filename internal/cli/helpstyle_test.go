package cli

import (
	"strings"
	"testing"
)

// The styled `--help` page — spec 012 delta §4, unit F.
//
// Every expectation below is **verbatim from a pty capture** of the vendored
// CLI at `COLUMNS=80`, `TERM=xterm-256color`, `COLORTERM=truecolor`. The
// end-to-end comparison is `TestHelpColour` in the pty differential; these are
// the rules that differential cannot see, because every line carrying them also
// carries the binary name and is therefore in D-010's re-wrapped region.

// colourTerminal is a terminal with the 256-colour palette, which is what the
// pty differential drives both sides at. None of the styles `--help` uses is a
// palette colour, so the same bytes come out of `ColorStandard` too.
var colourTerminal = Terminal{IsTerminal: true, System: ColorEightBit}

// TestHelpModelStylesAreKnown is what keeps `helpStyle`'s silent fallback
// honest.
//
// A style name that resolves to nothing renders a plain page rather than
// failing, which is the right behavior for generated data at run time and the
// wrong one for a build: a submodule bump that introduces `bold blue` would
// otherwise drop a colour and no test would notice. This is the build noticing.
func TestHelpModelStylesAreKnown(t *testing.T) {
	model := loadHelpModel()

	check := func(where string, strings ...helpString) {
		t.Helper()
		for _, text := range strings {
			if _, ok := helpStyleOf(text.Style); !ok {
				t.Errorf("%s: unknown base style %q", where, text.Style)
			}
			for _, span := range text.Spans {
				if _, ok := helpStyleOf(span.Style); !ok {
					t.Errorf("%s: unknown span style %q", where, span.Style)
				}
			}
		}
	}

	commands := map[string]helpCommand{"": model.Root}
	for name, command := range model.Commands {
		commands[name] = command
	}
	for name, command := range commands {
		check(name+" usage", command.Usage)
		check(name+" description", command.Description)
		for _, param := range append(append([]helpParam{}, command.Arguments...), command.Options...) {
			check(name+" "+param.Long.Text, param.Long, param.Short,
				param.SecondaryLong, param.SecondaryShort, param.Metavar)
			check(name+" "+param.Long.Text+" help", param.Help...)
		}
		for _, sub := range command.Subcommands {
			check(name+" "+sub.Name, sub.Help)
		}
	}
}

// TestCommandsPanelRunStructure pins the two rules that are visible only in the
// `Commands` panel, both measured on `rendercv --help`.
//
//  1. **The first column's style is the column's, so it covers the padding cell
//     beside the name** — and the cell and that padding cell are two runs, not
//     one, because rich merges neither (`STYLE_COMMANDS_TABLE_FIRST_COLUMN`,
//     `typer/rich_utils.py:487-491`).
//  2. **A line the cell does not reach is a single run across the whole
//     column** — thirteen `bold cyan` spaces, content and padding together,
//     because that line comes from `Segment.set_shape` filling the row out to
//     its tallest cell rather than from the cell itself.
//
// Only the first line of the `create-theme` row is quoted whole: every other
// line of the panel carries the binary name and is D-010's, so the second line
// is pinned by the prefix that precedes the name.
func TestCommandsPanelRunStructure(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	page := HelpPage("", colourTerminal)
	lines := strings.Split(page, "\n")

	const (
		first = "\x1b[2m│\x1b[0m \x1b[1;36mcreate-theme\x1b[0m\x1b[1;36m \x1b[0m " +
			"Create a custom theme folder with Typst templates to           \x1b[2m│\x1b[0m"
		continuationPrefix = "\x1b[2m│\x1b[0m \x1b[1;36m             \x1b[0m "
	)

	index := -1
	for i, line := range lines {
		if strings.Contains(line, "create-theme") && strings.Contains(line, "Create a custom") {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("no create-theme row in the page:\n%s", spelled(page))
	}
	if lines[index] != first {
		t.Errorf("the row's first line differs from upstream's\n got %q\nwant %q",
			spelled(lines[index]), spelled(first))
	}
	if index+1 >= len(lines) || !strings.HasPrefix(lines[index+1], continuationPrefix) {
		t.Errorf("the row's continuation does not open with one run of thirteen spaces:\n got %q\nwant prefix %q",
			spelled(lines[min(index+1, len(lines)-1)]), spelled(continuationPrefix))
	}
}

// TestUsageBlockRuns pins the usage `Padding`, which is the one region printed
// under a style of its own (`console.print(Padding(…, 1), style="bold")`,
// `typer/rich_utils.py:552-554`).
//
// Two things a plausible implementation gets wrong, both measured:
// the blank lines above and below carry the `bold` too — they are eighty *bold*
// spaces, not eighty plain ones — and the line's own padding and fill are runs
// of their own, so `bold` closes and reopens around them rather than covering
// the line in one.
func TestUsageBlockRuns(t *testing.T) {
	t.Setenv("COLUMNS", "80")

	lines := strings.Split(HelpPage("create-theme", colourTerminal), "\n")
	if len(lines) < 3 {
		t.Fatalf("page is %d lines", len(lines))
	}

	wantBlank := "\x1b[1m" + strings.Repeat(" ", 80) + "\x1b[0m"
	if lines[0] != wantBlank {
		t.Errorf("the line above the usage is not eighty bold spaces:\n got %q\nwant %q",
			spelled(lines[0]), spelled(wantBlank))
	}
	if lines[2] != wantBlank {
		t.Errorf("the line below the usage is not eighty bold spaces:\n got %q",
			spelled(lines[2]))
	}

	// The binary name is the port's, so the fill is three columns shorter than
	// upstream's twenty-nine (D-010). Everything else is upstream's, run for
	// run.
	const usage = "Usage: rendercv-go create-theme [OPTIONS] THEME_NAME"
	want := "\x1b[1m \x1b[0m\x1b[1;33mUsage: \x1b[0m" +
		"\x1b[1m" + strings.TrimPrefix(usage, "Usage: ") + "\x1b[0m" +
		"\x1b[1m" + strings.Repeat(" ", 78-len(usage)) + "\x1b[0m\x1b[1m \x1b[0m"
	if lines[1] != want {
		t.Errorf("the usage line differs\n got %q\nwant %q", spelled(lines[1]), spelled(want))
	}
}

// TestHelpPageIsPlainWithoutATerminal is the invariant every golden depends on:
// the same page, written for a terminal that has no colour, is the bytes the
// 42 committed cases were captured as.
func TestHelpPageIsPlainWithoutATerminal(t *testing.T) {
	for _, command := range []string{"", "render", "new", "create-theme"} {
		page := HelpPage(command, Terminal{})
		if strings.ContainsRune(page, '\x1b') {
			t.Errorf("%q carries an escape sequence with no terminal", command)
		}
	}
}

// spelled writes the escape byte out so a failure is legible. Its twin in
// `style_test.go` belongs to the external test package, which cannot reach the
// unexported model this file drives.
func spelled(text string) string {
	return strings.ReplaceAll(text, "\x1b", "ESC")
}
