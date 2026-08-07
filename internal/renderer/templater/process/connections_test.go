package process_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// The icon table against upstream's, entry for entry.
//
// Diffed rather than spot-checked because the three that differ from the
// lowercased network name — `stack-overflow`, `graduation-cap`, `x-twitter` —
// are exactly the ones a reviewer's eye slides over, and a wrong icon renders as
// a missing glyph rather than as an error.
func TestFontAwesomeIcons(t *testing.T) {
	path := filepath.Join("testdata", "fontawesome_icons.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	for name, icon := range want {
		if got := process.FontAwesomeIcons[name]; got != icon {
			t.Errorf("%s = %q, want %q", name, got, icon)
		}
	}
	for name := range process.FontAwesomeIcons {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is in the port and not upstream", name)
		}
	}
}

// The four Typst shapes the two independent flags produce, plus the fifth case
// that is not a combination: a connection with no URL cannot be linked whatever
// `hyperlink` says.
func TestFormatConnectionsForTypst(t *testing.T) {
	linked := process.Connection{FontAwesomeIcon: "envelope", URL: "mailto:a@b", Body: "a@b"}
	bare := process.LocationConnection("Berlin")

	tests := []struct {
		name    string
		in      process.Connection
		options process.ConnectionOptions
		want    string
	}{
		{
			name: "icon and link", in: linked,
			options: process.ConnectionOptions{ShowIcons: true, Hyperlink: true},
			want: `#link("mailto:a@b", icon: false, if-underline: false, if-color: false)` +
				`[#connection-with-icon("envelope")[a\@b]]`,
		},
		{
			name: "icon only", in: linked,
			options: process.ConnectionOptions{ShowIcons: true},
			want:    `#connection-with-icon("envelope")[a\@b]`,
		},
		{
			name: "link only", in: linked,
			options: process.ConnectionOptions{Hyperlink: true},
			want: `#link("mailto:a@b", icon: false, if-underline: false, if-color: false)` +
				`[a\@b]`,
		},
		{
			name: "neither", in: linked,
			options: process.ConnectionOptions{},
			want:    `a\@b`,
		},
		{
			// `location` has no URL, so `hyperlink` does nothing.
			name: "no URL is never linked", in: bare,
			options: process.ConnectionOptions{Hyperlink: true},
			want:    "Berlin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := process.FormatConnectionsForTypst(
				[]process.Connection{tc.in}, tc.options)
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// Markdown has no icons and no Typst conversion — the `@` that Typst escapes
// stays bare, which is the check that the two paths really are separate.
func TestFormatConnectionsForMarkdown(t *testing.T) {
	got := process.FormatConnectionsForMarkdown([]process.Connection{
		{FontAwesomeIcon: "envelope", URL: "mailto:a@b", Body: "a@b"},
		process.LocationConnection("Berlin"),
	})
	want := []string{"[a@b](mailto:a@b)", "Berlin"}

	if len(got) != len(want) {
		t.Fatalf("= %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// `Google Scholar` is the one network whose body is a literal, and only when
// URLs are not being displayed.
func TestSocialNetworkBody(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		displayURLs bool
		want        string
	}{
		{"an ordinary network", "GitHub", false, "johndoe"},
		{"Google Scholar", "Google Scholar", false, "Google Scholar"},
		{"an ordinary network showing URLs", "GitHub", true, "example.com/johndoe"},
		{"Google Scholar showing URLs", "Google Scholar", true, "example.com/johndoe"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := process.SocialNetworkBody(tc.network, "johndoe",
				"https://example.com/johndoe", tc.displayURLs)
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// The three built-in keys whose body is derived.
func TestDerivedConnectionBodies(t *testing.T) {
	if got := process.EmailConnection("a@b.com"); got.URL != "mailto:a@b.com" || got.Body != "a@b.com" {
		t.Errorf("email = %+v", got)
	}
	// The website's body is the cleaned URL and its link target is not.
	if got := process.WebsiteConnection("https://x.com/y/"); got.Body != "x.com/y" || got.URL != "https://x.com/y/" {
		t.Errorf("website = %+v", got)
	}
	if got := process.LocationConnection("Berlin"); got.URL != "" {
		t.Errorf("location has URL %q, want none", got.URL)
	}
}
