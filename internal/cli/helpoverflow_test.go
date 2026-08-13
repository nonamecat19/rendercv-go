package cli

import (
	"slices"
	"strings"
	"testing"
)

// A help table's cells overflow the way every other Rich table's do: with an
// ellipsis, not a fold.
//
// The rule, from the vendored Rich and typer:
//
//   - `Column.overflow` defaults to `"ellipsis"` (`rich/table.py:90`) and each
//     cell renders under it (`:834`).
//   - `Text.wrap` divides the line with `fold=(overflow == "fold")`
//     (`rich/text.py:1239`), so under `ellipsis` a word longer than the column
//     is *not* split; it stays whole and the line it sits on is then cut by
//     `truncate`, which is `set_cell_size(plain, width - 1) + "…"`
//     (`:1248`, `:877-878`).
//   - The console-wide default is `"fold"` (`:36`), which is why a panel's
//     prose still folds. Only table cells ellipsize.
//   - **One exception, and it is reachable**: typer builds the metavar as
//     `Text(style=STYLE_METAVAR, overflow="fold")` (`typer/rich_utils.py:376`),
//     and a `Text`'s own overflow beats the options it is rendered under
//     (`rich/text.py:694`). So the metavar column folds inside a table that
//     ellipsizes everywhere else.
//
// The port folded everywhere, so `--help` in a narrow terminal produced no `…`
// at all: measured against the vendored CLI, `COLUMNS=40 render --help` is 188
// lines with 69 ellipses upstream and 231 lines with none here.

// TestHelpTableCellsEllipsize is the rule for an ordinary column: the word is
// left whole by the wrap and cut by the truncate.
func TestHelpTableCellsEllipsize(t *testing.T) {
	// 12 cells wide, and `installation.` is 13 — the shape measured in the
	// root page's `--install-completion` row at COLUMNS=40.
	lines := helpTable(
		[]helpColumn{{Flexible: true}},
		[][]helpCell{{plain("installation.")}},
		12,
	)
	if len(lines) != 1 {
		t.Fatalf("lines = %q, want one ellipsized line rather than a fold", lines)
	}
	if want := "installatio…"; strings.TrimRight(lines[0], " ") != want {
		t.Errorf("line = %q, want %q", strings.TrimRight(lines[0], " "), want)
	}
}

// TestHelpTableNoWrapColumnEllipsizes covers the commands panel's first column,
// which is `no_wrap=True` (`typer/rich_utils.py:489`): the wrap is skipped
// entirely and the single line is truncated.
func TestHelpTableNoWrapColumnEllipsizes(t *testing.T) {
	lines := helpTable(
		[]helpColumn{{Width: 6, NoWrap: true}, {Flexible: true}},
		[][]helpCell{{plain("create-theme"), plain("x")}},
		20,
	)
	if len(lines) != 1 {
		t.Fatalf("lines = %q, want one line: a no_wrap column does not wrap", lines)
	}
	if !strings.HasPrefix(lines[0], "creat…") {
		t.Errorf("line = %q, want it to start with %q", lines[0], "creat…")
	}
}

// TestHelpMetavarColumnFolds pins the exception through the panel builder, so
// it holds whatever shape the column carries it. Measured against the vendored
// CLI at `COLUMNS=24`, where the metavar column is two cells wide and `PATH`
// comes out as `PA` over `TH` while every neighbour on the same row ellipsizes.
func TestHelpMetavarColumnFolds(t *testing.T) {
	t.Setenv("COLUMNS", "24")

	panel := plainPanel(paramPanel("Arguments", []helpParam{{
		Short:    helpString{Text: "input_file_name"},
		Metavar:  helpString{Text: "PATH"},
		Help:     []helpString{{Text: "The YAML input file."}, {Text: "[required]"}},
		Required: true,
	}}))

	joined := strings.Join(splitLines(panel), "\n")
	if !strings.Contains(joined, "PA") || !strings.Contains(joined, "TH") {
		t.Errorf("metavar did not fold to PA/TH:\n%s", panel)
	}
	if strings.Contains(joined, "PA…") || strings.Contains(joined, "P…") {
		t.Errorf("metavar was ellipsized; it must fold (rich_utils.py:376):\n%s", panel)
	}
}

// TestNarrowHelpPageMatchesUpstream is the end-to-end fixture: the `Arguments`
// panel of `render --help` at `COLUMNS=24`, captured byte for byte from the
// vendored CLI. It carries no binary name, so D-009 does not touch it, and it
// exercises all three behaviors at once — an ellipsized name, a folded metavar,
// and an ellipsized help column.
func TestNarrowHelpPageMatchesUpstream(t *testing.T) {
	t.Setenv("COLUMNS", "24")

	want := []string{
		"╭─ Arguments ──────────╮",
		"│ *    i…      PA  The │",
		"│              TH  YA… │",
		"│                  in… │",
		"│                  fi… │",
		"│                  [r… │",
		"╰──────────────────────╯",
	}

	page := HelpPage("render", Terminal{})
	lines := splitLines(page)
	start := slices.Index(lines, want[0])
	if start < 0 {
		t.Fatalf("no Arguments panel in the page:\n%s", page)
	}
	end := min(start+len(want), len(lines))
	got := lines[start:end]
	if !slices.Equal(got, want) {
		for i := range want {
			if i >= len(got) {
				t.Errorf("line %d missing, want %q", i, want[i])
				continue
			}
			if got[i] != want[i] {
				t.Errorf("line %d\n got %q\nwant %q", i, got[i], want[i])
			}
		}
	}
}
