package bridge_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// validCv reads a `cv` block and fails the test if it did not validate, so no
// case here can accidentally assert on a model built from a rejected document.
func validCv(t *testing.T, document string) *cv.Cv {
	t.Helper()
	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}

	model, errs := cv.Validate(node, []string{"cv"}, schemaerr.SourceMain, cv.Options{
		Registry: entries.Default(),
		Context: &valctx.ValidationContext{
			CurrentDate: time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
		},
	})
	if len(errs) > 0 {
		t.Fatalf("the document did not validate: %v", errs)
	}
	return model
}

// Spec 009 §1 behaviors 1-4. The order is the input file's, and the two titles
// are computed from the key by two different rules.
func TestSectionsKeepInputOrderAndBothTitles(t *testing.T) {
	model := validCv(t, `
sections:
  work_experience:
    - company: A Company
      position: Engineer
  skills_and_tools:
    - label: Programming
      details: Go
`)

	got := bridge.Sections(model, entries.Default())
	want := []struct{ title, snake, entryType string }{
		{"Work Experience", "work_experience", "ExperienceEntry"},
		// The stop word stays lowercase in the title and the snake-case form is
		// taken from that title, not from the key — the round trip is only the
		// identity by coincidence here.
		{"Skills and Tools", "skills_and_tools", "OneLineEntry"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d sections, want %d", len(got), len(want))
	}
	for i, expected := range want {
		if got[i].Title != expected.title {
			t.Errorf("section %d title = %q, want %q", i, got[i].Title, expected.title)
		}
		if got[i].SnakeCaseTitle != expected.snake {
			t.Errorf("section %d snake title = %q, want %q", i, got[i].SnakeCaseTitle, expected.snake)
		}
		if got[i].EntryType != expected.entryType {
			t.Errorf("section %d type = %q, want %q", i, got[i].EntryType, expected.entryType)
		}
	}
}

// Spec 009 §1 behavior 5. `TextEntry` is chosen before any entry is examined,
// because there is none to examine.
func TestEmptySectionIsATextEntrySection(t *testing.T) {
	model := validCv(t, "sections:\n  notes: []\n")

	got := bridge.Sections(model, entries.Default())
	if len(got) != 1 {
		t.Fatalf("got %d sections, want 1", len(got))
	}
	if got[0].EntryType != "TextEntry" {
		t.Errorf("entry type = %q, want %q", got[0].EntryType, "TextEntry")
	}
	if len(got[0].Entries) != 0 {
		t.Errorf("entries = %v, want none", got[0].Entries)
	}
}

// Spec 009 §1 behavior 7: a bare string is a `TextEntry`, which has no model and
// so no dump — it arrives as text rather than as a field map.
func TestTextEntriesArriveAsText(t *testing.T) {
	model := validCv(t, "sections:\n  notes:\n    - A note.\n    - Another.\n")

	got := bridge.Sections(model, entries.Default())
	entryList := got[0].Entries
	if len(entryList) != 2 {
		t.Fatalf("got %d entries, want 2", len(entryList))
	}
	for i, want := range []string{"A note.", "Another."} {
		if !entryList[i].IsText || entryList[i].Text != want {
			t.Errorf("entry %d = %#v, want the text %q", i, entryList[i], want)
		}
	}
}

// A mapping entry arrives dumped, with the integer dates its own YAML wrote.
func TestMappingEntriesArriveDumped(t *testing.T) {
	model := validCv(t, `
sections:
  education:
    - institution: MIT
      area: CS
      start_date: 2000
      end_date: "2005-06"
`)

	entry := bridge.Sections(model, entries.Default())[0].Entries[0]
	want := map[string]any{
		"institution": "MIT",
		"area":        "CS",
		"start_date":  2000,
		"end_date":    "2005-06",
	}
	if !reflect.DeepEqual(entry.Fields, want) {
		t.Errorf("fields = %#v, want %#v", entry.Fields, want)
	}
	if !entry.YearOnly["start_date"] || entry.YearOnly["end_date"] {
		t.Errorf("yearOnly = %#v, want start_date only", entry.YearOnly)
	}
}

// A document with no `sections` key at all has no sections, rather than one
// empty one — the difference between a CV with an empty section and a CV
// without.
func TestNoSectionsKeyYieldsNothing(t *testing.T) {
	model := validCv(t, "name: John Doe\n")
	if got := bridge.Sections(model, entries.Default()); len(got) != 0 {
		t.Errorf("got %d sections, want none", len(got))
	}
}
