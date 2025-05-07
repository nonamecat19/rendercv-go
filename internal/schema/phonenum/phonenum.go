// Package phonenum mirrors `pydantic_extra_types.phone_numbers.PhoneNumber`,
// the type `cv.phone`'s elements are validated by.
//
// Upstream parses with libphonenumber, requires a valid number, and stores the
// **RFC 3966** form. The stored value is re-grouped rather than passed through,
// which is the whole reason this is a library wrapper and not a `tel:` strip.
package phonenum

import (
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// MessageInvalid is the raw failure message, which is exactly dictionary row 7's
// key (`error_dictionary.yaml:8`).
//
// **The key, not spec 004 §4.8's replacement.** The pipeline does the
// substituting; a validator that pre-substitutes takes the message out of the
// dictionary's reach, and the trailing-period rule then applies to text that has
// already been finalized. Harmless for this one string and a landmine for the
// rest.
const MessageInvalid = "value is not a valid phone number"

// Error is an invalid phone number.
type Error struct{}

func (e *Error) Error() string { return MessageInvalid }

// Validate parses a phone number and returns its RFC 3966 form, `tel:` prefix
// included (models/cv/cv.py:23-25).
//
// No default region: upstream's `PhoneNumber` has none either, so a number
// without a country code is invalid rather than guessed at.
//
// Parsing is not enough — libphonenumber parses plenty of numbers it will not
// vouch for — so validity is checked separately, mirroring upstream's
// `is_valid_number` call.
func Validate(value string) (string, error) {
	parsed, err := phonenumbers.Parse(value, "")
	if err != nil {
		return "", &Error{}
	}
	if !phonenumbers.IsValidNumber(parsed) {
		return "", &Error{}
	}
	return phonenumbers.Format(parsed, phonenumbers.RFC3966), nil
}

// Serialize mirrors serialize_phone (models/cv/cv.py:231-250): the stored value
// without its `tel:` prefix.
//
// Replacement rather than a trim, because upstream replaces. A phone number
// cannot contain a second `tel:` so the two agree on every input; the shape is
// kept so the two functions diff against each other.
func Serialize(stored string) string {
	return strings.ReplaceAll(stored, "tel:", "")
}
