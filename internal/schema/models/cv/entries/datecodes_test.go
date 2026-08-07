package entries_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §4.13 and §4.33, and the code each failure carries.
//
// **An exact date does not have one code.** Upstream's `validate_exact_date`
// catches only its own internal error, so CPython's exceptions escape uncaught
// and arrive as `value_error`; only the structural failure the validator raises
// deliberately is `rendercv_other_error`. Measured, all five rows.
//
// The port had every exact-date failure on the custom code, which is right for
// one row of five.
func TestExactDateCodesAndMessages(t *testing.T) {
	tests := []struct {
		value   string
		code    string
		message string
	}{
		{"2020-13-01", "value_error", "month must be in 1..12"},
		{"0000-01-01", "value_error", "year 0 is out of range"},
		{"2020-01-99", "value_error", "day is out of range for month"},
		// A year outside four digits reaches fromisoformat as a malformed
		// string, so CPython reports the string rather than a range.
		{"10000", "value_error", "Invalid isoformat string: '10000-01-01'"},
		{"1", "value_error", "Invalid isoformat string: '1-01-01'"},
		// The one the validator raises itself.
		{"aaa", "rendercv_other_error", "This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format."},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			errs, err := entries.Validate(
				parseNode(t, "company: c\nposition: p\nstart_date: "+test.value+"\n"),
				"ExperienceEntry", nil, schemaerr.SourceMain, entryReference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if string(errs[0].Code) != test.code {
				t.Errorf("code = %q, want %q", errs[0].Code, test.code)
			}
			if errs[0].Message != test.message {
				t.Errorf("message = %q, want %q", errs[0].Message, test.message)
			}
		})
	}
}

// The same three range messages on the arbitrary `date`, which has always been
// `value_error`, plus the two the dictionary rewrites.
//
// They are stored **unprefixed** here: upstream's carry `Value error, ` because
// the exception escaped, and the port does not fabricate that prefix. The
// dictionary matches by substring either way, so the final text is identical —
// which the second half asserts rather than assuming.
func TestArbitraryDateMessagesAreUnprefixed(t *testing.T) {
	for _, test := range []struct{ value, raw string }{
		{"2020-13-01", "month must be in 1..12"},
		{"0000-01-01", "year 0 is out of range"},
		{"2020-01-99", "day is out of range for month"},
	} {
		t.Run(test.value, func(t *testing.T) {
			errs, err := entries.Validate(
				parseNode(t, "name: n\ndate: "+test.value+"\n"),
				"NormalEntry", nil, schemaerr.SourceMain, entryReference,
			)
			if err != nil {
				t.Fatalf("internal error: %v", err)
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.raw {
				t.Errorf("raw message = %q, want %q (unprefixed)", errs[0].Message, test.raw)
			}
		})
	}
}
