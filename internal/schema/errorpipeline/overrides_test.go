package errorpipeline

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.5 behaviors 15-17. Containment, not equality, is upstream's test
// verbatim — the `my_end_date` row is what fails if it is tightened.
func TestOverrideEndDate(t *testing.T) {
	tests := []struct {
		name     string
		location []string
		fires    bool
	}{
		{"the field itself", []string{"cv", "sections", "x", "0", "end_date"}, true},
		{"a field merely containing it", []string{"cv", "my_end_date"}, true},
		{
			"before the filter runs, the branch element still matches",
			[]string{"end_date", "literal['present']"},
			false,
		},
		{"start_date does not match", []string{"cv", "start_date"}, false},
		{"an unrelated field", []string{"cv", "name"}, false},
		{"an empty location does not panic", nil, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := overrideEndDate(test.location, "original")
			if test.fires && got != messageEndDate {
				t.Errorf("location %v: message = %q, want the override", test.location, got)
			}
			if !test.fires && got != "original" {
				t.Errorf("location %v: message = %q, want it untouched", test.location, got)
			}
		})
	}
}

// Behavior 18's first half. Both conditions are equality and both are required,
// so each row below turns one of them off.
func TestStripCurrentDateSuffix(t *testing.T) {
	tests := []struct {
		name     string
		location []string
		want     []string
	}{
		{
			name:     "both conditions hold",
			location: []string{"settings", "current_date", "date"},
			want:     []string{"settings", "current_date"},
		},
		{
			name:     "the last element is not exactly date",
			location: []string{"settings", "current_date", "literal['today']"},
			want:     []string{"settings", "current_date", "literal['today']"},
		},
		{
			name:     "the element before is not current_date",
			location: []string{"cv", "start_date", "date"},
			want:     []string{"cv", "start_date", "date"},
		},
		{
			name:     "a one-element location cannot match",
			location: []string{"date"},
			want:     []string{"date"},
		},
		{name: "an empty location does not panic", location: nil, want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stripCurrentDateSuffix(test.location)
			if strings.Join(got, ".") != strings.Join(test.want, ".") {
				t.Errorf("stripCurrentDateSuffix(%v) = %v, want %v",
					test.location, got, test.want)
			}
		})
	}
}

// The two overrides end to end, through Parse, which is where the interaction
// with step 8 shows: `end_date` picks up a period after its `!`, and
// `current_date` does not gain a second one.
func TestParseAppliesTheTwoOverrides(t *testing.T) {
	tests := []struct {
		name     string
		location []string
		want     string
	}{
		{
			// §4.12. The `!.` ending is upstream's and is not a typo to fix.
			name:     "a bad end_date",
			location: []string{"cv", "sections", "x", "0", "end_date"},
			want: "This is not a valid `end_date`! Please use either YYYY-MM-DD," +
				` YYYY-MM, or YYYY format or "present"!.`,
		},
		{
			// §4.13, already period-terminated.
			name:     "a bad current_date, reached through the suffix strip",
			location: []string{"settings", "current_date", "date"},
			want: "This is not a valid `current_date`! Please use YYYY-MM-DD format" +
				` or "today".`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Parse([]schemaerr.ValidationError{{
				SchemaLocation: test.location,
				Message:        "This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format.",
			}}, nil, nil)[0]

			if got.Message != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", got.Message, test.want)
			}
		})
	}
}

// Both raw failures for one bad `end_date` reduce to the same location and the
// same message, which is the whole reason the override exists — deduplication
// can then keep either one and get the right answer.
func TestBothEndDateBranchesReduceAlike(t *testing.T) {
	const wrapper = "function-after[validate_exact_date(), union[str,int]]"

	got := Parse([]schemaerr.ValidationError{
		{SchemaLocation: []string{"cv", "end_date", wrapper}, Message: "branch one"},
		{SchemaLocation: []string{"cv", "end_date", "literal['present']"}, Message: "branch two"},
	}, nil, nil)

	if strings.Join(got[0].SchemaLocation, ".") != "cv.end_date" ||
		strings.Join(got[1].SchemaLocation, ".") != "cv.end_date" {
		t.Fatalf("locations = %v and %v, want both cv.end_date",
			got[0].SchemaLocation, got[1].SchemaLocation)
	}
	if got[0].Message != got[1].Message {
		t.Errorf("messages differ:\n  %q\n  %q", got[0].Message, got[1].Message)
	}
}
