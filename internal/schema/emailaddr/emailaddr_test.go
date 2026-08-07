package emailaddr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/emailaddr"
)

// Every rejection this package reproduces, measured one by one against the
// vendored Python.
//
// The first twelve rows are spec 004 §3.15 behavior 55's required table; the
// rest were measured while deriving the **order** of the checks, and they are
// kept because the order is what the table specifies and nothing else pins it.
// `a@` must report "something after" and not "no period"; `.` alone must report
// "immediately before the @-sign" and not "cannot start with one"; `a@b..` must
// report the trailing period and not the pair.
func TestRejections(t *testing.T) {
	tests := []struct{ input, reason string }{
		// Behavior 55's twelve.
		{"not_a_valid_email", "An email address must have an @-sign."},
		{"", "An email address must have an @-sign."},
		{"a@", "There must be something after the @-sign."},
		{"@b.com", "There must be something before the @-sign."},
		{"a@b", "The part after the @-sign is not valid. It should have a period."},
		{"a b@c.com", "The email address contains invalid characters before the @-sign: SPACE."},
		{"a@@b.com", "The part after the @-sign contains invalid characters: '@'."},
		{"a@b..com", "An email address cannot have two periods in a row."},
		{".a@b.com", "An email address cannot start with a period."},
		{"a.@b.com", "An email address cannot have a period immediately before the @-sign."},
		{"a@-b.com", "An email address cannot have a hyphen immediately after the @-sign."},
		{"a@[1.2.3.4]", "A bracketed IP address after the @-sign is not allowed here."},

		// The pre-check and the two length limits.
		{strings.Repeat("x", 2049) + "@b.com", "Length must not exceed 2048 characters"},
		{strings.Repeat("a", 300) + "@b.com", "The email address is too long (52 characters too many)."},
		{strings.Repeat("a", 250) + "@b.com", "The email address is too long (2 characters too many)."},

		// Order: the local part is checked before the domain.
		{"@", "There must be something before the @-sign."},
		{"a b@", "The email address contains invalid characters before the @-sign: SPACE."},
		{".a@b", "An email address cannot start with a period."},
		// Order: domain syntax before the total-length check.
		{strings.Repeat("a", 300) + "@b", "The part after the @-sign is not valid. It should have a period."},

		// Order inside the local part: trailing period before leading.
		{".@b.com", "An email address cannot have a period immediately before the @-sign."},
		{"..a@b.com", "An email address cannot start with a period."},
		{"a.b.@c.com", "An email address cannot have a period immediately before the @-sign."},
		{"a..b@c.com", "An email address cannot have two periods in a row."},
		{`"a b"@c.com`, "Quoting the part before the @-sign is not allowed here."},
		{"a\tb@c.com", "The email address contains invalid characters before the @-sign: U+0009."},
		{"a(b@c.com", "The email address contains invalid characters before the @-sign: '('."},

		// Order inside the domain.
		{"a@.b.com", "An email address cannot have a period immediately after the @-sign."},
		{"a@-b", "An email address cannot have a hyphen immediately after the @-sign."},
		{"a@b.", "An email address cannot end with a period."},
		{"a@b.com.", "An email address cannot end with a period."},
		{"a@b..", "An email address cannot end with a period."},
		{"a@b.c-", "An email address cannot end with a hyphen."},
		{"a@B..COM", "An email address cannot have two periods in a row."},
		{"a@b-.com", "An email address cannot have a period and a hyphen next to each other."},
		{"a@@b", "The part after the @-sign contains invalid characters: '@'."},
		// Only a *fully* bracketed domain is an IP literal.
		{"a@[1.2.3.4].com", "The part after the @-sign contains invalid characters: '[', ']'."},
		{"a@b_c.com", "The part after the @-sign contains invalid characters: '_'."},
		{"a@b c.com", "The part after the @-sign contains invalid characters: SPACE."},
		{"a@b$c.com", "The part after the @-sign contains invalid characters: '$'."},
		{"a@b(c)d.com", "The part after the @-sign contains invalid characters: '(', ')'."},
		// A top-level domain ending in a digit, which is what rejects a bare IP.
		{"a@1.2.3.4", "The part after the @-sign is not valid. It is not within a valid top-level domain."},
		{"a@b.co1", "The part after the @-sign is not valid. It is not within a valid top-level domain."},
		{"a@b.1", "The part after the @-sign is not valid. It is not within a valid top-level domain."},
		{".", "An email address must have an @-sign."},
	}

	for _, test := range tests {
		name := test.input
		if len(name) > 30 {
			name = name[:30] + "…"
		}
		t.Run(name, func(t *testing.T) {
			got, err := emailaddr.Validate(test.input)
			if err == nil {
				t.Fatalf("Validate accepted it as %q", got)
			}
			if errors.Is(err, emailaddr.ErrUnclassified) {
				t.Fatalf("reached the unclassified branch; every row here is measured")
			}
			if err.Error() != test.reason {
				t.Errorf("reason =\n  %q\nwant\n  %q", err.Error(), test.reason)
			}
		})
	}
}

