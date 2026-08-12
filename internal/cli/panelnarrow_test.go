package cli

import (
	"slices"
	"strconv"
	"testing"
)

// A panel narrower than its own title is Rich's `width <= 4` branch and its
// title crop, and the port panicked on both.
//
// `rich/panel.py:234-246`:
//
//	if title_text is None or width <= 4:
//	    yield Segment(box.get_top([width - 2]))
//	else:
//	    title_text = align_text(title_text, width - 4, self.title_align, box.top, ...)
//	    yield Segment(box.top_left + box.top)
//	    yield from console.render(title_text, child_options.update_width(width - 4))
//	    yield Segment(box.top + box.top_right)
//
// So the top border is `╭` `─`, then the title rendered at exactly `width - 4`
// and padded out with the box's own `─`, then `─` `╮`. The title carries
// `no_wrap = True` and a one-space pad on each side (`:119`, `:121`), and it is
// rendered under the console's default overflow, which is `"fold"`
// (`rich/text.py:36`) — so it is **cropped, not ellipsized**: `truncate` spends
// no column on `…` unless the overflow is `"ellipsis"` (`:877-880`). At four
// columns or fewer there is no room for a title at all, and the body renders to
// nothing.
//
// The port computed the fill as `width - cellLen(head) - 1` and handed it to
// `strings.Repeat`, which **panics on a negative count**. Measured before the
// fix: `COLUMNS=30 rendercv-go render CV.yaml` on a document with any validation
// error exits **2** with `panic: strings: negative Repeat count` and a goroutine
// dump, where upstream prints its panel and exits 1. Reachable from any terminal
// narrower than the title plus four — 32 columns for `There are validation
// errors!`.
//
// Every expectation below is upstream's own output for the same invocation,
// `render CV.yaml --nosuchoption`, captured from the vendored CLI.
func TestPanelNarrowerThanItsTitle(t *testing.T) {
	const message = "There is a problem with the extra arguments (--nosuchoption)! " +
		"Each key should have a corresponding value."

	tests := []struct {
		width int
		want  []string
	}{{
		// width <= 4: no title, and no body at all.
		width: 4,
		want: []string{
			"╭──╮",
			"╰──╯",
		},
	}, {
		width: 9,
		want: []string{
			"╭─ Erro─╮",
			"│ There │",
			"│ is a  │",
			"│ probl │",
			"│ em    │",
			"│ with  │",
			"│ the   │",
			"│ extra │",
			"│ argum │",
			"│ ents  │",
			"│ (--no │",
			"│ sucho │",
			"│ ption │",
			"│ )!    │",
			"│ Each  │",
			"│ key   │",
			"│ shoul │",
			"│ d     │",
			"│ have  │",
			"│ a     │",
			"│ corre │",
			"│ spond │",
			"│ ing   │",
			"│ value │",
			"│ .     │",
			"╰───────╯",
		},
	}, {
		width: 14,
		want: []string{
			"╭─ Error ────╮",
			"│ There is a │",
			"│ problem    │",
			"│ with the   │",
			"│ extra      │",
			"│ arguments  │",
			"│ (--nosucho │",
			"│ ption)!    │",
			"│ Each key   │",
			"│ should     │",
			"│ have a     │",
			"│ correspond │",
			"│ ing value. │",
			"╰────────────╯",
		},
	}, {
		width: 20,
		want: []string{
			"╭─ Error ──────────╮",
			"│ There is a       │",
			"│ problem with the │",
			"│ extra arguments  │",
			"│ (--nosuchoption) │",
			"│ ! Each key       │",
			"│ should have a    │",
			"│ corresponding    │",
			"│ value.           │",
			"╰──────────────────╯",
		},
	}}

	for _, test := range tests {
		t.Run(strconv.Itoa(test.width), func(t *testing.T) {
			t.Setenv("COLUMNS", strconv.Itoa(test.width))

			// No `TrimRight`: `splitLines` keeps only what a `\n` closes, so
			// trimming the panel's last newline would drop its closing border
			// and no expectation below could ever be met.
			got := splitLines(Panel("Error", []PanelRow{{Text: message}}))
			if !slices.Equal(got, test.want) {
				t.Errorf("panel at %d columns:\n got %q\nwant %q", test.width, got, test.want)
			}
		})
	}
}

// TestValidationPanelTitleIsCropped is the shape that actually crashed a real
// run: the title `There are validation errors!` needs 32 columns, and the port
// panicked at every width below that.
func TestValidationPanelTitleIsCropped(t *testing.T) {
	t.Setenv("COLUMNS", "20")

	// Upstream's top border for this panel at 20 columns, measured.
	const want = "╭─ There are valid─╮"

	got := splitLines(Panel("There are validation errors!", []PanelRow{{Text: "x"}}))
	if len(got) == 0 || got[0] != want {
		t.Errorf("top border = %q, want %q", got, want)
	}
}
