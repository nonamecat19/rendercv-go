package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// SectionRecord is one entry of the typed section list
// (section.py:320-359): a formatted title, the section's entry type, and its
// entries, which are not validated a second time (spec §3.65).
type SectionRecord struct {
	Title     string
	EntryType entries.TypeName
	Entries   []*yamldoc.Node
}

// SnakeCaseTitle is the snake-case form of the record's title: lowercased with
// spaces replaced by underscores (spec §3.66, section.py:85-87).
func (r SectionRecord) SnakeCaseTitle() string {
	return SnakeCaseTitle(r.Title)
}

// Sections derives the typed section list from the `sections` mapping, in input
// order (spec §3.65, §6.1, §6.2). Upstream computes it on demand and caches it
// (spec §3.52, cv.py:128-140); here the caller holds the result.
//
// The type of a section comes from its first entry only, which is safe because
// the whole list has already been validated to one type (section.py:344-349).
// An empty entry list forces `TextEntry` (spec §3.65, §5.5).
func (c *Cv) SectionRecords(registry *entries.Registry) []SectionRecord {
	if c == nil || c.Sections == nil || c.Sections.Kind != yamldoc.KindMapping {
		return nil
	}

	records := make([]SectionRecord, 0, len(c.Sections.Items))
	for _, item := range c.Sections.Items {
		record := SectionRecord{
			Title:     TitleFromKey(item.Key),
			EntryType: TextEntry,
		}

		if item.Value != nil && item.Value.Kind == yamldoc.KindSequence {
			record.Entries = item.Value.Elems
		}

		if len(record.Entries) > 0 {
			if name, err := InferEntryType(record.Entries[0], registry); err == nil {
				record.EntryType = name
			}
		}

		records = append(records, record)
	}
	return records
}
