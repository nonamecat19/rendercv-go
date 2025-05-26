package models

import (
	"sort"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

// The four keys the generator adds after pydantic has run
// (json_schema_generator.py:23-32, spec 005 §4).
const (
	schemaTitle       = "RenderCV"
	schemaDescription = "Resume builder for academics and engineers"
	schemaID          = "https://raw.githubusercontent.com/rendercv/rendercv/main/schema.json"
	schemaDialect     = "http://json-schema.org/draft-07/schema#"
)

// Schema builds the whole JSON Schema, mirroring generate_json_schema.
//
// The key order at the top level is the one thing here that is not sorted, and
// it is not an accident (spec 005 §3.2 behavior 8): pydantic emits a sorted
// object, then the generator **overwrites** `title` and **inserts** the other
// three. `title` therefore stays inside the sorted run and the rest land after
// it, which is exactly what jsonschema.Object.Set reproduces.
//
// **The `$defs` are incomplete and that is this iteration's whole scope**
// (spec 005 §1). 209 of upstream's 227 come from the design, locale and settings
// models, which iterations 6 and 7 own; `just schema-diff` stays red until then,
// and `specs/STATE.md` records Axis 3 as blocked rather than failing.
func Schema() *jsonschema.Object {
	object := jsonschema.NewObject().
		Set("$defs", schemaDefs()).
		Set("additionalProperties", false).
		Set("properties", topLevelProperties()).
		// Empty rather than absent: every field of RenderCVModel has a default,
		// and pydantic emits the empty list (spec 005 §5 behavior 16). `Cv` by
		// contrast omits the key entirely, so the two are not the same rule.
		Set("required", []any{}).
		Set("title", "RenderCVModel").
		Set("type", "object").
		Sort()

	object.Set("title", schemaTitle)
	object.Set("description", schemaDescription)
	object.Set("$id", schemaID)
	object.Set("$schema", schemaDialect)
	return object
}

// topLevelProperties is the four blocks, in declaration order.
//
// Each is a `$ref` carrying its own `description` and `title` — so unlike the
// entry fields of spec 005 §3.2, a reference here does keep a title, because it
// is a bare `$ref` rather than an `anyOf` of one.
func topLevelProperties() *jsonschema.Object {
	return jsonschema.NewObject().
		Set("cv", refWith("Cv", "The content of the CV.", "CV")).
		Set("design", refWith("BuiltInDesign",
			"The design information of the CV. The default is the `classic` theme.",
			"Design")).
		Set("locale", refWith("Locale",
			"The locale catalog of the CV to allow the support of multiple languages.",
			"Locale Catalog")).
		Set("settings", refWith("Settings", "The settings of the RenderCV.",
			"RenderCV Settings"))
}

func refWith(def, description, title string) *jsonschema.Object {
	return jsonschema.Ref(def).
		Set("description", description).
		Set("title", title).
		Sort()
}

// schemaDefs assembles every `$defs` entry the port can build, sorted by key
// (spec 005 §6 rule 4).
func schemaDefs() *jsonschema.Object {
	all := map[string]*jsonschema.Object{}
	for name, object := range entries.SchemaDefs() {
		all[name] = object
	}
	for name, object := range cv.SchemaDefs() {
		all[name] = object
	}
	for name, object := range locale.SchemaDefs() {
		all[name] = object
	}

	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := jsonschema.NewObject()
	for _, name := range names {
		defs.Set(name, all[name])
	}
	return defs
}

// SchemaJSON is the serialized schema, which is what `rendercv-go schema`
// writes. No trailing newline (spec 005 §3.4 behavior 15).
func SchemaJSON() (string, error) {
	return jsonschema.Marshal(Schema())
}
