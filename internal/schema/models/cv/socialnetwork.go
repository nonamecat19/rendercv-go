package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// SocialNetworkName mirrors the SocialNetworkName literal type of
// schema/models/cv/social_network.py:13-31 (spec §3.80): the network field of
// a social-network record is restricted to one of seventeen platform names.
type SocialNetworkName string

// The seventeen valid social-network names, in upstream's declared order
// (schema/models/cv/social_network.py:13-31).
const (
	SocialNetworkLinkedIn      SocialNetworkName = "LinkedIn"
	SocialNetworkGitHub        SocialNetworkName = "GitHub"
	SocialNetworkGitLab        SocialNetworkName = "GitLab"
	SocialNetworkIMDB          SocialNetworkName = "IMDB"
	SocialNetworkInstagram     SocialNetworkName = "Instagram"
	SocialNetworkORCID         SocialNetworkName = "ORCID"
	SocialNetworkMastodon      SocialNetworkName = "Mastodon"
	SocialNetworkStackOverflow SocialNetworkName = "StackOverflow"
	SocialNetworkResearchGate  SocialNetworkName = "ResearchGate"
	SocialNetworkYouTube       SocialNetworkName = "YouTube"
	SocialNetworkGoogleScholar SocialNetworkName = "Google Scholar"
	SocialNetworkTelegram      SocialNetworkName = "Telegram"
	SocialNetworkWhatsApp      SocialNetworkName = "WhatsApp"
	SocialNetworkLeetcode      SocialNetworkName = "Leetcode"
	SocialNetworkX             SocialNetworkName = "X"
	SocialNetworkBluesky       SocialNetworkName = "Bluesky"
	SocialNetworkReddit        SocialNetworkName = "Reddit"
)

// SocialNetworkNames is available_social_networks
// (schema/models/cv/social_network.py:32): the seventeen valid names, in
// upstream's declared order.
var SocialNetworkNames = []SocialNetworkName{
	SocialNetworkLinkedIn,
	SocialNetworkGitHub,
	SocialNetworkGitLab,
	SocialNetworkIMDB,
	SocialNetworkInstagram,
	SocialNetworkORCID,
	SocialNetworkMastodon,
	SocialNetworkStackOverflow,
	SocialNetworkResearchGate,
	SocialNetworkYouTube,
	SocialNetworkGoogleScholar,
	SocialNetworkTelegram,
	SocialNetworkWhatsApp,
	SocialNetworkLeetcode,
	SocialNetworkX,
	SocialNetworkBluesky,
	SocialNetworkReddit,
}

// SocialNetworkFields is the ordered set of keys a social-network record
// accepts. SocialNetwork is a "without extra keys" model
// (schema/models/cv/social_network.py:53, spec §3.13, §3.80), so any key
// outside this set is rejected by the binder.
var SocialNetworkFields = []string{"network", "username"}

// SocialNetwork is the shell of schema/models/cv/social_network.py's
// SocialNetwork model: both fields are required, with no default
// (social_network.py:54-57, spec §3.80).
//
// TODO(iteration-4): per-network username pattern validation and the
// generated profile URL are out of scope for this shell
// (social_network.py:59-184, spec §7).
type SocialNetwork struct {
	Network  SocialNetworkName
	Username string
}

// socialNetworkFields is SocialNetworkFields in binder form: both keys are
// required, and unknown keys are rejected (social_network.py:53-57).
var socialNetworkFields = []binder.Field{
	{Name: "network", Required: true},
	{Name: "username", Required: true},
}

// CodeLiteral marks a value outside a fixed set of literals.
//
// TODO(iteration-4): spec §7.3 — the text of this failure is pydantic's
// `literal_error`, not RenderCV's, and is pinned with the other borrowed
// strings in iteration 4.
const CodeLiteral schemaerr.Code = "literal_error"

// ValidateSocialNetwork binds a social-network record: two required fields, no
// unknown keys, and a `network` drawn from the seventeen names (spec §3.80).
func ValidateSocialNetwork(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) (*SocialNetwork, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: socialNetworkFields, Policy: binder.ForbidExtra},
		location,
		source,
	)

	model := &SocialNetwork{}
	if value, ok := result.Value("username"); ok && value != nil {
		model.Username = value.Raw
	}

	value, ok := result.Value("network")
	if !ok || value == nil {
		return model, errs
	}

	name := SocialNetworkName(value.Raw)
	if !IsSocialNetworkName(name) {
		span := value.Span
		errs = append(errs, schemaerr.ValidationError{
			Code:           CodeLiteral,
			SchemaLocation: fieldLocation(location, "network"),
			YamlLocation:   &span,
			YamlSource:     source,
			Message:        "Input should be one of the supported social networks",
			Input:          schemaerr.RenderInput(value),
		})
		return model, errs
	}

	model.Network = name
	return model, errs
}

// IsSocialNetworkName reports whether a name is one of the seventeen
// (spec §3.80).
func IsSocialNetworkName(name SocialNetworkName) bool {
	for _, known := range SocialNetworkNames {
		if known == name {
			return true
		}
	}
	return false
}

func fieldLocation(location []string, key string) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, key)
}
