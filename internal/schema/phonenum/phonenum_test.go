package phonenum_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
)

// Spec 004 §3.14 behavior 48's five measured rows, plus two more from other
// numbering plans.
//
// The stored value is **re-grouped**, not passed through, and row 2 is what
// proves it: the input's `612-345-678` comes back as `612-34-56-78`. Iteration 2
// implemented a `tel:` strip alone, which reproduces the input grouping and is
// wrong — and wrong in golden output, since two `.typ` files contain these
// exact strings.
//
// The two extra rows exist so a libphonenumber metadata bump that changes
// grouping fails here loudly rather than in a PDF diff months later.
func TestValidateStoresTheRegroupedRFC3966Form(t *testing.T) {
	tests := []struct{ input, stored, serialized string }{
		{"+905419999999", "tel:+90-541-999-99-99", "+90-541-999-99-99"},
		{"+34-612-345-678", "tel:+34-612-34-56-78", "+34-612-34-56-78"},
		{"+1-415-555-0142", "tel:+1-415-555-0142", "+1-415-555-0142"},
		{"+44 20 1234 5678", "tel:+44-20-1234-5678", "+44-20-1234-5678"},
		{"+493012345678", "tel:+49-30-12345678", "+49-30-12345678"},
		// Not in upstream's table; measured here as metadata canaries.
		{"+819012345678", "tel:+81-90-1234-5678", "+81-90-1234-5678"},
		{"+61412345678", "tel:+61-412-345-678", "+61-412-345-678"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			stored, err := phonenum.Validate(test.input)
			if err != nil {
				t.Fatalf("Validate(%q) = %v", test.input, err)
			}
			if stored != test.stored {
				t.Errorf("stored = %q, want %q", stored, test.stored)
			}
			if got := phonenum.Serialize(stored); got != test.serialized {
				t.Errorf("serialized = %q, want %q", got, test.serialized)
			}
		})
	}
}

// Behavior 50: one failure kind, and its message is the **dictionary key**
// rather than the replacement.
//
// Emitting the replacement here would take the message out of the dictionary's
// reach and hand already-finalized text to the period rule. It happens to be
// harmless for this string — §4.8 ends in a period — which is exactly why the
// distinction needs a test rather than a comment.
func TestValidateRejectsWithTheDictionaryKey(t *testing.T) {
	for _, input := range []string{
		"not_a_valid_phone_number",
		"",
		"12345",
		// No default region upstream, so a national number is invalid rather
		// than guessed at.
		"415-555-0142",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := phonenum.Validate(input); err == nil {
				t.Fatalf("Validate(%q) succeeded", input)
			} else if err.Error() != "value is not a valid phone number" {
				t.Errorf("message = %q, want the dictionary key", err.Error())
			}
		})
	}
}

// A number that parses but is not a valid subscriber number is still rejected.
// libphonenumber parses far more than it will vouch for, so the validity check
// is separate and load-bearing.
func TestValidateRequiresAValidNumber(t *testing.T) {
	// A well-formed +1 number whose area code does not exist.
	if stored, err := phonenum.Validate("+1-000-000-0000"); err == nil {
		t.Errorf("Validate accepted %q as %q; parsing is not validity", "+1-000-000-0000", stored)
	}
}

// Serialize replaces rather than trims, because upstream replaces. The two agree
// on every real input — a phone number cannot contain a second `tel:` — and this
// pins the shape so the difference stays visible if one ever could.
func TestSerializeRemovesEveryPrefix(t *testing.T) {
	if got := phonenum.Serialize("tel:+90-541-999-99-99"); got != "+90-541-999-99-99" {
		t.Errorf("Serialize = %q", got)
	}
	if got := phonenum.Serialize("+90-541-999-99-99"); got != "+90-541-999-99-99" {
		t.Errorf("Serialize of an unprefixed value = %q, want it unchanged", got)
	}
}
