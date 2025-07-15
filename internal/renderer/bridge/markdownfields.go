package bridge

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// MarkdownFields is `cv`'s five contact fields as the Markdown header reads
// them (spec 011 §2).
//
// **None of them is processed** (`model_processor.py:88-95` touches only the
// name, the headline and the sections), so an underscore in an email reaches the
// Markdown unescaped — correct there, and exactly what the Typst path escapes.
//
// The formatting they need is in the template, not here: the phone's `tel:` and
// its hyphens go through two `replace` filters, and the website is printed once
// cleaned and once whole.
func MarkdownFields(doc Document) map[string]any {
	model := doc.Model
	if model == nil || model.CvModel == nil {
		return nil
	}
	block := map[string]any{}

	// A scalar-or-list field contributes its **first** value here: the template
	// prints one `- Phone:` line, not one per number. Upstream prints
	// `str(cv.phone)` of the whole field, which for a list is Python's list
	// repr — a shape no corpus case has and no user would want.
	// **The phone is the *stored* RFC 3966 string**, `tel:` and all: the
	// template strips the scheme and turns the hyphens into spaces, and
	// `PhoneNumber` re-groups on validation, so `+34-612-345-678` prints as
	// `+34 612 34 56 78` rather than as the user's own grouping.
	block["phone"] = storedPhone(model.CvModel.Phone)
	block["email"] = firstScalar(model.CvModel.Email)
	block["location"] = text(model.CvModel.Location)
	block["website"] = websiteOf(model.CvModel.Website)
	block["social_networks"] = socialNetworksOf(model.CvModel.SocialNetworks)
	return block
}

func storedPhone(node *yamldoc.Node) string {
	raw := firstScalar(node)
	if raw == "" {
		return ""
	}
	if stored, err := phonenum.Validate(raw); err == nil {
		return stored
	}
	return raw
}

func firstScalar(node *yamldoc.Node) string {
	values := scalars(node)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// websiteOf serializes the website the way `pydantic.HttpUrl` does, which is
// where a bare host gains its trailing slash — the template then strips it for
// the link text and keeps it in the href.
func websiteOf(node *yamldoc.Node) string {
	raw := firstScalar(node)
	if raw == "" {
		return ""
	}
	if normalized, err := httpurl.Validate(raw); err == nil {
		return normalized
	}
	return raw
}

// socialNetworksOf gives the template the three names it reads per network. The
// URL is the generated profile URL, uncleaned — the Markdown link text is the
// username, so nothing here is shortened.
func socialNetworksOf(node *yamldoc.Node) []map[string]any {
	if node == nil || node.Kind != yamldoc.KindSequence {
		return nil
	}

	out := make([]map[string]any, 0, len(node.Elems))
	for _, elem := range node.Elems {
		network := cv.SocialNetworkName(field(elem, "network"))
		username := field(elem, "username")
		model := cv.SocialNetwork{Network: network, Username: username}
		out = append(out, map[string]any{
			"network":  string(network),
			"username": username,
			"url":      model.URL(),
		})
	}
	return out
}
