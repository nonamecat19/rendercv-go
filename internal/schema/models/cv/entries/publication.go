package entries

import (
	"errors"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Description metadata of BasePublicationEntry's fields (publication.py:19-44,
// spec §4.9-§4.12). They are not error text; iteration 5 emits them verbatim
// into the JSON Schema. `title` and `summary` carry none (publication.py:13-26).
const (
	AuthorsDescription = "You can bold your name with **double asterisks**."
	DOIDescription     = "The DOI (Digital Object Identifier). If provided, it will be" +
		" used as the link instead of the URL."
	URLDescription     = "A URL link to the publication. Ignored if DOI is provided."
	JournalDescription = "The journal, conference, or venue where it was published."
)

// DOIURLPrefix is the prefix DOIURL prepends to a `doi`
// (publication.py:94, spec §4.13).
const DOIURLPrefix = "https://doi.org/"

// Error codes of spec §4.1 and §4.2. Both are pydantic's own discriminators.
const (
	CodeStringPatternMismatch schemaerr.Code = "string_pattern_mismatch"
	CodeURLTooLong            schemaerr.Code = "url_too_long"
)

const (
	// doiPatternSource is the pattern as written at publication.py:34. It appears
	// inside the message of spec §4.1 exactly as spelled here, backslashes
	// included, which is why it is a string constant and not a compiled regexp —
	// matchDOIPattern evaluates it (see doipattern.go).
	doiPatternSource = `\b10\..*`

	// maxURLLength is the length limit pydantic's URL type enforces, reached here
	// through the generated DOI URL (spec §3.12 behavior 23).
	maxURLLength = 2083

	// TODO(iteration-4): spec §7.3 - both messages are pydantic's text, not
	// RenderCV's, and are pinned so a later decision to diverge shows as a diff.
	messageDOIPatternMismatch = "String should match pattern '" + doiPatternSource + "'"
	messageURLTooLong         = "URL should have at most 2083 characters"
)

// errDOIPattern is the `doi` pattern failure, carried through the binder's Scalar
// hook so the error lands at `doi`'s declared position.
//
// message text, reproduced verbatim as part of the parity contract, not a Go
// error string we are free to style.
//
//nolint:staticcheck // ST1005: the capital is upstream's. This is pydantic's own
var errDOIPattern = errors.New(messageDOIPatternMismatch)

// HTTPURLValidator validates one field declared `pydantic.HttpUrl`. The node is
// the value as written; location is its schema location.
//
// TODO(iteration-4): spec §7.3 - the real validator is HTTP-URL parsing and
// normalization (`https://example.com` gains a trailing slash,
// `HTTPS://Example.COM/Path` lowercases scheme and host), plus the `url_parsing`
// error of spec §4.6. It is deferred so the decision is made once for
// `PublicationEntry.url`, `cv.website`, `cv.email` and `cv.phone` together.
// Iteration 3 registers a pass-through so the doi/url rules below can be tested
// on their own.
type HTTPURLValidator func(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError

// httpURLValidators maps a field declared as an HTTP URL to its validator,
// mirroring the shape iteration 2 used for `{website, email, phone}`
// (internal/schema/models/cv/scalarorlist.go's elementValidators). It is a
// registration table rather than a call site so iteration 4 replaces one
// implementation instead of four.
//
// This seam carries more weight than its size suggests: no golden case sets
// `PublicationEntry.url` at all (spec §5.24), so the conformance suite is
// permanently blind to whatever iteration 4 decides here.
var httpURLValidators = map[string]HTTPURLValidator{
	"url": passThroughHTTPURL,
}

func passThroughHTTPURL(
	*yamldoc.Node,
	[]string,
	schemaerr.YamlSource,
) []schemaerr.ValidationError {
	return nil
}

// PublicationEntry mirrors PublicationEntry (entries/publication.py:100-101) and
// its own-field base BasePublicationEntry (entries/publication.py:12-96).
//
// Its base is BaseEntryWithDate, not BaseEntryWithComplexFields, so it has
// `date` and nothing else: no `start_date`, no `end_date`, no `location`, no
// `highlights` (publication.py:100, spec §3.10 behavior 16). Those keys are
// still accepted, as unknown keys, and get no date validation (spec §5.6).
type PublicationEntry struct {
	bases.BaseEntryWithDate

	Title   *yamldoc.Node
	Authors *yamldoc.Node
	Summary *yamldoc.Node
	DOI     *yamldoc.Node
	URL     *yamldoc.Node
	Journal *yamldoc.Node
}

// publicationOwnFields is the own-field order of spec §3.10. It precedes the
// base's `date` because upstream declares
// `class PublicationEntry(BaseEntryWithDate, BasePublicationEntry)` and pydantic
// emits the last-listed base first (publication.py:100, spec §3.2), which is why
// `date` is last in the field order and not first.
//
// `url` is bound as ValueAny - a raw node, unparsed - per the seam above.
var publicationOwnFields = []binder.Field{
	{Name: "title", Required: true, Value: binder.ValueString},
	{Name: "authors", Required: true, Value: binder.ValueStringList},
	{Name: "summary", Value: binder.ValueString},
	{
		Name:  "doi",
		Value: binder.ValueString,
		// Upstream's `pattern=r"\b10\..*"` is an enforced field constraint
		// (publication.py:34), so it is reported at `doi`'s declared position --
		// before `journal` and `date`, not after every other field (spec 004
		// §3.9a behavior 33a row 4).
		Scalar: func(raw string, _ bool) error {
			if matchDOIPattern(raw) {
				return nil
			}
			return errDOIPattern
		},
		ScalarCode: CodeStringPatternMismatch,
	},
	{Name: "url", Value: binder.ValueAny},
	{Name: "journal", Value: binder.ValueString},
}

// PublicationDescriptor is PublicationEntry's registration. Its field set is its
// six own fields then `date`, the only field BaseEntryWithDate contributes
// (publication.py:100, spec §3.10 behavior 15). DOIURL is a method, not a field,
// so it does not appear here (spec §3.12 behavior 22).
func PublicationDescriptor() Descriptor {
	fields := make([]string, 0, len(publicationOwnFields)+1)
	for _, field := range publicationOwnFields {
		fields = append(fields, field.Name)
	}
	return Descriptor{Name: "PublicationEntry", Fields: append(fields, bases.DateFieldNames()...)}
}

// ValidatePublicationEntry binds and validates one publication entry
// (entries/publication.py:12-101), including the `doi` pattern of spec §3.11 and
// the three model-level rules of spec §3.12.
func ValidatePublicationEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	_ time.Time,
) (*PublicationEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntryWithDate(node, publicationOwnFields, location, source)

	entry := &PublicationEntry{BaseEntryWithDate: *base}
	for _, field := range []struct {
		name  string
		value **yamldoc.Node
	}{
		{name: "title", value: &entry.Title},
		{name: "authors", value: &entry.Authors},
		{name: "summary", value: &entry.Summary},
		{name: "doi", value: &entry.DOI},
		{name: "url", value: &entry.URL},
		{name: "journal", value: &entry.Journal},
	} {
		value, ok := base.Field(field.name)
		if ok && value != nil && value.Kind != yamldoc.KindNull {
			*field.value = value
		}
	}

	// The `url` field type itself, ahead of the model-level rules: upstream parses
	// it during field validation, so a bad URL is reported even when `doi` is
	// about to discard it.
	if entry.URL != nil {
		if validate, ok := httpURLValidators["url"]; ok {
			errs = append(errs, validate(entry.URL, publicationFieldLocation(location, "url"), source)...)
		}
	}

	// The two model-level rules of spec §3.12 are `mode="after"` validators
	// upstream, which pydantic runs only once every field has validated
	// (publication.py:46, :64). Gating them on an error-free bind keeps that
	// sequencing: a `doi` that failed its pattern never reaches the length check.
	if len(errs) != 0 {
		return entry, errs
	}

	// Rule 1 (publication.py:46-62): `doi` beats `url`, silently.
	if entry.DOI != nil {
		entry.URL = nil
	}

	// Rule 3 (publication.py:64-78): the generated DOI URL is validated as an HTTP
	// URL. Only its length is reachable - spaces, `#`, tabs, newlines and NUL in a
	// `doi` all pass (spec §5.2) - and the error carries an empty schema location,
	// naming no field. That is verified upstream and deliberate.
	if doiURL := entry.DOIURL(); len(doiURL) > maxURLLength {
		errs = append(errs, schemaerr.ValidationError{
			Code:           CodeURLTooLong,
			SchemaLocation: []string{},
			YamlLocation:   publicationSpan(entry.DOI),
			YamlSource:     source,
			Message:        messageURLTooLong,
			Input:          doiURL,
		})
	}

	return entry, errs
}

// DOIURL mirrors the `doi_url` property (publication.py:80-96): the prefix of
// spec §4.13 concatenated with the `doi` verbatim, or the empty string when
// `doi` is absent. Nothing is trimmed, escaped or normalized, so
// `doi = "10. spaced ?"` yields `https://doi.org/10. spaced ?`.
//
// Upstream caches it (`functools.cached_property`); this recomputes on purpose -
// it is a two-string concatenation, and a cache would need a mutable member that
// iteration 5's schema generation would have to learn to ignore (plan §6).
func (e *PublicationEntry) DOIURL() string {
	if e == nil || e.DOI == nil {
		return ""
	}
	return DOIURLPrefix + e.DOI.Raw
}

func publicationFieldLocation(location []string, key string) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, key)
}

func publicationSpan(node *yamldoc.Node) *yamldoc.Span {
	if node == nil {
		return nil
	}
	span := node.Span
	return &span
}
