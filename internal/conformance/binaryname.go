package conformance

import (
	"strings"
	"unicode/utf8"
)

// The two spellings of the same command. D-001 sanctions the rename; D-009
// covers the one place where it leaks into output text.
const (
	upstreamBinary = "rendercv"
	portBinary     = "rendercv-go"
)

// consoleWidth is the terminal width every golden was captured at
// (`testdata/corpus.json`'s `COLUMNS`). A line of exactly this width came out
// of a Rich renderable that pads to the console, so its width is part of the
// comparison.
const consoleWidth = 80

// RebindBinaryName rewrites the port's output into upstream's spelling so the
// two can be compared, and re-pads any fixed-width panel row it shortened.
//
// **This is the one comparison rule that rewrites what the port produced**, and
// it exists because of a conflict inside the contract itself: D-001 renames the
// binary, and `new`'s panel prints a command the user is meant to run. Printing
// `rendercv` there would be an instruction that does not work. `specs/divergences.md`
// D-009 records the decision and its cost.
//
// The cost, stated precisely: on a row containing the binary token, the panel's
// right-hand padding is reconstructed rather than compared, so that row's width
// check is weakened. Every other row is untouched, and the row's *content* to the
// left of the padding is still compared byte for byte.
func RebindBinaryName(text string) string {
	if !strings.Contains(text, portBinary) {
		return text
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.Contains(line, portBinary) {
			continue
		}
		width := utf8.RuneCountInString(line)
		rebound := replaceCommand(line)
		if rebound == line {
			continue
		}

		// **A help page's padded lines are width-sensitive too.** Rich paints a
		// `Padding` region to the console width, so the usage line and the
		// description are runs of text followed by spaces out to column 80,
		// exactly as fixed as a bordered row — and the earlier `isPanelRow`
		// guard left them three characters short. D-010 records the extension.
		//
		// A line that was **not** full width is genuinely free text — the
		// greeting, a stderr usage line — and keeps whatever the substitution
		// produced.
		if !isPanelRow(line) && width != consoleWidth {
			lines[i] = rebound
			continue
		}
		lines[i] = repad(rebound, width)
	}
	return strings.Join(lines, "\n")
}

// replaceCommand rewrites the binary name only where it is used **as a
// command** — standing alone between spaces or at a line's edge.
//
// **The bound is not decoration.** This repository's own directory is called
// `rendercv-go`, so a message quoting an absolute path contains the token too;
// a blind substitution turned `/home/…/rendercv-go/testdata/…` into
// `/home/…/rendercv/testdata/…` and made a passing case fail on a path the port
// had printed correctly.
func replaceCommand(line string) string {
	var out strings.Builder
	for cursor := 0; ; {
		offset := strings.Index(line[cursor:], portBinary)
		if offset < 0 {
			out.WriteString(line[cursor:])
			return out.String()
		}
		start := cursor + offset
		end := start + len(portBinary)

		// The bounds are measured against the whole line, not the remainder, so
		// a second occurrence is judged by what precedes it there.
		standsAlone := (start == 0 || line[start-1] == ' ') &&
			(end == len(line) || line[end] == ' ')

		out.WriteString(line[cursor:start])
		if standsAlone {
			out.WriteString(upstreamBinary)
		} else {
			out.WriteString(portBinary)
		}
		cursor = end
	}
}

// isPanelRow reports whether a line is one of Rich's bordered rows, which is
// what makes its width part of the comparison.
func isPanelRow(line string) bool {
	return strings.HasPrefix(line, "│") && strings.HasSuffix(line, "│")
}

// repad restores a shortened row to its original display width. A bordered row
// grows the run of spaces before its closing border; a padded line simply grows
// its trailing run. Anything else about the line is left alone.
func repad(line string, width int) string {
	missing := width - utf8.RuneCountInString(line)
	if missing <= 0 {
		return line
	}
	const border = "│"
	if !strings.HasSuffix(line, border) {
		return line + strings.Repeat(" ", missing)
	}
	return strings.TrimSuffix(line, border) + strings.Repeat(" ", missing) + border
}
