package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
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
	{Name: "url", Required: true},
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
//
// TODO(iteration-4): `pydantic.HttpUrl` validation of Url is out of scope for
// this shell (spec §7).
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
// TODO(iteration-4): like the pass-through element validators of
// scalarorlist.go, the URL branch does not yet apply `pydantic.HttpUrl`
// validation (spec §7); any string that fails the path interpretation is
// accepted as a URL. Wiring this into `Cv.Validate` is deferred the same way
// scalar-or-list routing was in T18 — the resolver exists and is tested on
// its own; the spine connects it to the model when the renderer needs it.
func ResolvePhoto(raw string, ctx *valctx.ValidationContext) *Photo {
	if path, err := inputpath.ResolveExistingPath(raw, ctx); err == nil {
		return &Photo{Kind: PhotoKindPath, Path: path}
	}
	return &Photo{Kind: PhotoKindURL, URL: raw}
}
