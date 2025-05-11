package cv_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// usernameCase is one row of spec 004 §3.16 behavior 59's table: a network, a
// username, and the message it must produce (empty to accept).
type usernameCase struct {
	network  string
	username string
	want     string
}

// runUsernameCases asserts the message verbatim, the code, and the location,
// which behavior 59 makes uniform across all eight rules.
func runUsernameCases(t *testing.T, cases []usernameCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.network+"/"+test.username, func(t *testing.T) {
			_, errs := cv.ValidateSocialNetwork(
				parse(t, "network: \""+test.network+"\"\nusername: \""+test.username+"\"\n"),
				[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
			)

			if test.want == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, test.want)
			}
			if errs[0].Code != "rendercv_other_error" {
				t.Errorf("code = %q, want rendercv_other_error", errs[0].Code)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.social_networks.0.username" {
				t.Errorf("location = %q, want it at username", got)
			}
		})
	}
}

// Spec 004 §4.1. Full match of `@[^@]+@[^@]+`.
func TestMastodonUsername(t *testing.T) {
	const want = `Mastodon username should be in the format "@username@domain".`

	runUsernameCases(t, []usernameCase{
		{"Mastodon", "@johndoe@mastodon.social", ""},
		{"Mastodon", "@a@b", ""},
		{"Mastodon", "johndoe", want},
		{"Mastodon", "@johndoe", want},
		{"Mastodon", "johndoe@mastodon.social", want},
		{"Mastodon", "@johndoe@mastodon.social@extra", want},
		{"Mastodon", "", want},
		// Full match, not a search: a valid handle with anything before it is
		// still a failure.
		{"Mastodon", "x@johndoe@mastodon.social", want},
	})
}

