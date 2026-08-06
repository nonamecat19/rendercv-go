package entries_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The same property stated directly on the two types whose composition was
// wrong, with upstream's measured output as the expectation. Row 1 and row 2 of
// spec 004 §3.9a behavior 33a's table.
func TestOwnFieldErrorsPrecedeBaseFieldErrors(t *testing.T) {
	reference := time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			// Upstream: ('position',) missing, then ('summary',) string_type.
			name: "a missing own field precedes a bad base field",
			src:  "company: c\nsummary:\n  a: 1\n",
			want: []string{"position:missing", "summary:string_type"},
		},
		{
			// Upstream: ('company',) missing, then ('highlights',) list_type.
			name: "the other own field, and a bad list base field",
			src:  "position: p\nhighlights: x\n",
			want: []string{"company:missing", "highlights:list_type"},
		},
		{
			// Row 3, the one an appended date check cannot produce. Upstream:
			// ('position',) missing, ('date',) value_error, ('location',)
			// string_type — `date` sits between the own fields and `location`.
			name: "a bad date lands between the own fields and location",
			src:  "company: c\ndate: 2020-13-01\nlocation:\n  a: 1\n",
			want: []string{
				"position:missing",
				"date:value_error",
				"location:string_type",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := parseNode(t, test.src)
			errs, err := entries.Validate(
				node, "ExperienceEntry", nil, schemaerr.SourceMain, reference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}

			got := make([]string, 0, len(errs))
			for _, e := range errs {
				got = append(got, lastElement(e.SchemaLocation)+":"+string(e.Code))
			}

			if len(got) != len(test.want) {
				t.Fatalf("errors = %v, want %v", got, test.want)
			}
			for i := range test.want {
				if got[i] != test.want[i] {
					t.Fatalf("errors = %v, want %v", got, test.want)
				}
			}
		})
	}
}

// Row 4 of the same table, on PublicationEntry: the `doi` pattern is an enforced
// field constraint, so its failure precedes `journal`'s. Upstream: ('doi',)
// string_pattern_mismatch, then ('journal',) string_type. Iteration 3 appended
// the pattern check after the whole field pass and reported them reversed.
func TestDOIPatternFailureLandsAtItsField(t *testing.T) {
	node := parseNode(t, "title: T\nauthors:\n  - J\ndoi: bad\njournal:\n  a: 1\n")
	errs, err := entries.Validate(
		node, "PublicationEntry", nil, schemaerr.SourceMain,
		time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("internal error: %v", err)
	}

	got := make([]string, 0, len(errs))
	for _, e := range errs {
		got = append(got, lastElement(e.SchemaLocation)+":"+string(e.Code))
	}
	want := []string{"doi:string_pattern_mismatch", "journal:string_type"}

	if len(got) != len(want) {
		t.Fatalf("errors = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("errors = %v, want %v", got, want)
		}
	}
}

func lastElement(location []string) string {
	if len(location) == 0 {
		return ""
	}
	return location[len(location)-1]
}
