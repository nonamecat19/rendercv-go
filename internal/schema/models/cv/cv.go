package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/inputpath"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// fieldOrder is the declaration order of spec §3.44
// (schema/models/cv/cv.py:32-118). All ten fields are optional and default to
// absent, and unknown keys are rejected (spec §3.45, cv.py:31).
var fieldOrder = []binder.Field{
	// **The three plain-text fields are typed `str | None`** (cv.py:32, :36,
	// :40), so a non-string is `Input should be a valid string` — pydantic's
	// lax mode does not coerce an `int`, a `float` or a `bool` to a `str`.
	// They were declared with no shape at all, so `cv.name: 200` rendered a CV
	// named `200` at exit 0 where upstream exits 1, and any value the reader
	// resolved to something other than a string reached the artifact.
	{Name: "name", Value: binder.ValueString},
	{Name: "headline", Value: binder.ValueString},
	{Name: "location", Value: binder.ValueString},
	{Name: "email"},
	{Name: "photo"},
	{Name: "phone"},
	{Name: "website"},
	{Name: "social_networks"},
	{Name: "custom_connections"},
	{Name: "sections"},
}

// Cv mirrors the `cv` model (schema/models/cv/cv.py:30-250).
//
// Every field holds the raw document node it was bound from. `photo`, `email`,
// `phone`, `website` and `sections` gain their typed shapes in later tasks of
// this iteration; the field set and the recorded key order are what this one
// fixes.
type Cv struct {
	Name              *yamldoc.Node
	Headline          *yamldoc.Node
	Location          *yamldoc.Node
	Email             *yamldoc.Node
	Photo             *yamldoc.Node
	Phone             *yamldoc.Node
	Website           *yamldoc.Node
	SocialNetworks    *yamldoc.Node
	CustomConnections *yamldoc.Node
	Sections          *yamldoc.Node

	// PhotoValue is the resolved `photo` union (spec §3.46), set when a photo
	// was supplied.
	PhotoValue *Photo

	// keyOrder mirrors `_key_order` (cv.py:166, :173): the input mapping's key
	// order with null-valued keys dropped, empty for a non-mapping input. It
	// drives the order header connections are rendered in (cv.py:124-126).
	keyOrder []string
}

// FieldNames returns the ten field names in declaration order (spec §3.44).
func FieldNames() []string {
	names := make([]string, 0, len(fieldOrder))
	for _, field := range fieldOrder {
		names = append(names, field.Name)
	}
	return names
}

// KeyOrder reports the recorded key order (spec §3.50). The returned slice is a
// copy, so a caller cannot disturb the record — spec §3.51 requires an already
// validated object to keep its order untouched.
func (c *Cv) KeyOrder() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.keyOrder...)
}

// Validate binds a `cv` mapping. It returns the model together with every error
// it accumulated, in the order the binder produced them (spec §6.6).
func Validate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	opts Options,
) (*Cv, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fieldOrder, Policy: binder.ForbidExtra, Model: "Cv"},
		location,
		source,
	)

	model := &Cv{keyOrder: result.KeyOrder}
	model.Name, _ = result.Value("name")
	model.Headline, _ = result.Value("headline")
	model.Location, _ = result.Value("location")
	model.Email, _ = result.Value("email")
	model.Photo, _ = result.Value("photo")
	model.Phone, _ = result.Value("phone")
	model.Website, _ = result.Value("website")
	model.SocialNetworks, _ = result.Value("social_networks")
	model.CustomConnections, _ = result.Value("custom_connections")
	model.Sections, _ = result.Value("sections")

	errs = append(errs, model.validateFields(location, source, opts)...)

	// Unknown keys last: after every declared field of this model has reported,
	// including the ones validateFields emits (spec 004 §3.9 behavior 32 step 3).
	return model, append(errs, result.ExtraErrors...)
}

// Options carries what the field validators need beyond the document: the
// entry-type registry section discrimination runs against, and the validation
// context path resolution uses.
type Options struct {
	Registry *entries.Registry
	Context  *valctx.ValidationContext
}

// validateFields runs the per-field validators the model owns. `design`,
// `locale` and `settings` have no models yet (iterations 6 and 7). The top-level
// model reaches this through models.Validate as of iteration 3 T20: the import
// cycle that used to block it is gone, because the context and path types moved
// to models/valctx and models/inputpath (T1, T2).
//
// The reference date comes from the context rather than the clock, so a pinned
// `settings.current_date` renders reproducibly. A nil context yields today,
// which is what upstream's get_current_date does (validation_context.py:36-58).
func (c *Cv) validateFields(
	location []string,
	source schemaerr.YamlSource,
	opts Options,
) []schemaerr.ValidationError {
	var errs []schemaerr.ValidationError

	// **In declaration order, one field at a time.** Upstream is pydantic, which
	// validates fields in the order they are declared, so a document with a bad
	// `email` and a bad `website` reports `email` first — `email` is declared
	// fourth and `website` seventh (spec 004 §3.9 behavior 32).
	//
	// Iteration 2 ran the validators in groups: every scalar-or-list field, then
	// the photo, then the sequences. That produces the right records in the wrong
	// order, and dedup keeps the first record at a location, so a wrong order is
	// a wrong message somewhere downstream.
	for _, field := range FieldNames() {
		errs = append(errs, c.validateField(field, location, source, opts)...)
	}

	return errs
}

