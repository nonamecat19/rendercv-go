package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// durationPattern is the conformance harness's, copied rather than imported —
// `internal/conformance` is behind a build tag and this test is not.
var durationPattern = regexp.MustCompile(`\b\d+(\.\d+)?\s?(ms|s)\b[ \t]*`)

// The panel's geometry is measured from `render_typst_only`'s golden, and this
// is that golden's line 2 with the duration put back.
//
// **The duration column is the part a reader cannot check by eye**: the harness
// erases `\d+(\.\d+)?\s?(ms|s)` *and the spaces after it*, so the golden line is
// two characters longer than the bytes the process actually wrote. That is what
// pins the field at 9 wide, and getting it wrong shifts every column right of it
// while still looking plausible.
func TestPanelMatchesTheGoldenGeometry(t *testing.T) {
	got := Panel("Your CV is ready", []PanelRow{{
		Mark:   "✓",
		Timing: "0.1s",
		Label:  "Generated Typst:",
		Value:  "./rendercv_output/John_Doe_CV.typ",
	}})

	// **Compared after normalization, which is the only honest comparison
	// available here**: the golden stores what the harness saw, and what it saw
	// has the duration and its padding replaced. Writing the pre-normalization
	// bytes by hand means counting spaces, and the first attempt at this test
	// got that count wrong — asserting the author's arithmetic rather than
	// upstream's output.
	want := "╭─ Your CV is ready ───────────────────────────────────────────────────────────╮\n" +
		"│ ✓ <duration> Generated Typst:           ./rendercv_output/John_Doe_CV.typ      │\n" +
		"╰──────────────────────────────────────────────────────────────────────────────╯\n"

	if normalized := durationPattern.ReplaceAllString(got, "<duration> "); normalized != want {
		t.Errorf("panel =\n%q\nwant\n%q", normalized, want)
	}
}

// Every border line is exactly PanelWidth runes wide, whatever the rows hold.
func TestPanelBordersAreAlwaysTheSameWidth(t *testing.T) {
	for _, rows := range [][]PanelRow{
		nil,
		{{Mark: "✓", Timing: "0.1s", Label: "Generated Markdown:", Value: "./out/nested/custom.md"}},
		{{Mark: "✓", Timing: "12.5s", Label: "Generated HTML:", Value: "./a"}},
	} {
		out := Panel("Your CV is ready", rows)
		for i, line := range splitLines(out) {
			if width := runeWidth(line); width != PanelWidth {
				t.Errorf("line %d is %d wide, want %d: %q", i, width, PanelWidth, line)
			}
		}
	}
}

func splitLines(text string) []string {
	var out []string
	current := ""
	for _, r := range text {
		if r == '\n' {
			out = append(out, current)
			current = ""
			continue
		}
		current += string(r)
	}
	return out
}

func runeWidth(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// TestConsoleWidthHonoursColumns is spec 012 gaps.md G-11.
//
// **Rich honours `COLUMNS` even when stdout is a pipe**, and the port printed 80
// unconditionally: 20 columns narrower than upstream at `COLUMNS=100`, and 20
// wider at 60, where every panel overflowed the reader's terminal. Measured
// against the vendored CLI, whose `--help` is 60, 100 and 120 columns wide at
// those settings.
//
// No golden can see this — all of them are captured at 80 — so the width is
// gated here or nowhere.
func TestConsoleWidthHonoursColumns(t *testing.T) {
	cases := []struct {
		columns string
		want    int
	}{
		{columns: "60", want: 60},
		{columns: "100", want: 100},
		{columns: "120", want: 120},
		{columns: "80", want: 80},
		// Rich's own `int()` guard: anything unparseable falls back.
		{columns: "", want: PanelWidth},
		{columns: "not-a-number", want: PanelWidth},
		{columns: "0", want: PanelWidth},
		{columns: "-5", want: PanelWidth},
	}

	for _, row := range cases {
		t.Run("COLUMNS="+row.columns, func(t *testing.T) {
			t.Setenv("COLUMNS", row.columns)
			if got := ConsoleWidth(); got != row.want {
				t.Errorf("ConsoleWidth() = %d, want %d", got, row.want)
			}
		})
	}
}

// TestPanelTracksTheConsoleWidth is the same fact one layer up: the box is laid
// out to the console, not to a constant.
func TestPanelTracksTheConsoleWidth(t *testing.T) {
	for _, width := range []int{60, 80, 100} {
		t.Setenv("COLUMNS", strconv.Itoa(width))
		panel := Panel("Error", []PanelRow{{Text: "a message"}})
		for i, line := range strings.Split(strings.TrimRight(panel, "\n"), "\n") {
			if got := len([]rune(line)); got != width {
				t.Errorf("COLUMNS=%d: line %d is %d wide", width, i, got)
			}
		}
	}
}

// TestPanelBreaksRowsOnNewlines is Rich's hard break: a `\n` inside a row is a
// line boundary, not a character to print. `new` reaches it because it turns
// only spaces into underscores (`cli/new_command/new_command.py:81`), so a name
// with a newline in it produces a file name with a newline in it, and that file
// name goes into two rows of the "Get started" panel.
func TestPanelBreaksRowsOnNewlines(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	panel := Panel("Get started", []PanelRow{{Text: "✓ Created: ./line1\nline2_CV.yaml"}})

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("the panel has %d lines, want 4", len(lines))
	}
	for i, line := range lines {
		if got := len([]rune(line)); got != 80 {
			t.Errorf("line %d is %d wide, want 80: %q", i, got, line)
		}
	}
	if !strings.HasPrefix(lines[1], "│ ✓ Created: ./line1 ") {
		t.Errorf("the first segment is %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "│ line2_CV.yaml ") {
		t.Errorf("the second segment is %q", lines[2])
	}
}