// Behavior 56's accepted inputs, behavior 54's normalization, and behavior 53's
// three wrapper behaviors.
//
// `$` in a local part is the row that says the atext set is not "alphanumerics
// and a dot": it is valid before the @-sign and invalid after it.
func TestAccepted(t *testing.T) {
	tests := []struct{ input, want string }{
		{"a@b.c", "a@b.c"},
		{"john.doe+tag@example.co.uk", "john.doe+tag@example.co.uk"},
		{"a-@b.com", "a-@b.com"},
		{"a$b@c.com", "a$b@c.com"},

		// Behavior 54: the domain is lowercased, the local part is not, and a
		// non-ASCII domain keeps its Unicode form.
		{"JOHN.DOE@Example.COM", "JOHN.DOE@example.com"},
		{"A@B.COM", "A@b.com"},
		{"a@ünicode.de", "a@ünicode.de"},
		{"düsseldorf@example.com", "düsseldorf@example.com"},

		// Behavior 53: unwrap and trim, both silently.
		{"  a@b.c  ", "a@b.c"},
		{"Name <a@b.c>", "a@b.c"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := emailaddr.Validate(test.input)
			if err != nil {
				t.Fatalf("Validate(%q) = %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("Validate(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

// The reason is returned bare, matching the `{reason}` slot of pydantic's
// template. The caller adds the prefix, and the pipeline then strips it.
//
// Pinning this here keeps the two halves of spec 004 §3.2 behavior 4a from
// drifting: if this package started returning the prefixed form, the strip would
// remove it and the text would still be right — until something else read the
// reason.
func TestTheReasonIsBare(t *testing.T) {
	_, err := emailaddr.Validate("not_a_valid_email")
	if err == nil {
		t.Fatal("Validate accepted an address with no @-sign")
	}
	if strings.HasPrefix(err.Error(), "value is not a valid email address") {
		t.Errorf("reason = %q, want it without the template's prefix", err.Error())
	}
}

// Every reason is already period-terminated, except pydantic's own pre-check —
// which is upstream's inconsistency, not the port's. The pipeline's period rule
// is what makes them uniform, so it must have nothing to do for the rest.
func TestReasonsArePeriodTerminated(t *testing.T) {
	for _, input := range []string{
		"not_a_valid_email", "a@", "@b.com", "a@b", "a b@c.com", "a@@b.com",
		"a@b..com", ".a@b.com", "a.@b.com", "a@-b.com", "a@[1.2.3.4]",
		strings.Repeat("a", 300) + "@b.com",
	} {
		_, err := emailaddr.Validate(input)
		if err == nil {
			t.Errorf("Validate(%q) succeeded", input)
			continue
		}
		if !strings.HasSuffix(err.Error(), ".") {
			t.Errorf("Validate(%q) = %q, which does not end in a period", input, err.Error())
		}
	}
}

// The library's two domain-length rules, which have their own messages rather
// than folding into the total-length one — including its singular/plural.
func TestDomainLengthLimits(t *testing.T) {
	tests := []struct{ input, reason string }{
		{
			"a@" + strings.Repeat("b", 64) + ".com",
			"After the @-sign, periods cannot be separated by so many characters (1 character too many).",
		},
		{
			"a@b." + strings.Repeat("c", 64),
			"After the @-sign, periods cannot be separated by so many characters (1 character too many).",
		},
		{
			"a@" + strings.Repeat("b", 250) + ".com",
			"The email address is too long after the @-sign (1 character too many).",
		},
	}

	for _, test := range tests {
		got, err := emailaddr.Validate(test.input)
		if err == nil {
			t.Errorf("Validate accepted a %d-character address as %q", len(test.input), got)
			continue
		}
		if err.Error() != test.reason {
			t.Errorf("reason =\n  %q\nwant\n  %q", err.Error(), test.reason)
		}
	}

	// A 71-character local part is accepted, so the local part has no limit of
	// its own here — the total-length rule is what eventually catches it.
	if _, err := emailaddr.Validate(strings.Repeat("a", 65) + "@b.com"); err != nil {
		t.Errorf("a 65-character local part was rejected: %v", err)
	}
}
