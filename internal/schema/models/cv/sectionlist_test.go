package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

func records(t *testing.T, sections string) []cv.SectionRecord {
	t.Helper()
	model, errs := cv.Validate(parse(t, "sections:\n"+sections), []string{"cv"}, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	return model.SectionRecords(fixtureRegistry())
}

// Spec §3.65 — one record per section, in input order, with the title formatted.
func TestSectionRecordsInInputOrder(t *testing.T) {
	got := records(t, ""+
		"  education_and_training:\n    - institution: MIT\n"+
		"  experience:\n    - company: Acme\n"+
		"  a_section:\n    - just text\n")

	want := []struct {
		title     string
		entryType entries.TypeName
		snake     string
	}{
		{title: "Education and Training", entryType: "EducationEntry", snake: "education_and_training"},
		{title: "Experience", entryType: "ExperienceEntry", snake: "experience"},
		{title: "a Section", entryType: cv.TextEntry, snake: "a_section"},
	}
	if len(got) != len(want) {
		t.Fatalf("records = %+v, want %d", got, len(want))
	}
	for i, tc := range want {
		if got[i].Title != tc.title {
			t.Errorf("record[%d].Title = %q, want %q", i, got[i].Title, tc.title)
		}
		if got[i].EntryType != tc.entryType {
			t.Errorf("record[%d].EntryType = %q, want %q", i, got[i].EntryType, tc.entryType)
		}
		if got[i].SnakeCaseTitle() != tc.snake {
			t.Errorf("record[%d].SnakeCaseTitle() = %q, want %q", i, got[i].SnakeCaseTitle(), tc.snake)
		}
	}
}

// Spec §3.65, §5.5 — an empty entry list forces TextEntry and keeps the record.
func TestEmptySectionForcesTextEntry(t *testing.T) {
	got := records(t, "  References: []\n")
	if len(got) != 1 {
		t.Fatalf("records = %+v, want exactly one", got)
	}
	if got[0].Title != "References" {
		t.Errorf("title = %q, want %q", got[0].Title, "References")
	}
	if got[0].EntryType != cv.TextEntry {
		t.Errorf("entry type = %q, want TextEntry", got[0].EntryType)
	}
	if len(got[0].Entries) != 0 {
		t.Errorf("entries = %+v, want none", got[0].Entries)
	}
}

// Spec §3.65 — the type comes from the first entry only.
func TestTypeComesFromFirstEntry(t *testing.T) {
	got := records(t, "  mixed:\n    - company: Acme\n    - institution: MIT\n")
	if len(got) != 1 {
		t.Fatalf("records = %+v, want exactly one", got)
	}
	if got[0].EntryType != "ExperienceEntry" {
		t.Errorf("entry type = %q, want ExperienceEntry — the first entry decides", got[0].EntryType)
	}
	if len(got[0].Entries) != 2 {
		t.Errorf("entries = %d, want both carried unvalidated", len(got[0].Entries))
	}
}

// A cv with no sections has no records.
func TestNoSections(t *testing.T) {
	model, _ := cv.Validate(parse(t, "name: John\n"), []string{"cv"}, schemaerr.SourceMain)
	if got := model.SectionRecords(fixtureRegistry()); got != nil {
		t.Errorf("records = %+v, want none", got)
	}
}
