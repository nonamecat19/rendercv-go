package cv_test

import (
	"strings"
	"testing"

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
		{"WhatsApp", "905419999999", "https://wa.me/905419999999"},
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
