// Tests of the one comparison rule that rewrites what the port produced. These
// run under a plain `go test ./...`: a harness that rewrites output is only
// trustworthy if the rewrite is itself pinned, and until now nothing pinned it.
package conformance_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
)

// TestRebindBinaryName covers D-001, D-009 and D-010.
func TestRebindBinaryName(t *testing.T) {
	// row builds a bordered panel row of exactly 80 columns.
	row := func(body string) string {
		return "│ " + body + strings.Repeat(" ", 80-4-len([]rune(body))) + " │"
	}
	// padded builds a Rich `Padding` line: text, then spaces out to 80.
	padded := func(body string) string {
		return body + strings.Repeat(" ", 80-len([]rune(body)))
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "nothing to do",
			in:   row("Generated Typst:            ./out/John_Doe_CV.typ"),
			want: row("Generated Typst:            ./out/John_Doe_CV.typ"),
		},
		{
			// D-009: `new`'s next-step line. The row keeps its width.
			name: "a panel row is rebound and re-padded",
			in:   row("2. Run: rendercv-go render John_Doe_CV.yaml"),
			want: row("2. Run: rendercv render John_Doe_CV.yaml"),
		},
		{
			// D-010: a help page's usage line. It carries no border and is
			// still exactly the console width, so it is re-padded too.
			name: "a full-width padded line is rebound and re-padded",
			in:   padded(" Usage: rendercv-go render [OPTIONS] INPUT_FILE_NAME"),
			want: padded(" Usage: rendercv render [OPTIONS] INPUT_FILE_NAME"),
		},
		{
			// The other half of D-010's rule: free text keeps whatever the
			// substitution produced, because nothing there is width-sensitive.
			name: "a short bare line is not re-padded",
			in:   "Run rendercv-go render to continue",
			want: "Run rendercv render to continue",
		},
		{
			// **The bound is not decoration.** This repository's directory is
			// called `rendercv-go`, so a quoted path carries the token and a
			// blind substitution corrupts a path the port printed correctly.
			name: "a path is not a command",
			in:   row("/home/x/rendercv-go/testdata/cv.yaml"),
			want: row("/home/x/rendercv-go/testdata/cv.yaml"),
		},
		{
			name: "two commands on one row",
			in:   row("Example: rendercv-go new X. Details: rendercv-go new --help"),
			want: row("Example: rendercv new X. Details: rendercv new --help"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := conformance.RebindBinaryName(c.in)
			if got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
			if width := len([]rune(c.in)); width == 80 && len([]rune(got)) != width {
				t.Errorf("a %d-column line came back %d wide", width, len([]rune(got)))
			}
		})
	}
}

// TestRebindPreservesWidth is the property D-009 and D-010 both rest on: a line
// that arrived at the console width leaves at the console width, so the width
// check is weakened only by the reconstruction and never by three missing
// characters.
func TestRebindPreservesWidth(t *testing.T) {
	lines := []string{
		"│ 2. Run: rendercv-go render John_Doe_CV.yaml                                  │",
		" Usage: rendercv-go [OPTIONS] COMMAND [ARGS]...                                 ",
		" Render a YAML input file. Example: rendercv-go render John_Doe_CV.yaml.        ",
	}
	for _, line := range lines {
		if width := len([]rune(line)); width != 80 {
			t.Fatalf("fixture is %d columns, not 80: %q", width, line)
		}
		if got := conformance.RebindBinaryName(line); len([]rune(got)) != 80 {
			t.Errorf("rebound line is %d columns: %q", len([]rune(got)), got)
		}
	}
}
