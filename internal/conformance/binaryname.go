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

		// Only a bordered row gets its padding restored. A bare line — the
		// greeting, a usage line outside a panel — is left exactly as the
		// substitution produced it, because nothing there is width-sensitive.
		if !isPanelRow(line) {
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

// repad restores a shortened panel row to its original display width by growing
// the run of spaces before the closing border. Anything else about the row is
// left alone.
func repad(line string, width int) string {
	missing := width - utf8.RuneCountInString(line)
	if missing <= 0 {
		return line
	}
	border := "│"
	body := strings.TrimSuffix(line, border)
	return body + strings.Repeat(" ", missing) + border
}
