package bridge

import "github.com/nyaruka/phonenumbers"

// FormatPhone is the header's phone body (connections.py:96-110):
// `phonenumbers.format_number` over the **stored** RFC 3966 string, in the
// format `design.header.connections.phone_number_format` names.
//
// **The stored string is the input, not the user's text.** `PhoneNumber`
// re-groups on validation — `+34-612-345-678` stores as `tel:+34-612-34-56-78`
// — and formatting the user's text instead agrees on most numbers and differs
// on exactly the ones whose national grouping is not the one they typed.
//
// The three format names are `PhoneNumberFormatType`'s members, upper-cased into
// libphonenumber's enum the way `getattr(PhoneNumberFormat, …upper())` does.
// An unknown name cannot arrive from a validated document — the option is a
// Literal — so it falls back to the declared default rather than reporting.
func FormatPhone(stored, format string) string {
	parsed, err := phonenumbers.Parse(stored, "")
	if err != nil {
		// Unreachable from a validated document: `phonenum.Validate` already
		// parsed this very string. Echoing it keeps the header printable.
		return stored
	}
	return phonenumbers.Format(parsed, phoneFormat(format))
}

func phoneFormat(name string) phonenumbers.PhoneNumberFormat {
	switch name {
	case "international":
		return phonenumbers.INTERNATIONAL
	case "E164":
		return phonenumbers.E164
	default:
		// `national` is the declared default (classic_theme.py:326).
		return phonenumbers.NATIONAL
	}
}
