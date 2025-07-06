package bridge

import (
	"fmt"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ConnectionDesign is the slice of `design.header.connections` the list needs
// while it is being built, as opposed to `process.ConnectionOptions`, which is
// what the two formatters need afterwards.
type ConnectionDesign struct {
	PhoneNumberFormat            string
	DisplayURLsInsteadOfUsername bool
}

// Connections is `parse_connections` (connections.py:60-180).
//
// **The order is `cv._key_order`** (`:76`), the keys as the user wrote them —
// not the model's field order and not a fixed contact order. A CV that lists
// `website` before `email` renders them that way, and a key written with a null
// value never entered the order at all (spec 009 §2 behavior 10), so it
// contributes nothing here.
//
// It returns an error where upstream raises `RenderCVInternalError`: a key in
// the order whose value is missing. That is unreachable from a validated
// document — the order drops null-valued keys as it is recorded — and it is an
// error rather than a panic because the renderer's caller can report it.
func Connections(model *cv.Cv, design ConnectionDesign) ([]process.Connection, error) {
	if model == nil {
		return nil, nil
	}

	var out []process.Connection
	for _, key := range model.KeyOrder() {
		switch key {
		case "email":
			for _, email := range scalars(model.Email) {
				out = append(out, process.Connection{
					FontAwesomeIcon: process.FontAwesomeIcons["email"],
					URL:             "mailto:" + email,
					Body:            email,
				})
			}

		case "phone":
			if model.Phone == nil {
				return nil, fmt.Errorf("phone key present but value is None")
			}
			for _, phone := range scalars(model.Phone) {
				// **The URL is the stored RFC 3966 string, `tel:` and all**
				// (`:104`), while the body is that same string formatted. The
				// two differ on every number whose national form is not what
				// the user typed.
				stored, err := phonenum.Validate(phone)
				if err != nil {
					return nil, fmt.Errorf("phone %q did not re-validate: %w", phone, err)
				}
				out = append(out, process.Connection{
					FontAwesomeIcon: process.FontAwesomeIcons["phone"],
					URL:             stored,
					Body:            FormatPhone(stored, design.PhoneNumberFormat),
				})
			}

		case "website":
			if model.Website == nil {
				return nil, fmt.Errorf("website key present but value is None")
			}
			for _, website := range scalars(model.Website) {
				// `str(website)` is `pydantic.HttpUrl`'s serialization, which is
				// where a bare host gains its trailing slash — and `clean_url`
				// then takes that slash back off for the body while the link
				// keeps it.
				url := website
				if normalized, err := httpurl.Validate(website); err == nil {
					url = normalized
				}
				out = append(out, process.Connection{
					FontAwesomeIcon: process.FontAwesomeIcons["website"],
					URL:             url,
					Body:            process.CleanURL(url),
				})
			}

		case "location":
			// **The only built-in connection with no URL** (`:127-133`), so it
			// stays unlinked even with `hyperlink` on.
			out = append(out, process.Connection{
				FontAwesomeIcon: process.FontAwesomeIcons["location"],
				Body:            text(model.Location),
			})

		case "social_networks":
			if model.SocialNetworks == nil {
				return nil, fmt.Errorf("social_networks key present but value is None")
			}
			out = append(out, socialConnections(model.SocialNetworks, design)...)

		case "custom_connections":
			if model.CustomConnections == nil {
				return nil, fmt.Errorf("custom_connections key present but value is None")
			}
			out = append(out, customConnections(model.CustomConnections)...)
		}
	}
	return out, nil
}

// socialConnections is `:141-165`. The body is one of three things: the URL when
// the design asks for URLs, the literal `Google Scholar` for that one network —
// whose username is an opaque id no reader would recognize — and the username
// otherwise.
func socialConnections(node *yamldoc.Node, design ConnectionDesign) []process.Connection {
	var out []process.Connection
	for _, elem := range sequence(node) {
		network := cv.SocialNetworkName(field(elem, "network"))
		username := field(elem, "username")

		model := cv.SocialNetwork{Network: network, Username: username}
		url := model.URL()

		body := username
		switch {
		case design.DisplayURLsInsteadOfUsername:
			body = process.CleanURL(url)
		case network == cv.SocialNetworkGoogleScholar:
			body = "Google Scholar"
		}

		out = append(out, process.Connection{
			FontAwesomeIcon: process.FontAwesomeIcons[string(network)],
			URL:             url,
			Body:            body,
		})
	}
	return out
}

// customConnections is `:167-180`. The icon is the user's own string rather than
// a lookup, and the URL is optional — `custom_connections[].url` is the one
// required field whose declared type admits null (spec 004 §3.81).
func customConnections(node *yamldoc.Node) []process.Connection {
	var out []process.Connection
	for _, elem := range sequence(node) {
		url := field(elem, "url")
		if url != "" {
			if normalized, err := httpurl.Validate(url); err == nil {
				url = normalized
			}
		}
		out = append(out, process.Connection{
			FontAwesomeIcon: field(elem, "fontawesome_icon"),
			URL:             url,
			Body:            field(elem, "placeholder"),
		})
	}
	return out
}

// scalars is the `if not isinstance(x, list): x = [x]` of `:79`, `:98` and
// `:117`: the three fields are each a scalar or a list of them, and both shapes
// contribute one connection per value.
func scalars(node *yamldoc.Node) []string {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}
	if node.Kind != yamldoc.KindSequence {
		return []string{node.Raw}
	}

	out := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || elem.Kind == yamldoc.KindNull {
			continue
		}
		out = append(out, elem.Raw)
	}
	return out
}

func sequence(node *yamldoc.Node) []*yamldoc.Node {
	if node == nil || node.Kind != yamldoc.KindSequence {
		return nil
	}
	return node.Elems
}

func field(node *yamldoc.Node, name string) string {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return ""
	}
	for _, item := range node.Items {
		if item.Key == name {
			return text(item.Value)
		}
	}
	return ""
}

func text(node *yamldoc.Node) string {
	if node == nil || node.Kind == yamldoc.KindNull {
		return ""
	}
	return node.Raw
}
