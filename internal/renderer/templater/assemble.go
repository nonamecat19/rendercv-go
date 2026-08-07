// Package templater is the Go side of upstream's `renderer/templater/`
// (spec 008). The processors live in `process/`; this package is the engine that
// renders templates and assembles their output into a document.
package templater

import "strings"

// Fragment names the templates `render_full_template` renders, in the order it
// renders them (templater.py:50-127).
//
// They are constants rather than strings at the call site because each is also a
// path under `templates/<format>/`, and the loader of spec 008 §2 builds four
// candidate paths from it.
const (
	FragmentPreamble         = "Preamble"
	FragmentHeader           = "Header"
	FragmentSectionBeginning = "SectionBeginning"
	FragmentSectionEnding    = "SectionEnding"

	// FragmentFull is the HTML document's single fragment. It carries its
	// extension because `Full.html` has no `.j2.` infix (templater.py:152-154)
	// and the loader takes the name verbatim for that format.
	FragmentFull = "Full.html"
)

// RenderedSection is one section's three rendered pieces.
type RenderedSection struct {
	Beginning string
	Entries   []string
	Ending    string
}

// Assemble is `render_full_template`'s document assembly (templater.py:88-125),
// separated from the rendering so it can be tested before a single template
// exists.
//
// **The separators are as much of the contract as the templates are**, and they
// differ between the two formats:
//
//   - Typst opens `preamble + "\n\n" + header + "\n"`; Markdown opens
//     `header + "\n"` and has no preamble at all.
//   - a section's entries are joined by **two** newlines;
//   - a section is `beginning + "\n" + entries + "\n" + ending`;
//   - each section is appended with a **leading** newline, so the document has
//     no trailing newline beyond whatever its last section ends with.
//
// Getting any one of them wrong is a byte diff on every corpus case at once,
// which is why they are pinned here rather than left to the first golden.
func Assemble(preamble, header string, sections []RenderedSection, typst bool) string {
	var out strings.Builder

	if typst {
		out.WriteString(preamble)
		out.WriteString("\n\n")
	}
	out.WriteString(header)
	out.WriteString("\n")

	for _, section := range sections {
		out.WriteString("\n")
		out.WriteString(section.Beginning)
		out.WriteString("\n")
		out.WriteString(strings.Join(section.Entries, "\n\n"))
		out.WriteString("\n")
		out.WriteString(section.Ending)
	}
	return out.String()
}
