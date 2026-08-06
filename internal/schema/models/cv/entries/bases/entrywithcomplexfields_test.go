package bases_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

var reference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// Spec §3.73 — the six ordered cases of date-object conversion.
func TestGetDateObject(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		isInteger bool
		reference time.Time
		want      time.Time
	}{
		{name: "integer year", value: "2023", isInteger: true, want: date(2023, 1, 1)},
		{name: "full date", value: "2023-05-17", want: date(2023, 5, 17)},
		{name: "year and month", value: "2023-05", want: date(2023, 5, 1)},
		{name: "year only", value: "2023", want: date(2023, 1, 1)},
		{name: "present", value: "present", reference: reference, want: reference},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bases.GetDateObject(tc.value, tc.isInteger, tc.reference)
			if err != nil {
				t.Fatalf("GetDateObject(%q) = %v", tc.value, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("GetDateObject(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

// Spec §3.74, §4.15 — case 5 with no reference date is an internal error.
func TestPresentWithoutReferenceDate(t *testing.T) {
	_, err := bases.GetDateObject("present", false, time.Time{})

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
	want := "current_date is None when processing 'present' date"
	if internal.Message != want {
		t.Errorf("message = %q, want %q", internal.Message, want)
	}
}

// Spec §3.75, §4.18 — cases 2-4 match the full string, so these reach case 6.
func TestValuesReachingCaseSix(t *testing.T) {
	for _, value := range []string{"20222", "202222-20200", "202222-12-20", "invalid", ""} {
		t.Run(value, func(t *testing.T) {
			_, err := bases.GetDateObject(value, false, reference)

			var internal *schemaerr.InternalError
			if !errors.As(err, &internal) {
				t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
			}
			if internal.Message != "This is not a valid date!" {
				t.Errorf("message = %q, want %q", internal.Message, "This is not a valid date!")
			}
		})
	}
}

// Spec §3.76, §4.13, §5.11 — structurally well-formed but out-of-range dates
// propagate the date library's own text, unwrapped. Pinned so a later decision
// to diverge is visible.
func TestExactDateRangeFailures(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "2022-20-20", want: "month must be in 1..12"},
		{value: "2020-99-99", want: "month must be in 1..12"},
		{value: "2020-01-99", want: "day is out of range for month"},
		{value: "0000-01-01", want: "year 0 is out of range"},
		{value: "2020-99", want: "month must be in 1..12"},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			err := bases.ValidateExactDate(tc.value, false, reference)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("ValidateExactDate(%q) = %v, want %q", tc.value, err, tc.want)
			}
			var rangeErr *bases.DateRangeError
			if !errors.As(err, &rangeErr) {
				t.Errorf("err = %T, want the range error unwrapped", err)
			}
		})
	}
}

// Spec §3.71, §4.14 — a structural failure is the not-a-valid-date message.
func TestExactDateStructuralFailure(t *testing.T) {
	err := bases.ValidateExactDate("aaa", false, reference)

	var exact *bases.ExactDateError
	if !errors.As(err, &exact) {
		t.Fatalf("err = %v (%T), want *bases.ExactDateError", err, err)
	}
	want := "This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM, or YYYY format."
	if exact.Message != want {
		t.Errorf("message = %q, want %q", exact.Message, want)
	}
}

// Spec §3.72 — `present` is accepted as an exact date.
func TestExactDateAcceptsPresent(t *testing.T) {
	if err := bases.ValidateExactDate("present", false, reference); err != nil {
		t.Fatalf("ValidateExactDate(present) = %v, want nil", err)
	}
}

// Spec §3.71 — the accepted exact-date forms.
func TestExactDateAccepts(t *testing.T) {
	tests := []struct {
		value     string
		isInteger bool
	}{
		{value: "2020-09-24"},
		{value: "2020-09"},
		{value: "2020"},
		{value: "2020", isInteger: true},
		{value: "2024-02-29"},
	}
	for _, tc := range tests {
		if err := bases.ValidateExactDate(tc.value, tc.isInteger, reference); err != nil {
			t.Errorf("ValidateExactDate(%q, integer=%v) = %v, want nil", tc.value, tc.isInteger, err)
		}
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
