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
// text, and Rich has no run to open for it.
func renderSegments(segments []Segment, terminal Terminal) string {
	var out strings.Builder
	for _, segment := range segments {
		if segment.Text == "" {
			continue
		}
		if sequence := segment.Style.SGR(terminal); sequence != "" {
			out.WriteString(sequence)
			out.WriteString(segment.Text)
			out.WriteString(reset)
			continue
		}
		out.WriteString(segment.Text)
	}
	return out.String()
}

// cropSegments cuts a line of segments to a cell width, dropping whatever falls
// past it — the operation `align_text` performs on a panel title that no longer
// fits (`rich/panel.py:174-178`).
//
// It cuts with `cutCells`, so the crop lands exactly where the unstyled path's
// does and a double-width character is replaced by a space rather than halved.
func cropSegments(segments []Segment, width int) []Segment {
	out := make([]Segment, 0, len(segments))
	remaining := width
	for _, segment := range segments {
		if remaining <= 0 {
			break
		}
		text := segment.Text
		if cellLen(text) > remaining {
			text, _ = cutCells(text, remaining)
		}
		remaining -= cellLen(text)
		out = append(out, Segment{Text: text, Style: segment.Style})
	}
	return out
}
