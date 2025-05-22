package cv

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// SocialNetworkNameSchema is the seventeen-name literal union
// (social_network.py:13-31).
//
// The members come from SocialNetworkNames rather than being written out, so the
// schema and the validator cannot disagree — and the order is the literal type's
// declaration order, which spec 004 §4.23 already pinned in the error message.
// A reordering therefore fails two tests rather than none.
func SocialNetworkNameSchema() *jsonschema.Object {
	members := make([]any, 0, len(SocialNetworkNames))
	for _, name := range SocialNetworkNames {
		members = append(members, string(name))
	}
	return jsonschema.NewObject().
		Set("enum", members).
		Set("type", "string")
}

// TypstDimensionSchema is `TypstDimension` (design/typst_dimension.py).
//
// It is a design type and it is here because nothing in `design` exists yet and
// `Cv` does not reach it either — it is shared, already modelled as a string,
// and belongs to whichever package first needs it. Iteration 6 may move it.
func TypstDimensionSchema() *jsonschema.Object {
	return jsonschema.NewObject().Set("type", "string")
}

// ExistingPathRelativeToInputSchema is the required-path type (path.py:67-72).
//
// `format: path` is pydantic's annotation for a `pathlib.Path`, and it carries
// no validation weight in JSON Schema — it is metadata an IDE may use.
func ExistingPathRelativeToInputSchema() *jsonschema.Object {
	return jsonschema.NewObject().
		Set("format", "path").
		Set("type", "string")
}

// httpURLArm is `pydantic.HttpUrl`'s schema, which three fields reach:
// `CustomConnection.url`, `PublicationEntry.url` and the URL branch of
// `cv.photo` (spec 004 §3.13 behavior 41).
//
// `maxLength` is the 2083 of spec 004 §3.13 behavior 46 — the limit whose unit
// is bytes rather than characters, though JSON Schema cannot say which.
func httpURLArm() *jsonschema.Object {
	return jsonschema.NewObject().
		Set("format", "uri").
		Set("maxLength", 2083).
		Set("minLength", 1).
		Set("type", "string")
}

// SocialNetworkSchema is `SocialNetwork` (social_network.py:53-57).
//
// `network` is a bare `$ref` rather than an `anyOf`: it is required, so there is
// no null arm, and a lone reference is emitted directly.
func SocialNetworkSchema() *jsonschema.Object {
	return jsonschema.Model("SocialNetwork", false, []jsonschema.Property{
		{Name: "network", Ref: "SocialNetworkName"},
		{Name: "username", Type: "string", Examples: []any{
			"john_doe", "@johndoe@mastodon.social", "12345/john-doe",
		}},
	})
}

// CustomConnectionSchema is `CustomConnection` (custom_connection.py:6-9).
//
// `url` is required **and** nullable, which is why it is in `required` and still
// carries the null arm — the distinction spec 002 §3.81 drew between "the key
// must be present" and "the value may be null".
func CustomConnectionSchema() *jsonschema.Object {
	return jsonschema.Model("CustomConnection", false, []jsonschema.Property{
		{Name: "fontawesome_icon", Type: "string"},
		{Name: "placeholder", Type: "string"},
		{Name: "url", Arms: []any{httpURLArm()}, Optional: true, Required: true},
	})
}

// SectionSchema is `Section`, a bare alias for `ListOfEntries`.
func SectionSchema() *jsonschema.Object {
	return jsonschema.Ref("ListOfEntries")
}

// SchemaDefs returns every `$defs` entry the `cv` models own.
func SchemaDefs() map[string]*jsonschema.Object {
	return map[string]*jsonschema.Object{
		"SocialNetworkName":           SocialNetworkNameSchema(),
		"SocialNetwork":               SocialNetworkSchema(),
		"CustomConnection":            CustomConnectionSchema(),
		"Section":                     SectionSchema(),
		"Cv":                          Schema(),
		"TypstDimension":              TypstDimensionSchema(),
		"ExistingPathRelativeToInput": ExistingPathRelativeToInputSchema(),
	}
}

// Schema is `Cv` (cv.py:31-...).
//
// Ten properties in **declaration order**, which is spec 002 §3.44's order and
// the same list FieldNames carries — so the schema and the validator cannot
// disagree about it.
//
// Three of them, `email`, `phone` and `website`, carry **no type at all**: they
// are the scalar-or-list fields, whose union pydantic cannot express, so their
// schema is metadata only (spec 002 §3.47).
func Schema() *jsonschema.Object {
	return jsonschema.Model("Cv", false, []jsonschema.Property{
		{Name: "name", Type: "string", Optional: true, Examples: []any{"John Doe", "Jane Smith"}},
		{Name: "headline", Type: "string", Optional: true, Examples: []any{
			"Software Engineer", "Data Scientist", "Product Manager",
		}},
		{Name: "location", Type: "string", Optional: true, Examples: []any{
			"New York, NY", "London, UK", "Istanbul, Türkiye",
		}},
		{
			Name: "email", Metadata: true, Optional: true,
			Description: "You can provide multiple emails as a list.",
			Examples: []any{
				"john.doe@example.com",
				[]any{"john.doe.1@example.com", "john.doe.2@example.com"},
			},
		},
		{
			Name: "photo", Optional: true,
			Arms: []any{
				jsonschema.Ref("ExistingPathRelativeToInput"),
				httpURLArm(),
			},
			Description: "Photo file path (relative to the YAML file) or a URL.",
			Examples: []any{
				"photo.jpg", "images/profile.png", "https://example.com/photo.jpg",
			},
		},
		{
			Name: "phone", Metadata: true, Optional: true,
			Description: "Your phone number with country code in international format" +
				" (e.g., +1 for USA, +44 for UK). The display format in the output is" +
				" controlled by `design.header.connections.phone_number_format`." +
				" You can provide multiple numbers as a list.",
			Examples: []any{
				"+1-234-567-8900",
				[]any{"+1-234-567-8900", "+44 20 1234 5678"},
			},
		},
		{
			Name: "website", Metadata: true, Optional: true,
			Description: "You can provide multiple URLs as a list.",
			Examples: []any{
				"https://johndoe.com",
				[]any{"https://johndoe.com", "https://www.janesmith.dev"},
			},
		},
		{
			Name: "social_networks", Optional: true,
			Arms: []any{jsonschema.NewObject().
				Set("items", jsonschema.Ref("SocialNetwork")).
				Set("type", "array")},
		},
		{
			Name: "custom_connections", Optional: true,
			Arms: []any{jsonschema.NewObject().
				Set("items", jsonschema.Ref("CustomConnection")).
				Set("type", "array")},
			Description: "Additional header connections you define yourself. Each item" +
				" has a `placeholder` (the displayed text), an optional `url`, and the" +
				" Font Awesome icon name to render (from https://fontawesome.com/search).",
			Examples: []any{[]any{jsonschema.NewObject().
				Set("fontawesome_icon", "calendar-days").
				Set("placeholder", "Book a call").
				Set("url", "https://cal.com/johndoe")}},
		},
		{
			Name: "sections", Optional: true,
			Arms: []any{jsonschema.NewObject().
				Set("additionalProperties", jsonschema.Ref("Section")).
				Set("type", "object")},
			Description: "The sections of your CV. Keys are section titles (e.g.," +
				" Experience, Education), and values are lists of entries. Entry types" +
				" are automatically detected based on their fields.",
			Examples: []any{jsonschema.NewObject().
				Set("Education", "...").
				Set("Experience", "...").
				Set("Projects", "...").
				Set("Skills", "...")},
		},
	})
}
