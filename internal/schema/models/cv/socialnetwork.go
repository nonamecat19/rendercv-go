package cv

import (
	"errors"
	"regexp"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
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
type SocialNetwork struct {
	Network  SocialNetworkName
	Username string
}

// socialNetworkFields is SocialNetworkFields in binder form: both keys are
// required, and unknown keys are rejected (social_network.py:53-57).
var socialNetworkFields = []binder.Field{
	// `network` is a literal set, whose own non-member message covers a
	// non-string; `username` is a plain `str` (social_network.py:55), and was
	// declared with no shape, so **`username: 5` rendered a CV at exit 0** where
	// upstream reports `Input should be a valid string.` Found by the sweep spec
	// 015's own `cv.name` fix asked for.
	{Name: "network", Required: true},
	{Name: "username", Required: true, Value: binder.ValueString},
}

// CodeLiteral marks a value outside a fixed set of literals. The text is
// pydantic's, not RenderCV's.
const CodeLiteral schemaerr.Code = "literal_error"

// CodeUsername is the kind every per-network username rule raises
// (models/custom_error_types.py:6). None of the eight messages is touched by
// the dictionary.
const CodeUsername schemaerr.Code = "rendercv_other_error"

// usernameRule is one network's check. It reports the message to raise, or the
// empty string to accept.
//
// Nine of the seventeen networks have no rule and accept any username; only the
// eight below appear here (spec 004 §3.16 behavior 59).
type usernameRule func(username string) string

// usernameRules is check_username's match statement (social_network.py:82-151).
//
// **Every pattern is a full match**, `re.fullmatch` rather than `re.search`.
// Bluesky's and Reddit's additionally carry redundant `^`/`$` anchors, which are
// kept in the source strings so a diff against upstream is mechanical
// (behavior 60).
//
// All eight are here; the other nine networks accept any username.
var usernameRules = map[SocialNetworkName]usernameRule{
	SocialNetworkMastodon: func(username string) string {
		if mastodonPattern.MatchString(username) {
			return ""
		}
		return `Mastodon username should be in the format "@username@domain".`
	},
	SocialNetworkStackOverflow: func(username string) string {
		if stackOverflowPattern.MatchString(username) {
			return ""
		}
		return `StackOverflow username should be in the format "user_id/username".`
	},
	// Not a pattern: the only rule of the eight that is a prefix test
	// (social_network.py:101).
	//
	// Its message ends with a stray `"` after the final period, verbatim from
	// social_network.py:104-105. The pipeline then appends its own period, so
	// the emitted text ends `username.".` — upstream's output, and not the
	// port's to fix (spec 004 §3.16 behavior 61).
	SocialNetworkYouTube: func(username string) string {
		if !strings.HasPrefix(username, "@") {
			return ""
		}
		return `YouTube username should not start with "@". Remove "@" from the beginning of the username."`
	},
	SocialNetworkORCID: func(username string) string {
		if orcidPattern.MatchString(username) {
			return ""
		}
		return "ORCID username should be in the format 'XXXX-XXXX-XXXX-XXX'."
	},
	SocialNetworkIMDB: func(username string) string {
		if imdbPattern.MatchString(username) {
			return ""
		}
		// "name", not "username": upstream's wording for this one row.
		return "IMDB name should be in the format 'nmXXXXXXX'."
	},
	SocialNetworkBluesky: func(username string) string {
		if blueskyPattern.MatchString(username) {
			return ""
		}
		return "Bluesky username should be a valid handle with no '@'" +
			" (e.g., 'username.bsky.social' or 'domain.com')."
	},
	// The one rule that is not syntactic: the username must validate as a phone
	// number (social_network.py:129-140). Upstream catches the library's failure
	// and replaces it wholesale, so the phone message never surfaces here.
	SocialNetworkWhatsApp: func(username string) string {
		if _, err := phonenum.Validate(username); err == nil {
			return ""
		}
		return "WhatsApp username should be your phone number with country" +
			" code in international format (e.g., +1 for USA, +44 for UK)."
	},
	SocialNetworkReddit: func(username string) string {
		if redditPattern.MatchString(username) {
			return ""
		}
		return "Reddit username should be made up of uppercase/lowercase letters," +
			" numbers, underscores, and hyphens between 3 and 23 characters."
	},
}

// mastodonPattern is `@[^@]+@[^@]+` (social_network.py:86-87), anchored here
// because Go's MatchString is a search and upstream's is a full match.
var mastodonPattern = regexp.MustCompile(`^@[^@]+@[^@]+$`)

// stackOverflowPattern is `\d+\/[^\/]+` (social_network.py:93-94). Go needs no
// escape before a slash.
var stackOverflowPattern = regexp.MustCompile(`^\d+/[^/]+$`)

// orcidPattern is `\d{4}-\d{4}-\d{4}-\d{3}[\dX]` (social_network.py:108-109).
// The last group takes a digit or a literal uppercase X, which is the ISO 7064
// check character an ORCID can end with.
var orcidPattern = regexp.MustCompile(`^\d{4}-\d{4}-\d{4}-\d{3}[\dX]$`)

// imdbPattern is `nm\d{7}` (social_network.py:115-116): exactly seven digits
// after a lowercase `nm`.
var imdbPattern = regexp.MustCompile(`^nm\d{7}$`)

// blueskyPattern is social_network.py:122, copied verbatim including its
// redundant `^` and `$` — the rule is a full match anyway, and keeping the
// anchors makes the diff against upstream mechanical.
//
// It is a DNS-style hostname: at least two dot-separated labels, each starting
// and ending with an alphanumeric and up to 63 characters long.
// redditPattern is `^[a-zA-Z0-9_-]{3,23}$` (social_network.py:142-143), whose
// anchors are upstream's own and redundant under a full match.
var redditPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,23}$`)

var blueskyPattern = regexp.MustCompile(
	`^([a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// checkUsername mirrors check_username (social_network.py:59-151).
//
// It runs **only** when a valid network is already present: an absent or unknown
// network returns the username unchecked and lets the network failure stand
// alone (social_network.py:76-80). That is why the caller reaches it after the
// literal check and not before.
func checkUsername(
	network SocialNetworkName,
	username string,
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	rule, ok := usernameRules[network]
	if !ok {
		return nil
	}
	message := rule(username)
	if message == "" {
		return nil
	}

	var span *yamldoc.Span
	if node != nil {
		copied := node.Span
		span = &copied
	}
	return []schemaerr.ValidationError{{
		Code:           CodeUsername,
		SchemaLocation: fieldLocation(location, "username"),
		YamlLocation:   span,
		YamlSource:     source,
		Message:        message,
		Input:          schemaerr.RenderInput(node),
	}}
}

// messageUnknownNetwork is pydantic's `literal_error` enumeration for
// `SocialNetworkName` (social_network.py:13-31): every name in the literal
// type's declared order, comma-separated, with `or` before the last and no
// serial comma. No dictionary row matches it, so the pipeline appends a period.
//
// Built from SocialNetworkNames rather than written out, so it cannot drift from
// the list it enumerates — the failure mode a hand-written string has is naming
// sixteen networks after a seventeenth is added.
var messageUnknownNetwork = buildUnknownNetworkMessage()

func buildUnknownNetworkMessage() string {
	members := make([]string, 0, len(SocialNetworkNames))
	for _, name := range SocialNetworkNames {
		members = append(members, string(name))
	}
	return binder.LiteralMessage(members, "Input should be a valid social network")
}

// urlPrefixes is `url_dictionary` (social_network.py:33-50): the profile-URL
// prefix each network's username is appended to.
//
// Mastodon is absent on purpose — its URL is built by splitting the handle, not
// by concatenation (social_network.py:178-180).
var urlPrefixes = map[SocialNetworkName]string{
	SocialNetworkLinkedIn:      "https://linkedin.com/in/",
	SocialNetworkGitHub:        "https://github.com/",
	SocialNetworkGitLab:        "https://gitlab.com/",
	SocialNetworkIMDB:          "https://imdb.com/name/",
	SocialNetworkInstagram:     "https://instagram.com/",
	SocialNetworkORCID:         "https://orcid.org/",
	SocialNetworkStackOverflow: "https://stackoverflow.com/users/",
	SocialNetworkResearchGate:  "https://researchgate.net/profile/",
	SocialNetworkYouTube:       "https://youtube.com/@",
	SocialNetworkGoogleScholar: "https://scholar.google.com/citations?user=",
	SocialNetworkTelegram:      "https://t.me/",
	SocialNetworkWhatsApp:      "https://wa.me/",
	SocialNetworkLeetcode:      "https://leetcode.com/u/",
	SocialNetworkX:             "https://x.com/",
	SocialNetworkBluesky:       "https://bsky.app/profile/",
	SocialNetworkReddit:        "https://reddit.com/user/",
}

// URL is the generated profile URL (social_network.py:170-184).
//
// A Mastodon handle is `@user@domain`, so splitting on `@` gives three parts and
// the URL is built from the last two. Every other network is prefix plus
// username.
func (s *SocialNetwork) URL() string {
	if s.Network == SocialNetworkMastodon {
		parts := strings.Split(s.Username, "@")
		if len(parts) != 3 {
			// Upstream unpacks into exactly three names and raises otherwise;
			// the username rule of spec 004 §4.1 rejects such a handle before
			// this is reached, so there is nothing to report here.
			return ""
		}
		return "https://" + parts[2] + "/@" + parts[1]
	}
	return urlPrefixes[s.Network] + s.Username
}

// validateGeneratedURL mirrors validate_generated_url
// (social_network.py:153-165).
//
// **It validates and discards.** Upstream calls the URL adapter for its side
// effect and throws the normalized value away, so the raw concatenation is what
// renders. Measured: a LinkedIn username of `not a valid %%^&*()` produces no
// error — the generated URL parses — and the spaces survive into output.
// `wrong_input.yaml:11-12` writes exactly that username and the expected-errors
// fixture has no record for `cv.social_networks.0` (spec 004 §3.13 behavior 44).
//
// So this is the one URL site whose *normalization* must not be kept. Assigning
// the return value here would silently rewrite users' profile links.
func validateGeneratedURL(
	model *SocialNetwork,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	url := model.URL()
	if url == "" {
		return nil
	}

	var urlErr *httpurl.Error
	if _, err := httpurl.Validate(url); errors.As(err, &urlErr) {
		return []schemaerr.ValidationError{{
			Code:           urlErr.Code,
			SchemaLocation: append([]string(nil), location...),
			YamlSource:     source,
			Message:        urlErr.Message,
			Input:          url,
		}}
	}
	return nil
}

// ValidateSocialNetwork binds a social-network record: two required fields, no
// unknown keys, and a `network` drawn from the seventeen names (spec §3.80).
func ValidateSocialNetwork(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) (*SocialNetwork, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: socialNetworkFields, Policy: binder.ForbidExtra, Model: "SocialNetwork"},
		location,
		source,
	)

	model := &SocialNetwork{}
	if value, ok := result.Value("username"); ok && value != nil {
		model.Username = value.Raw
	}

	value, ok := result.Value("network")
	if !ok || value == nil {
		return model, append(errs, result.ExtraErrors...)
	}

	// Membership is tested on the **node**, not on its text. `network` is a
	// `Literal[...]` (social_network.py:13-31, :54), and a `TaggedScalar` is no
	// more a member of a literal set of strings than the integer `5` is — so
	// `network: !!str LinkedIn` earns the same `literal_error` a misspelled name
	// does, with its own text in the Input Value column (measured: 2155 bytes,
	// exit 1, where the port rendered five artifacts at exit 0).
	//
	// The field is deliberately not declared ValueString: upstream never reports
	// `string_type` here. Every non-string reaches the literal enumeration
	// instead, measured on `5`, `null` and a mapping.
	name := SocialNetworkName(value.Raw)
	if !isTextNode(value) || !IsSocialNetworkName(name) {
		span := value.Span
		errs = append(errs, schemaerr.ValidationError{
			Code:           CodeLiteral,
			SchemaLocation: fieldLocation(location, "network"),
			YamlLocation:   &span,
			YamlSource:     source,
			Message:        messageUnknownNetwork,
			Input:          schemaerr.RenderInput(value),
		})
		return model, append(errs, result.ExtraErrors...)
	}

	model.Network = name

	// The username rules run only once the network is known to be valid — and
	// only once the username is known to be a **string**. A field validator is
	// pydantic's after-validator, so it never runs on a value that failed the
	// declared type: `username: 905419999999` (an integer to YAML) reports
	// `Input should be a valid string.` alone upstream, not that plus the
	// per-network rule. Measured on WhatsApp, whose usernames are phone numbers
	// and so the likeliest to be written unquoted.
	usernameNode, _ := result.Value("username")
	if !isTextNode(usernameNode) {
		// A non-string username has already failed its declared type; neither
		// the per-network rule nor the model-level URL check can run on it.
		return model, append(errs, result.ExtraErrors...)
	}
	usernameErrs := checkUsername(name, model.Username, usernameNode, location, source)
	errs = append(errs, usernameErrs...)

	// The generated URL is checked last, as a model-level validator upstream.
	// A username the rules rejected never reaches it: upstream's field validator
	// raises before the model validator runs.
	if len(usernameErrs) == 0 {
		errs = append(errs, validateGeneratedURL(model, location, source)...)
	}
	return model, append(errs, result.ExtraErrors...)
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

// isTextNode reports whether a node is a real string — the precondition every
// after-validator in this file shares.
func isTextNode(node *yamldoc.Node) bool {
	return node != nil && node.Kind == yamldoc.KindString
}
