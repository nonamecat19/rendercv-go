package cv

import (
	"errors"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/inputpath"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// customConnectionFields is the declaration order of spec §3.81
// (schema/models/cv/custom_connection.py:6-9): `fontawesome_icon` and
// `placeholder` are required text with no default, and `url` is
// required-but-nullable — its declared type admits null, but it has no
// default either, so the key must still be present (binder.Field's doc
// comment on Required). Extra keys are rejected.
var customConnectionFields = []binder.Field{
	{Name: "fontawesome_icon", Required: true},
	{Name: "placeholder", Required: true},
	{
		Name:     "url",
		Required: true,
		// Declared `pydantic.HttpUrl` (custom_connection.py:9), one of the four
		// sites of spec 004 §3.13 behavior 41. Required-but-nullable: an explicit
		// null is the declared default and validates nothing, so the shape stays
		// ValueAny and the check runs from the scalar hook.
		Scalar: func(raw string, _ bool) error {
			_, err := httpurl.Validate(raw)
			return err
		},
	},
}

// CustomConnectionFieldNames returns the three field names in declaration
// order (spec §3.81).
func CustomConnectionFieldNames() []string {
	names := make([]string, 0, len(customConnectionFields))
	for _, field := range customConnectionFields {
		names = append(names, field.Name)
	}
	return names
}

// CustomConnection mirrors the CustomConnection model
// (schema/models/cv/custom_connection.py:6-9, spec §3.81). All three fields
// hold the raw document node they were bound from. A present-but-null `Url`
// is a node of kind KindNull, which is how required-but-nullable stays
// distinguishable from absent here.
type CustomConnection struct {
	FontawesomeIcon *yamldoc.Node
	Placeholder     *yamldoc.Node
	Url             *yamldoc.Node
}

// ValidateCustomConnection binds a custom-connection mapping (spec §3.81). It
// returns the model together with every error the binder accumulated.
func ValidateCustomConnection(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) (*CustomConnection, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: customConnectionFields, Policy: binder.ForbidExtra},
		location,
		source,
	)

	model := &CustomConnection{}
	model.FontawesomeIcon, _ = result.Value("fontawesome_icon")
	model.Placeholder, _ = result.Value("placeholder")
	model.Url, _ = result.Value("url")

	return model, errs
}

// PhotoKind distinguishes which branch of the `cv.photo` union
// (schema/models/cv/cv.py:52-57, spec §3.46) a value resolved to.
type PhotoKind int

const (
	// PhotoKindPath is the file-path interpretation — T12's
	// ExistingPathRelativeToInput (path.py:67-72).
	PhotoKindPath PhotoKind = iota
	// PhotoKindURL is the HTTP-URL interpretation.
	PhotoKindURL
)

// Photo is the resolved value of `cv.photo` (schema/models/cv/cv.py:52-57,
// spec §3.46): a path relative to the input file, existence-required, or an
// HTTP URL.
type Photo struct {
	Kind PhotoKind
	Path inputpath.ExistingPathRelativeToInput
	URL  string
}

// ResolvePhoto mirrors `cv.photo`'s union resolution
// (schema/models/cv/cv.py:52-57, spec §3.46): the file-path interpretation —
// T12's existence-required path type — is attempted first, left to right,
// and only when it fails is raw treated as a URL. This is the only field in
// this iteration with a non-default union resolution order (spec §3.46).
//
// **When both arms fail, only the path failure is reported, and that is
// deliberate.** Upstream evaluates both and emits two records at
// `("cv", "photo")` — the path failure, then a URL parse failure — and its
// deduplication keeps the first. The port emits the survivor directly
// (spec 004 plan §2.2 consequence 2).
//
// This is the one site of the three where the surviving *message* differs
// between the branches, so getting it wrong is visible:
// `photo: photo_doesnt_exist.jpg` must report spec 004 §4.25's
// "The file `…` does not exist." and never §4.9's URL text
// (`expected_errors.yaml:14-18`).
//
// A URL that parses is a success, not a failure with a nicer message: the union
// only fails when neither arm accepts the value.
func ResolvePhoto(raw string, ctx *valctx.ValidationContext) (*Photo, *schemaerr.ValidationError) {
	path, pathErr := inputpath.ResolveExistingPath(raw, ctx)
	if pathErr == nil {
		return &Photo{Kind: PhotoKindPath, Path: path}, nil
	}

	if _, urlErr := httpurl.Validate(raw); urlErr == nil {
		return &Photo{Kind: PhotoKindURL, URL: raw}, nil
	}

	// Both arms failed. Report the path one; the URL record is the one dedup
	// would have thrown away.
	var failure *schemaerr.ValidationError
	if errors.As(pathErr, &failure) {
		reported := *failure
		return nil, &reported
	}

	// The path arm failed for a non-validation reason — no resolution base, say —
	// which is not something to report against the field.
	return &Photo{Kind: PhotoKindURL, URL: raw}, nil
}
