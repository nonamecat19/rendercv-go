package cli

import "strings"

// Segment is Rich's `rich.segment.Segment` (`rich/segment.py:23-40`): a run of
// text together with the style it is written in. A console renders a line by
// walking its segments and emitting each one's escape sequence, its text, and a
// reset.
//
// **Adjacent segments of the same style are not merged**, and that is not an
// implementation detail — it is visible in the bytes. Measured on the `Error`
// panel's top border, where `╭─`, ` `, `Error`, ` `, the fill and `─╮` are all
// `bold red` and come out as **six** separately opened and closed runs
// (spec 012 delta §2.4.1). Rich merges only where `Segment.simplify` is called,
// which nothing on these surfaces calls.
//
// This is why a line is a `[]Segment` and not a `Text`: `Text` carries spans
// over one string and collapses equal-styled neighbours when it renders
// (`style.go`), which is right *inside* a cell and wrong for the line that
// contains it.
type Segment struct {
	Text  string
	Style Style
}

// renderSegments writes a line of segments for one terminal.
//
// An empty segment writes nothing at all, style or no style: it contributes no
// text, and Rich has no run to open for it — `Style.render` returns the text
// untouched when it is empty (`rich/style.py:706`).
func renderSegments(segments []Segment, terminal Terminal) string {
	var out strings.Builder
	for _, segment := range segments {
		out.WriteString(segment.Style.Render(segment.Text, terminal))
	}
	return out.String()
}

// A panel title is cropped by `Text.Truncate` and cut into runs by
// `Text.Segments` (`style.go`), not by a segment-wise crop: `align_text` copies
// the title, truncates the *text*, and only then lets Rich cut it at its span
// boundaries (`rich/panel.py:174-178`). Cropping the segments instead gets the
// same characters and the wrong number of runs for a title whose markup
// straddles the cut.
