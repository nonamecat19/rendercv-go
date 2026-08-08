package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelpPageMatchesGolden is spec 012 help.md acceptance criteria 1 and 2, as
// far as they can be met.
//
// **It compares against the goldens directly rather than through the parity
// suite**, and it states, per page, how many lines are allowed to differ and
// why. Every allowed difference carries the binary name: the page wraps its
// prose to the console before the harness ever sees it, and `rendercv-go` is
// three characters longer than `rendercv`, so a line breaks somewhere else. The
// harness rebinds the token and re-pads a shortened row (D-009), which cannot
// undo a re-wrap.
//
// The number in each row is therefore a **budget**, not a target: a geometry
// regression shows up as a mismatch on a line with no binary name in it, and
// any change to the count fails here rather than passing quietly.
func TestHelpPageMatchesGolden(t *testing.T) {
	cases := []struct {
		command string
		golden  string
		// allowed is how many lines may differ, all of which must carry the
		// binary name.
		allowed int
	}{
		{command: "", golden: "cli_help", allowed: 2},
		{command: "render", golden: "cli_render_help", allowed: 2},
		{command: "new", golden: "cli_new_help", allowed: 2},
		{command: "create-theme", golden: "cli_create_theme_help", allowed: 0},
	}

	for _, row := range cases {
		t.Run(row.golden, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "golden", row.golden, "stdout.txt")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the golden: %v", err)
			}

			got := strings.Split(rebindToUpstream(HelpPage(row.command)), "\n")
			want := strings.Split(string(raw), "\n")

			if len(got) != len(want) {
				t.Fatalf("page is %d lines, golden is %d", len(got), len(want))
			}

			differing := 0
			for i := range want {
				// Trailing space is what the harness reconstructs for a row it
				// shortened, so it is compared separately below.
				if strings.TrimRight(got[i], " ") == strings.TrimRight(want[i], " ") {
					continue
				}
				differing++
				if !strings.Contains(want[i], upstreamBinaryName+" ") {
					t.Errorf("line %d differs and carries no binary name:\n got  %q\n want %q",
						i, got[i], want[i])
				}
			}
			if differing != row.allowed {
				t.Errorf("%d lines differ, expected exactly %d", differing, row.allowed)
			}
		})
	}
}

// TestHelpPageIsFullWidth pins the part of the layout the binary name cannot
// reach: every line of every page is exactly the console width, panels and
// `Padding` regions alike.
//
// **A `Padding` region is painted, not skipped** — the line above the usage is
// eighty spaces rather than nothing — and getting that wrong would shift no
// text at all while differing from the golden on five lines per page.
func TestHelpPageIsFullWidth(t *testing.T) {
	for _, command := range []string{"", "render", "new", "create-theme"} {
		page := HelpPage(command)
		lines := strings.Split(strings.TrimSuffix(page, "\n\n"), "\n")
		for i, line := range lines {
			if width := len([]rune(line)); width != PanelWidth {
				t.Errorf("%q line %d is %d wide, want %d: %q", command, i, width, PanelWidth, line)
			}
		}
	}
}

// TestHelpPageEndsWithABlankLine pins the trailing bytes: every golden ends
// `╯\n\n`.
func TestHelpPageEndsWithABlankLine(t *testing.T) {
	for _, command := range []string{"", "render", "new", "create-theme"} {
		if page := HelpPage(command); !strings.HasSuffix(page, "╯\n\n") {
			t.Errorf("%q does not end with a panel and a blank line: %q",
				command, page[max(0, len(page)-8):])
		}
	}
}

// rebindToUpstream is the comparison direction of
// `conformance.RebindBinaryName`: the port prints its own name, the golden
// carries upstream's, and a row shortened by the substitution has its padding
// restored.
//
// **It re-pads every line, where the harness re-pads only bordered rows.** A
// help page's `Padding` regions — the usage line and the description — are
// painted to the console width too, so they are just as width-sensitive as a
// panel row and the harness's `isPanelRow` guard does not reach them. Extending
// it there is the change `specs/012-cli/help.md` §7 recommends and does not
// make: the harness is the instrument every other case is measured by.
func rebindToUpstream(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		width := len([]rune(line))
		rebound := strings.ReplaceAll(line, portBinaryName+" ", upstreamBinaryName+" ")
		if rebound == line {
			continue
		}
		if missing := width - len([]rune(rebound)); missing > 0 {
			if border := strings.HasSuffix(rebound, "│"); border {
				rebound = strings.TrimSuffix(rebound, "│") +
					strings.Repeat(" ", missing) + "│"
			} else {
				rebound += strings.Repeat(" ", missing)
			}
		}
		lines[i] = rebound
	}
	return strings.Join(lines, "\n")
}
