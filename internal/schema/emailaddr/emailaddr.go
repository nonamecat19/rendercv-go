// Package emailaddr mirrors pydantic's `validate_email` wrapper over the
// `email-validator` library, which `cv.email`'s elements are validated by.
//
// **Deliberately partial.** The library's message catalogue is larger than what
// this reproduces; spec 004 §7.4 scopes the port to the measured set and names
// the residual as an open risk. What matters is that the boundary is visible:
// anything outside the set returns ErrUnclassified rather than a plausible
// guess, because guessing produces *wrong* text where upstream produces
// *specific* text, and wrong text passes review more easily than a missing one.
package emailaddr

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Validate returns the normalized address, or the library's own reason.
//
// The reason is returned **bare**, matching the `{reason}` slot of pydantic's
// message template. The caller builds the prefixed message, so this package
// stays a reusable address validator and the one place that knows the pipeline's
// prefix contract is the one place that feeds the pipeline (spec 004 §3.2
// behavior 4a, plan §5.3).
//
// Normalization: the domain is lowercased, the local part is not, and a
// non-ASCII domain keeps its Unicode form (spec 004 §3.15 behavior 54).
func Validate(value string) (string, error) {
	// Pydantic's own pre-check, before the library sees anything.
	if len(value) > maxWrapperLength {
		return "", &Error{Reason: fmt.Sprintf(
			"Length must not exceed %d characters", maxWrapperLength)}
	}

	// The wrapper unwraps a `Name <local@domain>` form and trims, both without
	// error (behavior 53).
	address := strings.TrimSpace(value)
	if open := strings.LastIndex(address, "<"); open >= 0 && strings.HasSuffix(address, ">") {
		address = strings.TrimSpace(address[open+1 : len(address)-1])
	}

	at := strings.Index(address, "@")
	if at < 0 {
		return "", &Error{Reason: "An email address must have an @-sign."}
	}
	local, domain := address[:at], address[at+1:]

	// The local part is checked before the domain throughout: `a b@` reports the
	// space rather than the empty domain, and `@` reports the empty local part
	// rather than the empty domain.
	if err := checkLocal(local); err != nil {
		return "", err
	}
	if err := checkDomain(domain); err != nil {
		return "", err
	}

	normalized := local + "@" + strings.ToLower(domain)

	// The total-length check is last: 300 local characters against a domain with
	// no period reports the domain, not the length.
	if over := len(normalized) - maxAddressLength; over > 0 {
		return "", &Error{Reason: fmt.Sprintf(
			"The email address is too long (%d characters too many).", over)}
	}
	return normalized, nil
}

const (
	// maxWrapperLength is pydantic's pre-check (spec 004 §4.21).
	maxWrapperLength = 2048
	// maxAddressLength is the library's own limit, derived from the measured
	// overshoot: a 306-character address is "52 characters too many".
	maxAddressLength = 254
	// maxDomainLength and maxLabelLength are the DNS limits the library enforces
	// with their own messages, both measured by overshoot.
	maxDomainLength = 253
	maxLabelLength  = 63
)

// plural is the library's own singular/plural on its two count-carrying
// messages: "1 character too many", "2 characters too many".
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ErrUnclassified is returned for a value this package cannot classify.
//
// **It is currently unreachable, and that is the honest shape of the gap rather
// than a claim of completeness.** The residual risk of spec 004 §7.4 runs the
// other way: an address upstream rejects for a rule this package does not
// implement is *accepted* here, not misreported. That is the safer of the two
// failure modes — a wrong-but-plausible message would pass review, whereas a
// missing rejection shows up as a record the differential does not have.
//
// It is kept because the alternative a porter reaches for is a catch-all
// message, which produces wrong text where upstream produces specific text
// (plan §7 hazard 10). If a future rule can be *detected* but not *named*, this
// is what it returns.
var ErrUnclassified = errors.New("email address rejected for an unported reason")

// Error is a failure carrying the library's verbatim reason. Each is already
// period-terminated, so the pipeline's period rule changes nothing.
type Error struct{ Reason string }

func (e *Error) Error() string { return e.Reason }

// localAtext is the character set the library accepts unquoted in a local part.
// `$` is in it and `(` is not, both measured.
const localAtext = "!#$%&'*+-/=?^_`{|}~."

