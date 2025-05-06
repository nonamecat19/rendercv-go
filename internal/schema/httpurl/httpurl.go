// Package httpurl mirrors `pydantic.HttpUrl`, the type four RenderCV fields are
// declared with and a fifth reaches as a union arm.
//
// Pydantic-core implements the WHATWG URL Standard, so this wraps a WHATWG
// parser rather than `net/url`: the standard's serialization is what produces
// the trailing slash, the punycoded host and the dropped default port that
// upstream's golden `.typ` files contain.
package httpurl

import (
	"strings"

	"github.com/nlnwa/whatwg-url/url"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The three failure kinds, measured (spec 004 §3.13 behavior 45).
const (
	// CodeURLParsing is every parse failure. Pydantic's raw message is
	// `Input should be a valid URL, ` plus a varying reason, and the dictionary
	// flattens all of them onto one replacement — so the reason is unobservable
	// and this package does not reproduce it.
	CodeURLParsing schemaerr.Code = "url_parsing"
	// CodeURLScheme is a URL that parsed but is not http or https.
	CodeURLScheme schemaerr.Code = "url_scheme"
	// CodeURLTooLong is an input over the length limit.
	CodeURLTooLong schemaerr.Code = "url_too_long"
)

// The three messages. Only the first has a dictionary row; the other two reach
// the user with a period appended and nothing else.
const (
	// MessageURLParsing is exactly the dictionary key
	// (`error_dictionary.yaml:6`), so the pipeline replaces it with spec 004
	// §4.9. Emitting the key rather than the replacement keeps the substitution
	// in one place.
	MessageURLParsing = "Input should be a valid URL"
	// MessageURLScheme is spec 004 §4.19, measured for `ftp://example.com`.
	MessageURLScheme = "URL scheme should be 'http' or 'https'"
	// MessageURLTooLong is spec 004 §4.20.
	MessageURLTooLong = "URL should have at most 2083 characters"
)

// MaxLength is the limit pydantic applies, inclusive.
//
// **UTF-8 bytes, not characters.** The two coincide for ASCII, so the
// distinction only shows on a non-ASCII URL: measured by bisection against the
// vendored Python, 1051 non-ASCII characters (2082 bytes) pass and 1052 (2084
// bytes) fail. Spec 004 §3.13 behavior 46 said "characters"; the check lives in
// pydantic-core, which is Rust and counts bytes. Go's len() is therefore right
// and utf8.RuneCountInString would be wrong.
const MaxLength = 2083

// Error is a URL failure carrying the code the pipeline dispatches on.
type Error struct {
	Code    schemaerr.Code
	Message string
}

func (e *Error) Error() string { return e.Message }

// Validate mirrors `pydantic.HttpUrl`, returning the WHATWG-serialized form.
//
// The four steps are in this order and the order is observable
// (spec 004 §3.13 behavior 46):
//
//  1. the length limit, **on the input string and before parsing** — measured:
//     `https://exa mple.com/` followed by 3000 characters reports `url_too_long`
//     and not `url_parsing`, which is only possible if length is checked first;
//  2. parse, discarding the library's reason text;
//  3. the scheme check, which needs a parsed URL;
//  4. the serialized form.
//
// Step 1 is on the **input**, not on the result. A 420-character input whose
// serialized form runs to 2420 characters passes, because the input is what is
// measured.
func Validate(value string) (string, error) {
	if len(value) > MaxLength {
		return "", &Error{Code: CodeURLTooLong, Message: MessageURLTooLong}
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", &Error{Code: CodeURLParsing, Message: MessageURLParsing}
	}

	scheme := strings.TrimSuffix(parsed.Protocol(), ":")
	if scheme != "http" && scheme != "https" {
		return "", &Error{Code: CodeURLScheme, Message: MessageURLScheme}
	}

	return parsed.Href(false), nil
}
