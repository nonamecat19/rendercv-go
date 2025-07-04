// Package bridge turns a validated schema model into the `process.Model` the
// templater consumes (spec 009).
//
// **It is the one package that imports both halves.** `process` is deliberately
// downstream of `models` (spec 008 plan §4) — the engine never reaches back into
// validation — so the code that reads a `cv.Cv` and writes a `process.Model` can
// live in neither and has a package of its own.
package bridge

import (
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Sections is `get_rendercv_sections` (section.py:320-355) projected onto the
// renderer's own types: the `sections` mapping becomes a list, in the mapping's
// own order, which the YAML reader preserved and which is therefore the input
// file's.
//
// **The snake-case title is the *formatted* title's**, not the key's
// (section.py:85-87). `work_experience` becomes the title `Work Experience` and
// then `work_experience` again, so the round trip looks like the identity —
// until a stop word appears. `skills_and_tools` titles as `Skills and Tools` and
// snake-cases back to `skills_and_tools`, but `Skills And Tools` written
// directly by the user keeps its capitals as a title and snake-cases the same
// way. The value decides which sections `show_time_spans_in` matches, so
// deriving it from the key instead would silently change which sections show a
// time span.
func Sections(model *cv.Cv, registry *entries.Registry) []process.Section {
	records := model.SectionRecords(registry)
	sections := make([]process.Section, 0, len(records))

	for _, record := range records {
		sections = append(sections, process.Section{
			Title:          record.Title,
			SnakeCaseTitle: record.SnakeCaseTitle(),
			// An empty section's type is `TextEntry` (`:342-344`), chosen by
			// SectionRecords before any entry is examined because there is none.
			EntryType: string(record.EntryType),
			Entries:   entriesOf(record),
		})
	}
	return sections
}

func entriesOf(record cv.SectionRecord) []process.Entry {
	out := make([]process.Entry, 0, len(record.Entries))
	for _, node := range record.Entries {
		out = append(out, entryOf(node, record.EntryType))
	}
	return out
}

// entryOf is the two entry shapes: a bare string is a `TextEntry`
// (section.py:162-165) with no model and therefore no dump, and anything else
// dumps as pydantic would.
func entryOf(node *yamldoc.Node, entryType entries.TypeName) process.Entry {
	if node == nil {
		return process.Entry{IsText: true}
	}
	if node.Kind != yamldoc.KindMapping {
		return process.Entry{Text: node.Raw, IsText: true}
	}

	fields, yearOnly := entries.Dump(node, entryType)
	return process.Entry{Fields: fields, YearOnly: yearOnly}
}