// Behavior 58: the username rules run only when the network is already valid. An
// unknown network reports once, about the network, and the username is not
// checked at all.
func TestUsernameRulesNeedAValidNetwork(t *testing.T) {
	_, errs := cv.ValidateSocialNetwork(
		parse(t, "network: Nope\nusername: johndoe\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if last := errs[0].SchemaLocation[len(errs[0].SchemaLocation)-1]; last != "network" {
		t.Errorf("location ends %q, want network", last)
	}
}

// A trailing space passes the Mastodon rule — `[^@]+` matches it — and then
// fails as a URL. Measured upstream, which reports exactly the same way, at the
// record rather than at the field.
//
// It is here rather than in the table above because it is the one Mastodon input
// that reaches the *second* validator, and reading it as a username-rule row
// would suggest the rule rejects it.
func TestMastodonTrailingSpaceFailsAsAURL(t *testing.T) {
	_, errs := cv.ValidateSocialNetwork(
		parse(t, "network: Mastodon\nusername: \"@johndoe@mastodon.social \"\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != "url_parsing" {
		t.Errorf("code = %q, want url_parsing", errs[0].Code)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "cv.social_networks.0" {
		t.Errorf("location = %q, want the record's", got)
	}
}

// Spec 004 §4.2. Full match of `\d+/[^/]+`.
func TestStackOverflowUsername(t *testing.T) {
	const want = `StackOverflow username should be in the format "user_id/username".`

	runUsernameCases(t, []usernameCase{
		{"StackOverflow", "12345/john", ""},
		{"StackOverflow", "1/a", ""},
		// The second part takes anything but a slash, spaces included.
		{"StackOverflow", "12345/john doe", ""},
		{"StackOverflow", "johndoe", want},
		{"StackOverflow", "12345", want},
		{"StackOverflow", "12345/", want},
		{"StackOverflow", "/john", want},
		{"StackOverflow", "12345/john/extra", want},
		{"StackOverflow", "abc/john", want},
		{"StackOverflow", "", want},
	})
}

// Spec 004 §4.3, the only rule of the eight that is a prefix test rather than a
// pattern.
//
// Its message ends with a stray `"` after the final period. That is verbatim
// from upstream and is not a transcription slip here; the pipeline then appends
// its own period, so what a user sees ends `username.".`.
func TestYouTubeUsername(t *testing.T) {
	const want = `YouTube username should not start with "@". Remove "@" from the beginning of the username."`

	runUsernameCases(t, []usernameCase{
		{"YouTube", "johndoe", ""},
		{"YouTube", "john@doe", ""},
		{"YouTube", "", ""},
		{"YouTube", "@johndoe", want},
		{"YouTube", "@", want},
	})

	if !strings.HasSuffix(want, `."`) {
		t.Errorf("the message no longer ends with the stray quote: %q", want)
	}
}

// The stray quote survives the pipeline, which appends its period after it.
func TestYouTubeMessageEndsWithQuotePeriod(t *testing.T) {
	_, errs := cv.ValidateSocialNetwork(
		parse(t, "network: YouTube\nusername: \"@johndoe\"\n"),
		[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasSuffix(final[0].Message, `.".`) {
		t.Errorf("final message = %q, want it to end `.\".`", final[0].Message)
	}
}

// Spec 004 §4.4. Full match of `\d{4}-\d{4}-\d{4}-\d{3}[\dX]`.
func TestORCIDUsername(t *testing.T) {
	const want = "ORCID username should be in the format 'XXXX-XXXX-XXXX-XXX'."

	runUsernameCases(t, []usernameCase{
		{"ORCID", "0000-0002-1825-0097", ""},
		// The check character may be a literal uppercase X.
		{"ORCID", "0000-0002-1825-009X", ""},
		// Lowercase is not the same character.
		{"ORCID", "0000-0002-1825-009x", want},
		{"ORCID", "0000-0002-1825-00970", want},
		{"ORCID", "0000-0002-1825-009", want},
		{"ORCID", "000-0002-1825-0097", want},
		{"ORCID", "0000000218250097", want},
		{"ORCID", "johndoe", want},
		{"ORCID", "", want},
	})
}

// Spec 004 §4.5. Full match of `nm\d{7}`.
//
// The message says "IMDB name", not "IMDB username" — upstream's wording for
// this one row, and the kind of thing a porter smooths out without noticing.
func TestIMDBUsername(t *testing.T) {
	const want = "IMDB name should be in the format 'nmXXXXXXX'."

	runUsernameCases(t, []usernameCase{
		{"IMDB", "nm0000001", ""},
		{"IMDB", "nm1234567", ""},
		{"IMDB", "nm123456", want},
		{"IMDB", "nm12345678", want},
		{"IMDB", "NM0000001", want},
		{"IMDB", "0000001", want},
		{"IMDB", "johndoe", want},
		{"IMDB", "", want},
	})

	if strings.Contains(want, "username") {
		t.Errorf("the message says username; upstream says name: %q", want)
	}
}

// Spec 004 §4.6. A DNS-style handle: at least two labels, each bounded by
// alphanumerics.
func TestBlueskyUsername(t *testing.T) {
	const want = "Bluesky username should be a valid handle with no '@'" +
		" (e.g., 'username.bsky.social' or 'domain.com')."

	runUsernameCases(t, []usernameCase{
		{"Bluesky", "john.bsky.social", ""},
		{"Bluesky", "domain.com", ""},
		{"Bluesky", "a.b", ""},
		{"Bluesky", "my-handle.bsky.social", ""},
		// One label is not enough — the pattern requires at least one dot.
		{"Bluesky", "johndoe", want},
		{"Bluesky", "@john.bsky.social", want},
		{"Bluesky", ".bsky.social", want},
		{"Bluesky", "john..social", want},
		{"Bluesky", "-john.bsky.social", want},
		{"Bluesky", "john-.bsky.social", want},
		{"Bluesky", "john.bsky.social.", want},
		{"Bluesky", "", want},
	})
}

// Spec 004 §4.7, the one rule of the eight that is not syntactic: the username
// must validate as a phone number.
//
// Upstream catches the phone library's failure and replaces it wholesale, so
// `value is not a valid phone number` never reaches a user through this path —
// which the last row asserts, since that is the message the same library
// produces for `cv.phone`.
func TestWhatsAppUsername(t *testing.T) {
	const want = "WhatsApp username should be your phone number with country" +
		" code in international format (e.g., +1 for USA, +44 for UK)."

	runUsernameCases(t, []usernameCase{
		{"WhatsApp", "+905419999999", ""},
		{"WhatsApp", "+1-415-555-0142", ""},
		{"WhatsApp", "+44 20 1234 5678", ""},
		// No default region, so a national number is not a phone number here.
		{"WhatsApp", "4155550142", want},
		{"WhatsApp", "johndoe", want},
		{"WhatsApp", "", want},
	})

	if strings.Contains(want, "value is not a valid phone number") {
		t.Errorf("the library's own message leaked into the rule: %q", want)
	}
}

// Spec 004 §4.24. Full match of `^[a-zA-Z0-9_-]{3,23}$`.
func TestRedditUsername(t *testing.T) {
	const want = "Reddit username should be made up of uppercase/lowercase letters," +
		" numbers, underscores, and hyphens between 3 and 23 characters."

	runUsernameCases(t, []usernameCase{
		{"Reddit", "johndoe", ""},
		{"Reddit", "abc", ""},
		{"Reddit", strings.Repeat("a", 23), ""},
		{"Reddit", "john_doe-1", ""},
		{"Reddit", "ab", want},
		{"Reddit", strings.Repeat("a", 24), want},
		{"Reddit", "john.doe", want},
		{"Reddit", "john doe", want},
		{"Reddit", "", want},
	})
}

// All eight rules exist and the other nine networks accept anything. Asserted as
// a count so a ninth rule cannot be added without deciding it belongs.
func TestEightNetworksHaveAUsernameRule(t *testing.T) {
	withRules := map[string]bool{
		"Mastodon": true, "StackOverflow": true, "YouTube": true, "ORCID": true,
		"IMDB": true, "Bluesky": true, "WhatsApp": true, "Reddit": true,
	}

	// Two probes rather than one: `x` is rejected by every syntactic rule and by
	// the phone check, but YouTube's rule only fires on a leading `@`. A network
	// has a rule if either probe reports at `username`.
	for _, name := range cv.SocialNetworkNames {
		hasRule := false
		for _, username := range []string{"x", "@x"} {
			_, errs := cv.ValidateSocialNetwork(
				parse(t, "network: \""+string(name)+"\"\nusername: \""+username+"\"\n"),
				[]string{"cv", "social_networks", "0"}, schemaerr.SourceMain,
			)
			if len(errs) > 0 &&
				errs[0].SchemaLocation[len(errs[0].SchemaLocation)-1] == "username" {
				hasRule = true
			}
		}

		if hasRule != withRules[string(name)] {
			t.Errorf("%s: has a username rule = %v, want %v", name, hasRule, withRules[string(name)])
		}
	}

	if len(withRules) != 8 {
		t.Errorf("the expectation lists %d networks, want 8", len(withRules))
	}
}