// validateField runs the one validator a field has, if it has one. The switch is
// exhaustive over FieldNames so a new field cannot be added without deciding
// whether it needs one.
func (c *Cv) validateField(
	field string,
	location []string,
	source schemaerr.YamlSource,
	opts Options,
) []schemaerr.ValidationError {
	switch field {
	case "website", "email", "phone":
		// Spec §3.47: the shared scalar-or-list rule.
		node, _ := c.fieldNode(field)
		errs, err := ValidateScalarOrList(field, node, fieldLocation(location, field), source)
		if err != nil {
			// The internal error of spec §4.7 cannot arise here: the field name
			// always comes from FieldNames.
			return nil
		}
		return errs

	case "photo":
		// Spec §3.46: the photo union, path interpretation first. Only the path
		// arm reports — see ResolvePhoto for why the URL record must not exist.
		if c.Photo == nil || c.Photo.Kind == yamldoc.KindNull {
			return nil
		}
		// Both arms of the union take a `str`, so a non-string fails on type
		// before either is tried. Reporting a *resolution* message for it —
		// "The file `5` does not exist." — named a file the user never wrote,
		// and for a mapping or a sequence, whose Raw is empty, it resolved the
		// empty path and left the renderer to fail on `open :` at the end.
		if c.Photo.Kind != yamldoc.KindString {
			return []schemaerr.ValidationError{{
				Code:           inputpath.CodePathType,
				SchemaLocation: fieldLocation(location, "photo"),
				YamlLocation:   &c.Photo.Span,
				YamlSource:     source,
				Message:        inputpath.MessagePathType,
				Input:          schemaerr.RenderInput(c.Photo),
			}}
		}
		photo, failure := ResolvePhoto(c.Photo.Raw, opts.Context)
		c.PhotoValue = photo
		if failure == nil {
			return nil
		}
		located := *failure
		located.SchemaLocation = fieldLocation(location, "photo")
		located.YamlSource = source
		located.YamlLocation = &c.Photo.Span
		// The Input Value column is the value the user wrote, not the path the
		// message interpolates. The two agree for an ordinary relative path,
		// which is why the resolver's own rendering passed for so long, and
		// disagree for an absolute path and for `photo: ""` — whose column is
		// empty while the message names `.`.
		located.Input = schemaerr.RenderInput(c.Photo)
		return []schemaerr.ValidationError{located}

	case "social_networks":
		if errs := listShape(c.SocialNetworks, location, "social_networks", source); errs != nil {
			return errs
		}
		if c.SocialNetworks == nil || c.SocialNetworks.Kind != yamldoc.KindSequence {
			return nil
		}
		base := fieldLocation(location, "social_networks")
		var errs []schemaerr.ValidationError
		for i, elem := range c.SocialNetworks.Elems {
			_, elemErrs := ValidateSocialNetwork(elem, indexLocation(base, i), source)
			errs = append(errs, elemErrs...)
		}
		return errs

	case "custom_connections":
		if errs := listShape(c.CustomConnections, location, "custom_connections", source); errs != nil {
			return errs
		}
		if c.CustomConnections == nil || c.CustomConnections.Kind != yamldoc.KindSequence {
			return nil
		}
		base := fieldLocation(location, "custom_connections")
		var errs []schemaerr.ValidationError
		for i, elem := range c.CustomConnections.Elems {
			_, elemErrs := ValidateCustomConnection(elem, indexLocation(base, i), source)
			errs = append(errs, elemErrs...)
		}
		return errs

	case "sections":
		// Spec §3.53-§3.61: every section, in input order.
		if c.Sections != nil && c.Sections.Kind != yamldoc.KindNull &&
			c.Sections.Kind != yamldoc.KindMapping {
			// `sections` is `dict[str, list[...]]`, so a non-mapping is
			// pydantic's `dict_type` and nothing under it is looked at.
			// Skipping it silently rendered a CV from `sections: abc`.
			return []schemaerr.ValidationError{{
				Code:           binder.CodeDictType,
				SchemaLocation: fieldLocation(location, "sections"),
				YamlLocation:   &c.Sections.Span,
				YamlSource:     source,
				Message:        binder.MessageDictType,
				Input:          schemaerr.RenderInput(c.Sections),
			}}
		}
		if c.Sections == nil || c.Sections.Kind != yamldoc.KindMapping || opts.Registry == nil {
			return nil
		}
		base := fieldLocation(location, "sections")
		var errs []schemaerr.ValidationError
		for _, item := range c.Sections.Items {
			_, sectionErrs := ValidateSection(
				item.Value, opts.Registry, fieldLocation(base, item.Key), source,
				opts.Context.Today(),
			)
			errs = append(errs, sectionErrs...)
		}
		return errs

	case "name", "headline", "location":
		// Plain optional text; the binder's shape check is the whole rule.
		return nil
	}
	return nil
}

func (c *Cv) fieldNode(name string) (*yamldoc.Node, bool) {
	switch name {
	case "email":
		return c.Email, c.Email != nil
	case "phone":
		return c.Phone, c.Phone != nil
	case "website":
		return c.Website, c.Website != nil
	default:
		return nil, false
	}
}

// listShape is the sequence check the two list-valued `cv` fields share. Both
// are `list[...]` with a default of None, so a null passes and every other
// non-sequence is pydantic's `list_type` — which the port skipped silently,
// rendering a CV from `social_networks: 5` where upstream exits 1.
//
// It returns nil when the value is a sequence, a null or absent, which is what
// the caller's `if` reads.
func listShape(
	node *yamldoc.Node,
	location []string,
	field string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull || node.Kind == yamldoc.KindSequence {
		return nil
	}
	return []schemaerr.ValidationError{{
		Code:           binder.CodeListType,
		SchemaLocation: fieldLocation(location, field),
		YamlLocation:   &node.Span,
		YamlSource:     source,
		Message:        binder.MessageListType,
		Input:          schemaerr.RenderInput(node),
	}}
}