func checkLocal(local string) error {
	if local == "" {
		return &Error{Reason: "There must be something before the @-sign."}
	}
	if strings.HasPrefix(local, `"`) {
		return &Error{Reason: "Quoting the part before the @-sign is not allowed here."}
	}
	if bad := invalidRunes(local, localAtext); len(bad) > 0 {
		return &Error{Reason: fmt.Sprintf(
			"The email address contains invalid characters before the @-sign: %s.",
			strings.Join(bad, ", "))}
	}
	// The trailing period is reported before the leading one: `.` alone reports
	// "immediately before the @-sign", while `.a` reports "cannot start with".
	if strings.HasSuffix(local, ".") {
		return &Error{Reason: "An email address cannot have a period immediately before the @-sign."}
	}
	if strings.HasPrefix(local, ".") {
		return &Error{Reason: "An email address cannot start with a period."}
	}
	if strings.Contains(local, "..") {
		return &Error{Reason: "An email address cannot have two periods in a row."}
	}
	return nil
}

func checkDomain(domain string) error {
	if domain == "" {
		return &Error{Reason: "There must be something after the @-sign."}
	}
	// Only a *fully* bracketed domain is an IP literal; `[1.2.3.4].com` reports
	// the brackets as invalid characters instead.
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		return &Error{Reason: "A bracketed IP address after the @-sign is not allowed here."}
	}
	if bad := invalidRunes(domain, "-."); len(bad) > 0 {
		return &Error{Reason: fmt.Sprintf(
			"The part after the @-sign contains invalid characters: %s.",
			strings.Join(bad, ", "))}
	}
	if strings.HasPrefix(domain, ".") {
		return &Error{Reason: "An email address cannot have a period immediately after the @-sign."}
	}
	if strings.HasPrefix(domain, "-") {
		return &Error{Reason: "An email address cannot have a hyphen immediately after the @-sign."}
	}
	// Trailing before doubled, as in the local part: `b..` reports the trailing
	// period rather than the pair.
	if strings.HasSuffix(domain, ".") {
		return &Error{Reason: "An email address cannot end with a period."}
	}
	if strings.HasSuffix(domain, "-") {
		return &Error{Reason: "An email address cannot end with a hyphen."}
	}
	if strings.Contains(domain, "..") {
		return &Error{Reason: "An email address cannot have two periods in a row."}
	}
	if strings.Contains(domain, ".-") || strings.Contains(domain, "-.") {
		return &Error{Reason: "An email address cannot have a period and a hyphen next to each other."}
	}
	if !strings.Contains(domain, ".") {
		return &Error{Reason: "The part after the @-sign is not valid. It should have a period."}
	}
	// Two limits the library applies to the domain, both measured. They are
	// checked here rather than left to the total-length rule because their
	// messages are different and more specific.
	if over := len(domain) - maxDomainLength; over > 0 {
		return &Error{Reason: fmt.Sprintf(
			"The email address is too long after the @-sign (%d character%s too many).",
			over, plural(over))}
	}
	for _, label := range strings.Split(domain, ".") {
		if over := len(label) - maxLabelLength; over > 0 {
			return &Error{Reason: fmt.Sprintf(
				"After the @-sign, periods cannot be separated by so many characters"+
					" (%d character%s too many).", over, plural(over))}
		}
	}
	// A top-level domain ending in a digit is rejected, which is what makes a
	// bare IPv4 address invalid: `1.2.3.4`, `b.co1` and `b.1` all fail here.
	labels := strings.Split(domain, ".")
	if tld := labels[len(labels)-1]; endsWithDigit(tld) {
		return &Error{Reason: "The part after the @-sign is not valid. It is not within a valid top-level domain."}
	}
	return nil
}

// invalidRunes lists the characters of value that are neither letters, digits,
// nor members of extra, formatted the way the library names them: `SPACE` for a
// space, `U+XXXX` for another non-printing character, and quoted otherwise.
// Each distinct character appears once, in order of first appearance.
func invalidRunes(value, extra string) []string {
	var named []string
	seen := map[rune]bool{}

	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(extra, r) || seen[r] {
			continue
		}
		seen[r] = true
		switch {
		case r == ' ':
			named = append(named, "SPACE")
		case !unicode.IsPrint(r):
			named = append(named, fmt.Sprintf("U+%04X", r))
		default:
			named = append(named, fmt.Sprintf("'%c'", r))
		}
	}
	return named
}

func endsWithDigit(label string) bool {
	if label == "" {
		return false
	}
	runes := []rune(label)
	return unicode.IsDigit(runes[len(runes)-1])
}
