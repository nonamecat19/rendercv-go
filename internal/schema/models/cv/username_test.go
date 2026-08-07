package cv_test

import (
	"strings"
	"testing"

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
