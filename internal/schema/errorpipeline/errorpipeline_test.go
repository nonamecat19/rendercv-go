package errorpipeline

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.6's four-row table, for the rows reachable with only steps 1 and 8
// in place. Three of the four look like bugs and are not — the period is
// appended after whatever punctuation the message already ends with.
func TestTrailingPeriod(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			// The one row with no dictionary entry, measured at
			// expected_errors.yaml:112.
			name:    "an unmatched message just gains the period",
			message: "Input should be a valid string",
			want:    "Input should be a valid string.",
		},
		{
			// §4.12's end_date text ends in `!`, so the final message ends `!.`.
			name: "a message ending in an exclamation keeps it",
			message: "This is not a valid `end_date`. Please use either YYYY-MM-DD," +
				" YYYY-MM, or YYYY format or 'present'!",
			want: "This is not a valid `end_date`. Please use either YYYY-MM-DD," +
				" YYYY-MM, or YYYY format or 'present'!.",
		},
		{
			// §4.7's YouTube text ends `username."`, so the final ends `.".`.
			name:    "a message ending in a quoted sentence gains a second period",
			message: `The username should be a valid YouTube username."`,
			want:    `The username should be a valid YouTube username.".`,
		},
		{
			// §4.11's color text ends `50%)"`, so the final ends `)".`.
			name:    "a message ending in a quoted parenthesis",
			message: `some examples of valid colors: "hsl(0, 100%, 50%)"`,
			want:    `some examples of valid colors: "hsl(0, 100%, 50%)".`,
		},
		{
			name:    "a message already ending in a period is untouched",
			message: "This field is required.",
			want:    "This field is required.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := appendPeriod(test.message); got != test.want {
				t.Errorf("appendPeriod(%q) =\n  %q\nwant\n  %q", test.message, got, test.want)
			}
		})
	}
}

// Step 1 is replacement, not a prefix test, so **every** occurrence goes
// (spec 004 §6 rule 6). The doubled cases are synthetic: no upstream message
// carries either prefix twice. They exist because the rule is contractual and
// the obvious implementation — `strings.TrimPrefix` — satisfies every
// single-occurrence row while getting these wrong.
func TestStripPrefixes(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			// Measured on `email: bad`.
			name:    "the email prefix, once",
			message: "value is not a valid email address: An email address must have an @-sign.",
			want:    "An email address must have an @-sign.",
		},
		{
			// Measured on `date: 2020-13-01`.
			name:    "the value-error prefix, once",
			message: "Value error, month must be in 1..12",
			want:    "month must be in 1..12",
		},
		{
			name:    "the email prefix twice, both removed",
			message: "value is not a valid email address: a value is not a valid email address: b",
			want:    "a b",
		},
		{
			name:    "the value-error prefix twice, both removed",
			message: "Value error, a Value error, b",
			want:    "a b",
		},
		{
			name:    "not at the start, still removed",
			message: "wrapped: Value error, inner",
			want:    "wrapped: inner",
		},
		{
			name:    "a message with neither is unchanged",
			message: "This field is required.",
			want:    "This field is required.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := stripPrefixes(test.message); got != test.want {
				t.Errorf("stripPrefixes(%q) =\n  %q\nwant\n  %q", test.message, got, test.want)
			}
		})
	}
}

// The two steps compose in the documented order, and the period is genuinely
// last: stripping after appending would leave `month must be in 1..12` without
// its period on the value-error row.
func TestParseAppliesStripThenPeriod(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{
		{Code: "value_error", Message: "Value error, month must be in 1..12"},
		{Code: "string_type", Message: "Input should be a valid string"},
	})

	want := []string{"month must be in 1..12.", "Input should be a valid string."}
	if len(got) != len(want) {
		t.Fatalf("Parse returned %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Message != want[i] {
			t.Errorf("record %d message = %q, want %q", i, got[i].Message, want[i])
		}
	}
}

// Record order is the raw order, unsorted (spec 004 §6 rule 1). Asserted on
// messages that would sort differently, so an accidental sort fails here.
func TestParseKeepsRawOrder(t *testing.T) {
	got := Parse([]schemaerr.ValidationError{
		{Message: "zebra"}, {Message: "alpha"}, {Message: "middle"},
	})

	want := []string{"zebra.", "alpha.", "middle."}
	for i := range want {
		if got[i].Message != want[i] {
			t.Fatalf("messages = %v, want %v", messagesOf(got), want)
		}
	}
}

// Parse does not mutate what it was given. The raw list is iterated more than
// once downstream — for the record and again for its children — so aliasing it
// would corrupt the second pass.
func TestParseLeavesTheRawRecordsAlone(t *testing.T) {
	raw := []schemaerr.ValidationError{{Message: "Value error, boom"}}
	Parse(raw)

	if raw[0].Message != "Value error, boom" {
		t.Errorf("raw record was mutated: %q", raw[0].Message)
	}
}

func messagesOf(records []schemaerr.ValidationError) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, r.Message)
	}
	return out
}

// A record whose validator pinned its own location keeps it: the pipeline must
// not re-derive what the override already decided (spec 004 §3.2 step 3).
func TestParseSkipsTheDiscriminatorForAPinnedLocation(t *testing.T) {
	pinned := schemaerr.ValidationError{
		Message:         "nope",
		SchemaLocation:  []string{"design", "theme"},
		LocationIsFinal: true,
	}
	if got := Parse([]schemaerr.ValidationError{pinned})[0]; len(got.SchemaLocation) != 2 ||
		got.SchemaLocation[1] != "theme" {
		t.Errorf("location = %v, want it left alone", got.SchemaLocation)
	}

	// The same location without the flag loses its second element, which is what
	// makes the assertion above mean something.
	unpinned := pinned
	unpinned.LocationIsFinal = false
	if got := Parse([]schemaerr.ValidationError{unpinned})[0]; len(got.SchemaLocation) != 1 {
		t.Errorf("location = %v, want the branch element dropped", got.SchemaLocation)
	}
}
