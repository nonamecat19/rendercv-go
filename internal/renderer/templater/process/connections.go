package process

import "strings"

// FontAwesomeIcons is `fontawesome_icons` (connections.py:12-33): twenty-one
// entries, the seventeen social networks plus `location`, `email`, `phone` and
// `website`.
//
// **Three are not the lowercased name.** A port deriving the icon from the
// network gets eighteen of twenty-one right and fails silently on the other
// three — they render as a missing glyph, not as an error.
var FontAwesomeIcons = map[string]string{
	"LinkedIn":       "linkedin",
	"GitHub":         "github",
	"GitLab":         "gitlab",
	"IMDB":           "imdb",
	"Instagram":      "instagram",
	"Mastodon":       "mastodon",
	"ORCID":          "orcid",
	"StackOverflow":  "stack-overflow", // not `stackoverflow`
	"ResearchGate":   "researchgate",
	"YouTube":        "youtube",
	"Google Scholar": "graduation-cap", // not `google-scholar`
	"Telegram":       "telegram",
	"WhatsApp":       "whatsapp",
	"Leetcode":       "code",
	"X":              "x-twitter", // not `x`
	"Bluesky":        "bluesky",
	"Reddit":         "reddit",

	"location": "location-dot",
	"email":    "envelope",
	"phone":    "phone",
	"website":  "link",
}

// Connection is one entry of the header's contact list
// (connections.py:54-58).
type Connection struct {
	FontAwesomeIcon string
	// URL is empty when the connection is not a link. `location` is the only
	// built-in key with none.
	URL  string
	Body string
}

// ConnectionOptions is the slice of `design.header.connections` the two
// formatters read.
type ConnectionOptions struct {
	ShowIcons bool
	Hyperlink bool
}

// FormatConnectionsForTypst is `compute_connections_for_typst`
// (connections.py:186-221).
//
// **Two independent flags, four shapes.** The icon wrapper and the link wrapper
// are decided separately, and the link one also requires a URL — so a `location`
// with `hyperlink` on is still unlinked.
//
// `markdown_to_typst` runs on the **body only**, never on the URL.
func FormatConnectionsForTypst(connections []Connection, options ConnectionOptions) []string {
	out := make([]string, 0, len(connections))
	for _, connection := range connections {
		body := MarkdownToTypst(connection.Body)
		if options.ShowIcons {
			body = `#connection-with-icon("` + connection.FontAwesomeIcon + `")[` + body + `]`
		}
		if connection.URL != "" && options.Hyperlink {
			body = `#link("` + connection.URL +
				`", icon: false, if-underline: false, if-color: false)[` + body + `]`
		}
		out = append(out, body)
	}
	return out
}

// FormatConnectionsForMarkdown is `compute_connections_for_markdown`
// (connections.py:223-244).
//
// **No icons at all, and no Typst conversion** — the body stays Markdown,
// because the document it lands in is Markdown. The two formatters share only
// the connection list.
func FormatConnectionsForMarkdown(connections []Connection) []string {
	out := make([]string, 0, len(connections))
	for _, connection := range connections {
		if connection.URL != "" {
			out = append(out, "["+connection.Body+"]("+connection.URL+")")
			continue
		}
		out = append(out, connection.Body)
	}
	return out
}

// SocialNetworkBody is the body of a social-network connection
// (connections.py:145-155).
//
// **`Google Scholar` is the one network whose body is a literal** rather than
// the username, and only when `display_urls_instead_of_usernames` is off. Every
// other network shows the username.
func SocialNetworkBody(network, username, url string, displayURLs bool) string {
	if displayURLs {
		return CleanURL(url)
	}
	if network == "Google Scholar" {
		return "Google Scholar"
	}
	return username
}

// ConnectionKeys is the six `cv` keys `parse_connections` matches on, in the
// order it declares them (connections.py:74-181).
//
// **It is not the order they are rendered in.** The loop walks `cv._key_order`
// — the order the user wrote the keys — and this list only says which keys are
// recognized. A port that iterated this instead would render every CV's header
// in the wrong sequence, and nothing in the schema would notice.
var ConnectionKeys = []string{
	"email", "phone", "website", "location", "social_networks", "custom_connections",
}

// IsConnectionKey reports whether a `cv` key contributes a connection.
func IsConnectionKey(key string) bool {
	for _, known := range ConnectionKeys {
		if known == key {
			return true
		}
	}
	return false
}

// EmailConnection is the `email` key: the address is both the body and, with a
// `mailto:` scheme, the link.
func EmailConnection(address string) Connection {
	return Connection{
		FontAwesomeIcon: FontAwesomeIcons["email"],
		URL:             "mailto:" + address,
		Body:            address,
	}
}

// WebsiteConnection is the `website` key. Its **body is the cleaned URL and its
// target is not** — the user sees `x.com/y` and clicks `https://x.com/y/`.
func WebsiteConnection(url string) Connection {
	return Connection{
		FontAwesomeIcon: FontAwesomeIcons["website"],
		URL:             url,
		Body:            CleanURL(url),
	}
}

// LocationConnection has **no URL**, which is what makes the `hyperlink` flag
// insufficient on its own in FormatConnectionsForTypst.
func LocationConnection(location string) Connection {
	return Connection{
		FontAwesomeIcon: FontAwesomeIcons["location"],
		Body:            location,
	}
}

// PhoneConnection takes the number twice: `formatted` is the body, rendered in
// the theme's `phone_number_format`, and `raw` is the link target — the `tel:`
// form spec 002 §3.49 already models.
func PhoneConnection(raw, formatted string) Connection {
	return Connection{
		FontAwesomeIcon: FontAwesomeIcons["phone"],
		URL:             raw,
		Body:            formatted,
	}
}

// SocialConnection is one `cv.social_networks` entry.
func SocialConnection(network, username, url string, displayURLs bool) Connection {
	return Connection{
		FontAwesomeIcon: FontAwesomeIcons[network],
		URL:             url,
		Body:            SocialNetworkBody(network, username, url, displayURLs),
	}
}

// CustomConnection is one `cv.custom_connections` entry: the icon and the body
// are the user's, and the URL may be absent.
func CustomConnection(icon, placeholder, url string) Connection {
	return Connection{
		FontAwesomeIcon: icon,
		URL:             strings.TrimSpace(url),
		Body:            placeholder,
	}
}
