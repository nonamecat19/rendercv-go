package cv

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
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
) (*Cv, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fieldOrder, Policy: binder.ForbidExtra},
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

	return model, errs
}
