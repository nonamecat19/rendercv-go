package conformance

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// AssertBytes fails unless got is byte-for-byte want. Parity axis 1 is about bytes, so
// nothing is normalised here — not line endings, not trailing whitespace, not the final
// newline. The failure output makes invisible characters visible, because in this project
// most diffs are whitespace.
func AssertBytes(t *testing.T, name string, want, got []byte) {
	t.Helper()

	if string(want) == string(got) {
		return
	}

	off := firstDiff(want, got)
	line, col := lineCol(want, off)

	var b strings.Builder
	fmt.Fprintf(&b, "%s: output differs from the golden\n", name)
	fmt.Fprintf(&b, "  first difference at byte %d (line %d, column %d)\n", off, line, col)
	fmt.Fprintf(&b, "  golden length %d, got length %d\n\n", len(want), len(got))
	fmt.Fprintf(&b, "  golden:\n%s\n", visibleContext(want, off))
	fmt.Fprintf(&b, "  got:\n%s\n", visibleContext(got, off))
	b.WriteString("  likely causes, in order: trim_blocks/lstrip_blocks whitespace, " +
		"str.splitlines() semantics, slice bounds, filter behavior, number/date formatting, " +
		"map ordering. See .claude/skills/rendercv-parity-debug.")

	t.Error(b.String())
}

// AssertText is AssertBytes for strings.
func AssertText(t *testing.T, name, want, got string) {
	t.Helper()
	AssertBytes(t, name, []byte(want), []byte(got))
}

// AssertFileSet fails unless rendercv-go created exactly the files upstream created.
// Output layout and naming are part of the contract (spec §1.3), so an extra or missing
// file is a parity failure even when every shared file matches.
func AssertFileSet(t *testing.T, want, got []string) {
	t.Helper()

	missing, extra := diffSets(want, got)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("output file set differs from the golden\n")
	for _, p := range missing {
		fmt.Fprintf(&b, "  missing: %s\n", p)
	}
	for _, p := range extra {
		fmt.Fprintf(&b, "  unexpected: %s\n", p)
	}
	t.Error(b.String())
}

// AssertExitCode checks parity axis 2's exit-code requirement.
func AssertExitCode(t *testing.T, want, got int) {
	t.Helper()

	if want != got {
		t.Errorf("exit code: golden %d, got %d", want, got)
	}
}

// AssertPNGGeometry checks the PNG half of spec §1.2: same pages, same pixel dimensions.
// Pixel-level comparison is a stretch goal tracked in specs/STATE.md, not a gate.
func AssertPNGGeometry(t *testing.T, want, got map[string]string) {
	t.Helper()

	for path, w := range want {
		g, ok := got[path]
		switch {
		case !ok:
			t.Errorf("PNG %s: not produced", path)
		case w != g:
			t.Errorf("PNG %s: golden %s, got %s", path, w, g)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("PNG %s: produced but not in the golden", path)
		}
	}
}

func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func lineCol(b []byte, off int) (line, col int) {
	line, col = 1, 1
	for i := 0; i < off && i < len(b); i++ {
		if b[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

// visibleContext renders the three lines around off with whitespace made visible,
// in the spirit of `cat -A`.
func visibleContext(b []byte, off int) string {
	lines := strings.Split(string(b), "\n")
	target, _ := lineCol(b, off)

	lo := max(target-3, 1)
	hi := min(target+3, len(lines))

	var out strings.Builder
	for i := lo; i <= hi; i++ {
		marker := "   "
		if i == target {
			marker = ">> "
		}
		fmt.Fprintf(&out, "    %s%4d | %s\n", marker, i, makeVisible(lines[i-1]))
	}
	return strings.TrimRight(out.String(), "\n")
}

func makeVisible(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == ' ':
			out.WriteString("·")
		case r == '\t':
			out.WriteString("→")
		case r == '\r':
			out.WriteString("^M")
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&out, "\\x%02x", r)
		case !utf8.ValidRune(r):
			out.WriteString("�")
		default:
			out.WriteRune(r)
		}
	}
	out.WriteString("$")
	return out.String()
}

func diffSets(want, got []string) (missing, extra []string) {
	inGot := make(map[string]bool, len(got))
	for _, g := range got {
		inGot[g] = true
	}
	inWant := make(map[string]bool, len(want))
	for _, w := range want {
		inWant[w] = true
		if !inGot[w] {
			missing = append(missing, w)
		}
	}
	for _, g := range got {
		if !inWant[g] {
			extra = append(extra, g)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}
