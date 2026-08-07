package cli

import (
	"regexp"
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
