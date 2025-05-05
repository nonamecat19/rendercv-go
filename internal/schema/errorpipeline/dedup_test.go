package errorpipeline

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.8 behavior 28's three rows, each as the pair of raw records the
// producer actually emits.
func TestDedupCollapsesTheThreeMeasuredSites(t *testing.T) {
	const exactDate = "function-after[validate_exact_date(), union[str,int]]"

	tests := []struct {
		name        string
		raw         []schemaerr.ValidationError
		wantMessage string
	}{
		{
			// Both branches force the same message, so either survivor is right —
			// which is why the override exists.
			name: "a bad end_date, two union branches",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"cv", "end_date", exactDate}, Message: "branch one"},
				{SchemaLocation: []string{"cv", "end_date", "literal['present']"}, Message: "branch two"},
			},
			wantMessage: "This is not a valid `end_date`! Please use either YYYY-MM-DD," +
				` YYYY-MM, or YYYY format or "present"!.`,
		},
		{
			name: "a bad current_date, two union branches",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"settings", "current_date", "date"}, Message: "branch one"},
				{SchemaLocation: []string{"settings", "current_date", "literal['today']"}, Message: "branch two"},
			},
			wantMessage: "This is not a valid `current_date`! Please use YYYY-MM-DD format" +
				` or "today".`,
		},
		{
			// The load-bearing one: the messages differ, and dedup is the only
			// thing suppressing the URL failure.
			name: "a missing photo, the path branch then a URL branch",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"cv", "photo"}, Message: "The file path does not exist"},
				{SchemaLocation: []string{"cv", "photo"}, Message: "Input should be a valid URL"},
			},
			wantMessage: "The file path does not exist.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mustParse(t, test.raw, nil, nil)
			if len(got) != 1 {
				t.Fatalf("got %d records, want 1: %v", len(got), locationsOf(got))
			}
			if got[0].Message != test.wantMessage {
				t.Errorf("message =\n  %q\nwant\n  %q", got[0].Message, test.wantMessage)
			}
		})
	}
}

// First occurrence wins and the survivors keep their relative order, so dedup is
// an ordered set. Three records at two locations is the smallest case that
// distinguishes "keep the first" from "keep the last" *and* catches a reorder.
func TestDedupKeepsTheFirstInOrder(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{
		{SchemaLocation: []string{"cv", "b"}, Message: "first at b"},
		{SchemaLocation: []string{"cv", "a"}, Message: "only at a"},
		{SchemaLocation: []string{"cv", "b"}, Message: "second at b"},
	}, nil, nil)

	want := []string{"first at b.", "only at a."}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Message != want[i] {
			t.Errorf("record %d = %q, want %q", i, got[i].Message, want[i])
		}
	}
}

// A section key is a user-chosen string and may contain the separator. Joining
// with a dot would make `("cv", "a.b")` and `("cv", "a", "b")` collide, silently
// dropping the second record; NUL cannot appear in a YAML key.
func TestDedupDoesNotCollideOnADottedKey(t *testing.T) {
	got := mustParse(t, []schemaerr.ValidationError{
		{SchemaLocation: []string{"cv", "a.b"}, Message: "the dotted key"},
		{SchemaLocation: []string{"cv", "a", "b"}, Message: "the nested path"},
	}, nil, nil)

	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 — the two locations collided: %v",
			len(got), locationsOf(got))
	}
}

// Spec 004 §3.9b behavior 33e, end to end. This is what A4's branch pairs were
// for: the filter collapses each pair onto the bare field and dedup keeps the
// first, so the surviving *message* is decided by the declared union arm order.
//
// `date` is `int | str` and `start_date` is `str | int`, so the two fields
// survive with different messages. A port that emitted one hand-picked record
// per field would get one of them wrong.
func TestNonScalarDatesSurviveWithTheRightMessage(t *testing.T) {
	const exactDate = "function-after[validate_exact_date(), union[str,int]]"

	tests := []struct {
		name string
		raw  []schemaerr.ValidationError
		want string
	}{
		{
			name: "date keeps the integer message",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"cv", "date", "int"}, Code: "int_type", Message: "Input should be a valid integer"},
				{SchemaLocation: []string{"cv", "date", "str"}, Code: "string_type", Message: "Input should be a valid string"},
			},
			want: "Input should be a valid integer.",
		},
		{
			name: "start_date keeps the string message",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"cv", "start_date", "str"}, Code: "string_type", Message: "Input should be a valid string"},
				{SchemaLocation: []string{"cv", "start_date", "int"}, Code: "int_type", Message: "Input should be a valid integer"},
			},
			want: "Input should be a valid string.",
		},
		{
			// end_date's override fires regardless of which branch survives.
			name: "end_date keeps its forced message",
			raw: []schemaerr.ValidationError{
				{SchemaLocation: []string{"cv", "end_date", exactDate, "str"}, Code: "string_type", Message: "Input should be a valid string"},
				{SchemaLocation: []string{"cv", "end_date", exactDate, "int"}, Code: "int_type", Message: "Input should be a valid integer"},
				{SchemaLocation: []string{"cv", "end_date", "literal['present']"}, Code: "literal_error", Message: "Input should be 'present'"},
			},
			want: "This is not a valid `end_date`! Please use either YYYY-MM-DD," +
				` YYYY-MM, or YYYY format or "present"!.`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mustParse(t, test.raw, nil, nil)
			if len(got) != 1 {
				t.Fatalf("got %d records, want 1: %v", len(got), locationsOf(got))
			}
			if got[0].Message != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", got[0].Message, test.want)
			}
			if location := strings.Join(got[0].SchemaLocation, "."); !strings.HasPrefix(location, "cv.") {
				t.Errorf("location = %q, want the bare field", location)
			}
		})
	}
}
