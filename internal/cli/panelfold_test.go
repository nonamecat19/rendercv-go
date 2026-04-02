package cli

import (
	"slices"
	"testing"
)

// A run of padding spaces is part of the word before it, and it folds with it.
//
// `rich/_wrap.py:9` matches a word as `\s*\S+\s*` — **the trailing whitespace
// belongs to the word** — and `divide_line` hands the whole thing, spaces
// included, to `chop_cells` when it is too long for a line (`:59`). So the ten
// spaces that pad `Generated Typst:` out to the progress panel's 26-column
// message field become lines of their own, and upstream's panel has two blank
// rows in the middle of a step that the port did not draw.
//
// The port's fold dropped the gap between two words whenever it did not fit
// (`panel.go`'s `if lineWidth+cellLen(gap) <= width`), which is a line short
// every time a padded field overflows. It also trimmed the last line, where
// Rich keeps whatever whitespace still fits: `rstrip_end` crops only the excess
// beyond the width (`rich/text.py:1015-1023`), so `ms    ` becomes `ms   ` and
// not `ms`.
//
// The expectation is upstream's own `rendercv render CV.yaml` at nine columns,
// captured from the vendored CLI.
func TestPanelFoldsPaddingIntoItsOwnLines(t *testing.T) {
	t.Setenv("COLUMNS", "9")

	row := PanelRow{
		Mark:   "✓",
		Timing: "35 ms",
		Label:  "Generated Typst:",
		Value:  "./rendercv_output/John_Doe_CV.typ",
	}

	want := []string{
		"╭─ Your─╮",
		"│ ✓ 35  │",
		"│ ms    │",
		"│ Gener │",
		"│ ated  │",
		"│ Typst │",
		"│ :     │",
		"│       │",
		"│       │",
		"│ ./ren │",
		"│ dercv │",
		"│ _outp │",
		"│ ut/Jo │",
		"│ hn_Do │",
		"│ e_CV. │",
		"│ typ   │",
		"╰───────╯",
	}

	got := splitLines(Panel("Your CV is ready", []PanelRow{row}))
	if !slices.Equal(got, want) {
		t.Errorf("progress panel at 9 columns:\n got %q\nwant %q", got, want)
	}
}
