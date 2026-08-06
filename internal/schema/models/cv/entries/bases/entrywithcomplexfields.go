package bases

import (
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

var yearPattern = regexp.MustCompile(`^\d{4}$`)

// Messages of spec §4.14, §4.15 and §4.18, verbatim.
const (
	messageInvalidExactDate = "This is not a valid date! Please use either YYYY-MM-DD," +
		" YYYY-MM, or YYYY format."
	messagePresentWithoutReference = "current_date is None when processing 'present' date"
	messageInvalidDate             = "This is not a valid date!"
)

// GetDateObject mirrors get_date_object
// (entry_with_complex_fields.py:43-87). The six cases of spec §3.73 are tried
// strictly in order and the first match wins:
//
//  1. an integer → January 1 of that year;
//  2. a full `YYYY-MM-DD` → that ISO date;
//  3. a full `YYYY-MM` → the 1st of that month;
//  4. a full `YYYY` → January 1 of that year;
//  5. exactly `present` → the reference date;
//  6. anything else → the internal error of spec §4.18.
//
// isInteger marks case 1, which upstream distinguishes by the Python type of
// the value rather than by its spelling.
//
// A reference date is required for case 5 only; supplying the zero time means
// "no reference date", which is the internal error of spec §4.15.
func GetDateObject(value string, isInteger bool, reference time.Time) (time.Time, error) {
	switch {
	case isInteger:
		return isoDate(value + "-01-01")
	case fullDatePattern.MatchString(value):
		return isoDate(value)
	case monthDatePattern.MatchString(value):
		return isoDate(value + "-01")
	case yearPattern.MatchString(value):
		return isoDate(value + "-01-01")
	case value == "present":
		if reference.IsZero() {
			return time.Time{}, &schemaerr.InternalError{Message: messagePresentWithoutReference}
		}
		return reference, nil
	default:
		return time.Time{}, &schemaerr.InternalError{Message: messageInvalidDate}
	}
}

// ValidateExactDate mirrors validate_exact_date
// (entry_with_complex_fields.py:15-37): a structural failure becomes the
// not-a-valid-date message of spec §4.14, while a range failure propagates the
// date library's own message (spec §4.13) unwrapped, because upstream's handler
// catches only the internal error.
func ValidateExactDate(value string, isInteger bool, reference time.Time) error {
	_, err := GetDateObject(value, isInteger, reference)
	if err == nil {
		return nil
	}

	var rangeErr *DateRangeError
	if errors.As(err, &rangeErr) {
		return rangeErr
	}

	var internal *schemaerr.InternalError
	if errors.As(err, &internal) && internal.Message == messagePresentWithoutReference {
		// Spec §4.15 is an internal error and is not rewritten into a
		// user-facing validation message.
		return internal
	}

	return &ExactDateError{Message: messageInvalidExactDate}
}

// ExactDateError is the user-facing failure of spec §4.14.
type ExactDateError struct {
	Message string
}

func (e *ExactDateError) Error() string {
	return e.Message
}

// isoDate parses a `YYYY-MM-DD` string with CPython's range checks and its
// messages (spec §4.13), the same ones ValidateArbitraryDate reports.
func isoDate(value string) (time.Time, error) {
	// An integer year outside four digits — `2020` is the only shape upstream's
	// tests pin — produces CPython's "Invalid isoformat string" rather than a
	// range message.
	//
	// TODO(iteration-4): spec §7.3 — no upstream test pins this text, so it is
	// reported through the not-a-valid-date path for now.
	if !fullDatePattern.MatchString(value) {
		return time.Time{}, &schemaerr.InternalError{Message: messageInvalidDate}
	}
	if err := parseISODate(value); err != nil {
		return time.Time{}, err
	}
	year, _ := strconv.Atoi(value[0:4])
	month, _ := strconv.Atoi(value[5:7])
	day, _ := strconv.Atoi(value[8:10])
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}
