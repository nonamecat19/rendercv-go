package httpurl_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
)

// Spec 004 §3.13 behavior 42's eleven measured rows, in full.
//
// This table is why the package wraps a WHATWG parser rather than `net/url`:
// the trailing slash, the punycoded host and the dropped default port are the
// standard's serialization, and two of them are visible in upstream's golden
// `.typ` output (behavior 43).
func TestNormalization(t *testing.T) {
	tests := []struct{ input, want string }{
		{"https://example.com", "https://example.com/"},
		{"HTTPS://Example.COM/Path", "https://example.com/Path"},
		{"https://example.com:443/a/b?x=1#frag", "https://example.com/a/b?x=1#frag"},
		{"http://example.com:80", "http://example.com/"},
		{"https://user:pw@ex.com/p", "https://user:pw@ex.com/p"},
		{"https://example.com/a%20b", "https://example.com/a%20b"},
		{"https://xn--80ak6aa92e.com", "https://xn--80ak6aa92e.com/"},
		{"https://ünicode.de/ünï", "https://xn--nicode-2ya.de/%C3%BCn%C3%AF"},
		{"https://[::1]:8080/x", "https://[::1]:8080/x"},
		{"https://example.com?", "https://example.com/?"},
		{"https://example.com#", "https://example.com/#"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := httpurl.Validate(test.input)
			if err != nil {
				t.Fatalf("Validate(%q) = %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("Validate(%q) =\n  %q\nwant\n  %q", test.input, got, test.want)
			}
		})
	}
}

// Behavior 45: every parse failure carries the same message, whatever the
// library's reason. That message is the dictionary key, so the pipeline replaces
// it with spec 004 §4.9 and the reason is unobservable.
//
// The inputs are upstream's measured ones, grouped by the reason pydantic gives
// them — `relative URL without a base`, `empty host`, `invalid international
// domain name` — precisely to show the reason makes no difference.
func TestEveryParseFailureLooksTheSame(t *testing.T) {
	for _, input := range []string{
		"example.com", "not a url", "//example.com", // relative URL without a base
		"https://",             // empty host
		"https://exa mple.com", // invalid international domain name
	} {
		t.Run(input, func(t *testing.T) {
			_, err := httpurl.Validate(input)

			var urlErr *httpurl.Error
			if !errors.As(err, &urlErr) {
				t.Fatalf("err = %v (%T), want *httpurl.Error", err, err)
			}
			if urlErr.Code != httpurl.CodeURLParsing {
				t.Errorf("code = %q, want %q", urlErr.Code, httpurl.CodeURLParsing)
			}
			if urlErr.Message != httpurl.MessageURLParsing {
				t.Errorf("message = %q, want the dictionary key %q",
					urlErr.Message, httpurl.MessageURLParsing)
			}
		})
	}
}

// Behavior 45's second row. A parsed URL with any other scheme, measured on
// upstream's own example.
func TestWrongScheme(t *testing.T) {
	_, err := httpurl.Validate("ftp://example.com")

	var urlErr *httpurl.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("err = %v (%T), want *httpurl.Error", err, err)
	}
	if urlErr.Code != httpurl.CodeURLScheme {
		t.Errorf("code = %q, want %q", urlErr.Code, httpurl.CodeURLScheme)
	}
	if urlErr.Message != "URL scheme should be 'http' or 'https'" {
		t.Errorf("message = %q", urlErr.Message)
	}
}

// Behavior 46, the whole of it. Three properties, and each needs its own row:
// the limit is inclusive, it is measured on the **input** rather than on the
// serialized form, and it is checked **before** parsing.
func TestLength(t *testing.T) {
	pad := func(n int) string {
		return "https://example.com/" + strings.Repeat("a", n-len("https://example.com/"))
	}

	t.Run("2083 characters pass", func(t *testing.T) {
		if _, err := httpurl.Validate(pad(2083)); err != nil {
			t.Errorf("a 2083-character URL was rejected: %v", err)
		}
	})

	t.Run("2084 fail", func(t *testing.T) {
		_, err := httpurl.Validate(pad(2084))
		var urlErr *httpurl.Error
		if !errors.As(err, &urlErr) || urlErr.Code != httpurl.CodeURLTooLong {
			t.Fatalf("err = %v, want url_too_long", err)
		}
		if urlErr.Message != "URL should have at most 2083 characters" {
			t.Errorf("message = %q", urlErr.Message)
		}
	})

	t.Run("the input is measured, not the serialized form", func(t *testing.T) {
		// Upstream's measured case: 400 non-ASCII path characters, which
		// percent-encode to six characters each. The input is 820 bytes and the
		// serialization is 2420 — well over the limit — and it passes, because
		// the input is what is measured.
		input := "https://example.com/" + strings.Repeat("ü", 400)

		got, err := httpurl.Validate(input)
		if err != nil {
			t.Fatalf("Validate = %v", err)
		}
		if len(input) > httpurl.MaxLength {
			t.Fatalf("the test input is %d bytes, over the limit; this row cannot"+
				" show what it claims", len(input))
		}
		if len(got) <= httpurl.MaxLength {
			t.Fatalf("the serialized form is only %d characters; this row cannot"+
				" show what it claims", len(got))
		}
	})

	t.Run("the limit is UTF-8 bytes, not characters", func(t *testing.T) {
		// The two coincide for ASCII, which is how this could go unnoticed.
		// Measured against the vendored Python by bisection: 1051 non-ASCII
		// characters — 2082 bytes — pass, and 1052 — 2084 bytes — fail. If the
		// limit were 2083 *characters*, both would pass.
		pass := "https://example.com/" + strings.Repeat("ü", 1031)
		fail := "https://example.com/" + strings.Repeat("ü", 1032)

		if _, err := httpurl.Validate(pass); err != nil {
			t.Errorf("%d bytes (%d characters) was rejected: %v",
				len(pass), len([]rune(pass)), err)
		}

		var urlErr *httpurl.Error
		if _, err := httpurl.Validate(fail); !errors.As(err, &urlErr) ||
			urlErr.Code != httpurl.CodeURLTooLong {
			t.Errorf("%d bytes (%d characters) was accepted; the limit is on bytes",
				len(fail), len([]rune(fail)))
		}
	})

	t.Run("length is checked before parsing", func(t *testing.T) {
		// A URL that would fail to parse, made too long. If parsing ran first
		// the code would be url_parsing.
		_, err := httpurl.Validate("https://exa mple.com/" + strings.Repeat("a", 3000))

		var urlErr *httpurl.Error
		if !errors.As(err, &urlErr) {
			t.Fatalf("err = %v (%T)", err, err)
		}
		if urlErr.Code != httpurl.CodeURLTooLong {
			t.Errorf("code = %q, want url_too_long — the length check must run"+
				" before the parse", urlErr.Code)
		}
	})
}
