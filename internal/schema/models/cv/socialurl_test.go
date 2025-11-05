package cv_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.13 behavior 44: the generated URL is **validated and discarded**.
//
// This is the one URL site whose normalization must not be kept. Upstream calls
// the adapter for its side effect and throws the value away, so the raw
// concatenation is what renders — and `wrong_input.yaml:11-12` writes a LinkedIn
// username of `not a valid %%^&*()` for which the expected-errors fixture has no
// record at all.
func TestGeneratedURLIsValidatedNotNormalized(t *testing.T) {
	model, errs := cv.ValidateSocialNetwork(
		parse(t, "network: LinkedIn\nusername: not a valid %%^&*()\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none — upstream reports nothing for this", errs)
	}

	// The spaces survive: a normalized URL would percent-encode them.
	const want = "https://linkedin.com/in/not a valid %%^&*()"
	if got := model.URL(); got != want {
		t.Errorf("URL() = %q, want the raw concatenation %q", got, want)
	}
}

// The prefix table, one row per network, plus Mastodon's split.
func TestGeneratedURLs(t *testing.T) {
	tests := []struct {
		network, username, want string
	}{
		{"LinkedIn", "johndoe", "https://linkedin.com/in/johndoe"},
		{"GitHub", "johndoe", "https://github.com/johndoe"},
		{"GitLab", "johndoe", "https://gitlab.com/johndoe"},
		{"IMDB", "nm0000001", "https://imdb.com/name/nm0000001"},
		{"Instagram", "johndoe", "https://instagram.com/johndoe"},
		{"ORCID", "0000-0002-1825-0097", "https://orcid.org/0000-0002-1825-0097"},
		{"StackOverflow", "12345/john", "https://stackoverflow.com/users/12345/john"},
		{"ResearchGate", "John-Doe", "https://researchgate.net/profile/John-Doe"},
		{"YouTube", "johndoe", "https://youtube.com/@johndoe"},
		{"Google Scholar", "abc123", "https://scholar.google.com/citations?user=abc123"},
		{"Telegram", "johndoe", "https://t.me/johndoe"},
		// **Quoted, and it has to be.** YAML resolves `+905419999999` to an
		// integer (measured through upstream's own loader), and `username` is
		// `str` (social_network.py:55), so an unquoted phone number is
		// `Input should be a valid string.` upstream — see
		// TestNumericUsernameIsRejected. The port used to render it.
		{"WhatsApp", `"+905419999999"`, "https://wa.me/+905419999999"},
		{"Leetcode", "johndoe", "https://leetcode.com/u/johndoe"},
		{"X", "johndoe", "https://x.com/johndoe"},
		{"Bluesky", "john.bsky.social", "https://bsky.app/profile/john.bsky.social"},
		{"Reddit", "johndoe", "https://reddit.com/user/johndoe"},
		// Mastodon is not in the prefix table: the handle is split and the URL
		// built from the domain and the user (social_network.py:178-180).
		{"Mastodon", `"@johndoe@mastodon.social"`, "https://mastodon.social/@johndoe"},
	}

	for _, test := range tests {
		t.Run(test.network, func(t *testing.T) {
			model, errs := cv.ValidateSocialNetwork(
				parse(t, "network: "+test.network+"\nusername: "+test.username+"\n"),
				[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
			)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if got := model.URL(); got != test.want {
				t.Errorf("URL() = %q, want %q", got, test.want)
			}
		})
	}

	// Every one of the seventeen names is covered, so a new network cannot be
	// added to the enum without a row here.
	if len(tests) != len(cv.SocialNetworkNames) {
		t.Errorf("the table has %d rows and there are %d networks",
			len(tests), len(cv.SocialNetworkNames))
	}
}

// A username that makes the generated URL unparseable is reported at the record,
// not at a field — upstream's is a model-level validator.
func TestGeneratedURLFailureIsReported(t *testing.T) {
	_, errs := cv.ValidateSocialNetwork(
		parse(t, "network: LinkedIn\nusername: \""+strings.Repeat("a", 2100)+"\"\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != "url_too_long" {
		t.Errorf("code = %q, want url_too_long", errs[0].Code)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.social_networks.0" {
		t.Errorf("location = %q, want the record's", got)
	}
}

// Spec 004 §4.23: pydantic's literal_error enumeration, in the literal type's
// declared order, with `or` before the last and no serial comma.
//
// Asserted as the whole literal rather than as a shape, because the order is the
// declaration order and nothing else would catch a reordering.
func TestUnknownNetworkMessage(t *testing.T) {
	_, errs := cv.ValidateSocialNetwork(
		parse(t, "network: Nope\nusername: johndoe\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	const want = "Input should be 'LinkedIn', 'GitHub', 'GitLab', 'IMDB', 'Instagram'," +
		" 'ORCID', 'Mastodon', 'StackOverflow', 'ResearchGate', 'YouTube'," +
		" 'Google Scholar', 'Telegram', 'WhatsApp', 'Leetcode', 'X', 'Bluesky'" +
		" or 'Reddit'"
	if errs[0].Message != want {
		t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, want)
	}
	if errs[0].Code != "literal_error" {
		t.Errorf("code = %q, want literal_error", errs[0].Code)
	}

	// No dictionary row matches, so the pipeline only appends a period.
	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if final[0].Message != want+"." {
		t.Errorf("final message = %q, want the raw text plus a period", final[0].Message)
	}
}

// A username YAML resolves to a number is not a string, and upstream says so:
// `username: +905419999999` reports `Input should be a valid string.` with the
// integer's own `str()` in the Input Value column — `905419999999`, the `+`
// gone. The port rendered a CV instead, because `username` was declared with no
// shape at all.
//
// The Input Value column is **still** the port's raw token here rather than
// Python's `str(int)`, which is the numeric-repr gap deferred since iteration
// 14's pass 13 — this is the most plausible trigger for it found so far, since a
// WhatsApp username is a phone number.
func TestNumericUsernameIsRejected(t *testing.T) {
	for _, username := range []string{"+905419999999", "905419999999", "0x1f", "1_000"} {
		t.Run(username, func(t *testing.T) {
			_, errs := cv.ValidateSocialNetwork(
				parse(t, "network: WhatsApp\nusername: "+username+"\n"),
				[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != "Input should be a valid string" {
				t.Errorf("message = %q, want the string-type message", errs[0].Message)
			}
		})
	}
}
