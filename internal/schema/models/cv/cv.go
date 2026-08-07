package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// fieldOrder is the declaration order of spec §3.44
// (schema/models/cv/cv.py:32-118). All ten fields are optional and default to
// absent, and unknown keys are rejected (spec §3.45, cv.py:31).
var fieldOrder = []binder.Field{
	{Name: "name"},
	{Name: "headline"},
	{Name: "location"},
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
	return model, errs
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

	// Spec §3.47: the shared scalar-or-list rule, in upstream's field order.
	for _, field := range ScalarOrListFields() {
		node, _ := c.fieldNode(field)
		fieldErrs, err := ValidateScalarOrList(field, node, fieldLocation(location, field), source)
		if err != nil {
			// The internal error of spec §4.7 cannot arise here: the field name
			// always comes from ScalarOrListFields.
			continue
		}
		errs = append(errs, fieldErrs...)
	}

	// Spec §3.46: the photo union, path interpretation first. Only the path
	// arm reports — see ResolvePhoto for why the URL record must not exist.
	if c.Photo != nil && c.Photo.Kind != yamldoc.KindNull {
		photo, failure := ResolvePhoto(c.Photo.Raw, opts.Context)
		c.PhotoValue = photo
		if failure != nil {
			located := *failure
			located.SchemaLocation = fieldLocation(location, "photo")
			located.YamlSource = source
			located.YamlLocation = &c.Photo.Span
			errs = append(errs, located)
		}
	}

	if c.SocialNetworks != nil && c.SocialNetworks.Kind == yamldoc.KindSequence {
		base := fieldLocation(location, "social_networks")
		for i, elem := range c.SocialNetworks.Elems {
			_, elemErrs := ValidateSocialNetwork(elem, indexLocation(base, i), source)
			errs = append(errs, elemErrs...)
		}
	}

	if c.CustomConnections != nil && c.CustomConnections.Kind == yamldoc.KindSequence {
		base := fieldLocation(location, "custom_connections")
		for i, elem := range c.CustomConnections.Elems {
			_, elemErrs := ValidateCustomConnection(elem, indexLocation(base, i), source)
			errs = append(errs, elemErrs...)
		}
	}

	// Spec §3.53–§3.61: every section, in input order.
	if c.Sections != nil && c.Sections.Kind == yamldoc.KindMapping && opts.Registry != nil {
		base := fieldLocation(location, "sections")
		for _, item := range c.Sections.Items {
			_, sectionErrs := ValidateSection(
				item.Value, opts.Registry, fieldLocation(base, item.Key), source,
				opts.Context.Today(),
			)
			errs = append(errs, sectionErrs...)
		}
	}

	return errs
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
