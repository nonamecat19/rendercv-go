package entries_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The entry-internal ordering fixture: spec 004 §3.9a behavior 33a's four rows,
// in one table.
//
// It runs on the **raw** failure list, before the error pipeline of spec 004 §3
// touches it, and asserts the whole location and the whole order. Both of those
// are deliberate:
//
//   - Order is a correctness prerequisite, not a cosmetic property. Dedup keeps
//     the *first* record at a location (§3.8 behavior 27), so a wrongly-ordered
//     raw list makes dedup keep the wrong row (§3.9a behavior 33c). Every
//     mechanism downstream assumes this list is already in upstream's order.
//   - Asserting here rather than downstream means a future change to field
//     composition or to date handling fails on one row of this table instead of
//     inside a 25-record end-to-end diff.
//
// Every `want` is measured against the vendored Python, not derived from the Go
// constants. Iteration 3's ordering defect survived a green suite precisely
// because its assertions were written from the port's own behavior.
func TestEntryFailureOrder(t *testing.T) {
	tests := []struct {
		name string
		// entryType and src are the input; want is upstream's measured output as
		// `location:code`, in order.
		entryType entries.TypeName
		src       string
		want      []string
	}{
		// §3.9a behavior 33a, row 1. ExperienceEntry declares
		// `company, position, date, start_date, end_date, location, summary,
		// highlights` — own fields first, because upstream writes
		// `class ExperienceEntry(BaseEntryWithComplexFields, BaseExperienceEntry)`
		// and pydantic emits the last-listed base's fields first.
		{
			name:      "a missing own field precedes a bad base field",
			entryType: "ExperienceEntry",
			src:       "company: c\nsummary:\n  a: 1\n",
			want:      []string{"position:missing", "summary:string_type"},
		},
		// Row 2, the other own field and a bad list-shaped base field.
		{
			name:      "the other own field, and a bad list base field",
			entryType: "ExperienceEntry",
			src:       "position: p\nhighlights: x\n",
			want:      []string{"company:missing", "highlights:list_type"},
		},
		// Row 3, the load-bearing one (behavior 33b): `date` sits between the own
		// fields and `location`, and its failure appears there. An appended date
		// check cannot produce this order, which is how iteration 3's defect was
		// found.
		{
			name:      "a bad date lands between the own fields and location",
			entryType: "ExperienceEntry",
			src:       "company: c\ndate: 2020-13-01\nlocation:\n  a: 1\n",
			want: []string{
				"position:missing",
				"date:value_error",
				"location:string_type",
			},
		},
		// Row 4, on PublicationEntry, which declares
		// `title, authors, summary, doi, url, journal, date`. The `doi` pattern is
		// an enforced field constraint, so its failure precedes `journal`'s;
		// iteration 3 appended it after the whole field pass and reported the two
		// reversed.
		{
			name:      "a doi pattern failure precedes a later field's",
			entryType: "PublicationEntry",
			src:       "title: T\nauthors:\n  - J\ndoi: bad\njournal:\n  a: 1\n",
			want:      []string{"doi:string_pattern_mismatch", "journal:string_type"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatErrors(validateEntry(t, test.entryType, test.src), false)
			assertRows(t, got, test.want)
		})
	}
}

// entryReference is the date `present` resolves to in these tests. Fixed rather
// than time.Now(), so an entry carrying `end_date: present` is reproducible.
var entryReference = time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC)

// validateEntry runs one entry through the raw validator, before any error
// pipeline.
func validateEntry(t *testing.T, entryType entries.TypeName, src string) []schemaerr.ValidationError {
	t.Helper()

	errs, err := entries.Validate(
		parseNode(t, src), entryType, nil, schemaerr.SourceMain, entryReference,
	)
	if err != nil {
		t.Fatalf("internal error: %v", err)
	}
	return errs
}

// formatErrors renders records as `location:code`, or `location:code:message`
// when the message is part of what is being asserted.
func formatErrors(errs []schemaerr.ValidationError, withMessage bool) []string {
	rows := make([]string, 0, len(errs))
	for _, e := range errs {
		row := strings.Join(e.SchemaLocation, ".") + ":" + string(e.Code)
		if withMessage {
			row += ":" + e.Message
		}
		rows = append(rows, row)
	}
	return rows
}

func assertRows(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("errors = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("errors = %#v, want %#v", got, want)
		}
	}
}
