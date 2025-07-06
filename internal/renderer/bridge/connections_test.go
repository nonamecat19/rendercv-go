package bridge_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

func connectionsOf(t *testing.T, document string, design bridge.ConnectionDesign) []process.Connection {
	t.Helper()
	got, err := bridge.Connections(validCv(t, document), design)
	if err != nil {
		t.Fatalf("Connections: %v", err)
	}
	return got
}

// Spec 009 §2 behaviors 8-9. **The keys are deliberately not in field order.**
// `cv.py`'s declaration is name, headline, location, email, photo, phone,
// website, …; this document writes website, location, email, and that is the
// order the header must show — the model's own field order would silently
// reorder every user's contact line.
func TestConnectionsFollowTheInputFileOrder(t *testing.T) {
	got := connectionsOf(t, `
name: John Doe
website: https://example.com
location: Berlin
email: john@example.com
`, bridge.ConnectionDesign{PhoneNumberFormat: "national"})

	want := []string{"link", "location-dot", "envelope"}
	if len(got) != len(want) {
		t.Fatalf("got %d connections (%v), want %d", len(got), got, len(want))
	}
	for i, icon := range want {
		if got[i].FontAwesomeIcon != icon {
			t.Errorf("connection %d icon = %q, want %q", i, got[i].FontAwesomeIcon, icon)
		}
	}
}

// Spec 009 §2 behavior 10. A key written with a null value never enters
// `_key_order`, so it contributes nothing — which is what the corpus's own
// `cv.yaml` relies on, since it writes `phone:` and `custom_connections:` with
// no value at all.
func TestNullValuedKeysContributeNothing(t *testing.T) {
	got := connectionsOf(t, `
name: John Doe
headline:
location: Berlin
email:
phone:
custom_connections:
`, bridge.ConnectionDesign{PhoneNumberFormat: "national"})

	if len(got) != 1 || got[0].Body != "Berlin" {
		t.Fatalf("got %v, want only the location", got)
	}
}

// A scalar and a list of the same field produce the same shape: one connection
// per value (`:79`, `:98`, `:117`).
func TestAListOfEmailsBecomesOneConnectionEach(t *testing.T) {
	got := connectionsOf(t, `
email:
  - a@example.com
  - b@example.com
`, bridge.ConnectionDesign{})

	if len(got) != 2 {
		t.Fatalf("got %d connections, want 2", len(got))
	}
	for i, want := range []string{"a@example.com", "b@example.com"} {
		if got[i].Body != want || got[i].URL != "mailto:"+want {
			t.Errorf("connection %d = %+v, want %q", i, got[i], want)
		}
	}
}

// The website's URL keeps `pydantic.HttpUrl`'s trailing slash and its body has
// it removed, so the link and the text differ by design.
func TestWebsiteLinksWithTheSlashAndReadsWithout(t *testing.T) {
	got := connectionsOf(t, "website: https://example.com\n", bridge.ConnectionDesign{})

	if len(got) != 1 {
		t.Fatalf("got %d connections, want 1", len(got))
	}
	if got[0].URL != "https://example.com/" {
		t.Errorf("url = %q, want the serialized form", got[0].URL)
	}
	if got[0].Body != "example.com" {
		t.Errorf("body = %q, want it cleaned", got[0].Body)
	}
}

// The phone's URL is the stored RFC 3966 string and its body is that string
// formatted — the two are not the same text.
func TestPhoneLinksToTelAndReadsFormatted(t *testing.T) {
	got := connectionsOf(t, "phone: +1-415-555-0142\n",
		bridge.ConnectionDesign{PhoneNumberFormat: "national"})

	if len(got) != 1 {
		t.Fatalf("got %d connections, want 1", len(got))
	}
	if got[0].URL != "tel:+1-415-555-0142" {
		t.Errorf("url = %q, want the stored form", got[0].URL)
	}
	if got[0].Body != "(415) 555-0142" {
		t.Errorf("body = %q, want the national form", got[0].Body)
	}
}

// Three social-network bodies, one per branch of `:145-155`.
func TestSocialNetworkBodies(t *testing.T) {
	document := `
social_networks:
  - network: GitHub
    username: rendercv
  - network: Google Scholar
    username: F8IyYrQAAAAJ
  - network: Mastodon
    username: "@user@fosstodon.org"
`

	got := connectionsOf(t, document, bridge.ConnectionDesign{})
	want := []struct{ icon, url, body string }{
		{"github", "https://github.com/rendercv", "rendercv"},
		// The one network whose body is a literal, because its username is an
		// opaque id.
		{
			"graduation-cap",
			"https://scholar.google.com/citations?user=F8IyYrQAAAAJ",
			"Google Scholar",
		},
		// Mastodon's URL is built by splitting the handle, not by concatenation.
		{"mastodon", "https://fosstodon.org/@user", "@user@fosstodon.org"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d connections, want %d", len(got), len(want))
	}
	for i, expected := range want {
		if got[i].FontAwesomeIcon != expected.icon || got[i].URL != expected.url ||
			got[i].Body != expected.body {
			t.Errorf("connection %d = %+v, want %+v", i, got[i], expected)
		}
	}
}

// `display_urls_instead_of_usernames` replaces every social body with the
// cleaned URL, including Google Scholar's literal.
func TestDisplayURLsInsteadOfUsernames(t *testing.T) {
	document := `
social_networks:
  - network: GitHub
    username: rendercv
  - network: Google Scholar
    username: F8IyYrQAAAAJ
`

	got := connectionsOf(t, document,
		bridge.ConnectionDesign{DisplayURLsInsteadOfUsername: true})

	want := []string{
		"github.com/rendercv",
		"scholar.google.com/citations?user=F8IyYrQAAAAJ",
	}
	for i, expected := range want {
		if got[i].Body != expected {
			t.Errorf("body %d = %q, want %q", i, got[i].Body, expected)
		}
	}
}

// A custom connection brings its own icon and may have no URL at all.
func TestCustomConnections(t *testing.T) {
	document := `
custom_connections:
  - fontawesome_icon: fa-brands fa-discord
    placeholder: rendercv
    url: https://discord.com/users/rendercv
  - fontawesome_icon: fa-solid fa-passport
    placeholder: Dual citizen
    url:
`

	got := connectionsOf(t, document, bridge.ConnectionDesign{})
	if len(got) != 2 {
		t.Fatalf("got %d connections, want 2", len(got))
	}
	if got[0].FontAwesomeIcon != "fa-brands fa-discord" || got[0].Body != "rendercv" {
		t.Errorf("connection 0 = %+v", got[0])
	}
	if got[1].URL != "" {
		t.Errorf("connection 1 url = %q, want none", got[1].URL)
	}
}

// The location is the one built-in connection with no URL, so it stays unlinked
// however the design is set.
func TestLocationHasNoURL(t *testing.T) {
	got := connectionsOf(t, "location: Berlin\n", bridge.ConnectionDesign{})
	if len(got) != 1 || got[0].URL != "" {
		t.Fatalf("got %+v, want an unlinked location", got)
	}
}
