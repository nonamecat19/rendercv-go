package cv

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
