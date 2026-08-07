package design

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
)

// fontFamilyEnumDef is the seventeen-name enum's `$defs` key. It is qualified
// with `font_family` while the five-key mapping model is qualified with
// `classic_theme` — **two different models with the same bare name**, which is
// why neither is numbered and a port that modelled one `FontFamily` would emit
// one entry and miss the other (plan §6 hazard 4).
const fontFamilyEnumDef = "rendercv__schema__models__design__font_family__FontFamily"

// SchemaDefs is the 161 `$defs` the design tree owns.
//
// Four groups: `BuiltInDesign`, the nine theme models, the five named `Literal`
// unions plus the font-name enum, and the nested option models with the
// collision suffixes variants.go assigns.
func SchemaDefs() map[string]*jsonschema.Object {
	defs := map[string]*jsonschema.Object{
		"BuiltInDesign":         UnionSchema(),
		"Bullet":                enumSchema(Bullets),
		"Alignment":             enumSchema(Alignments),
		"PageSize":              enumSchema(PageSizes),
		"SectionTitleType":      enumSchema(SectionTitleTypes),
		"PhoneNumberFormatType": enumSchema(PhoneNumberFormats),
		fontFamilyEnumDef:       enumSchema(FontFamilies),

		// **One entry, with the base's values, however many themes override
		// `typography.font_family`.** Seven of the eight do, and each gets a
		// variant `FontFamily` class at runtime — `EmberTheme`'s really does
		// carry `body='Ubuntu'`. None of them reaches the schema, because the
		// field's plain validator pins `json_schema_input_type=FontFamily |
		// FontFamilyType` (classic_theme.py:280-282), the **base** classes. So
		// the union is identical in all nine `Typography` entries and this model
		// is never numbered.
		qualify("FontFamily"): modelSchema(baseTree(), "FontFamily", BaseTheme),
	}

	tree := baseTree()
	for _, theme := range Themes() {
		defs[ThemeDefName(theme)] = themeSchema(tree, theme)
		for model, byTheme := range Ordinals() {
			if _, produced := byTheme[theme]; !produced {
				continue
			}
			defs[DefNameOf(model, theme)] = modelSchema(tree, model, theme)
		}
	}
	return defs
}

// UnionSchema is `BuiltInDesign` (built_in_design.py:41-43): `oneOf` in union
// order, `mapping` sorted by tag — the same two-order shape `locale.UnionSchema`
// has, and here `classic` happens to sort first as well as being the first arm,
// so only the locale case makes the difference visible.
func UnionSchema() *jsonschema.Object {
	arms := make([]any, 0, len(Themes()))
	for _, theme := range Themes() {
		arms = append(arms, jsonschema.Ref(ThemeDefName(theme)))
	}

	tags := append([]string(nil), Themes()...)
	sort.Strings(tags)
	mapping := jsonschema.NewObject()
	for _, tag := range tags {
		mapping.Set(tag, "#/$defs/"+ThemeDefName(tag))
	}

	return jsonschema.NewObject().
		Set("discriminator", jsonschema.NewObject().
			Set("mapping", mapping).
			Set("propertyName", "theme")).
		Set("oneOf", arms).
		Sort()
}

func enumSchema(members []string) *jsonschema.Object {
	values := make([]any, 0, len(members))
	for _, member := range members {
		values = append(values, member)
	}
	return jsonschema.NewObject().Set("enum", values).Set("type", "string")
}

// themeSchema is one theme model. Its title is the class name — `EmberTheme` —
// while every nested model keeps the base's, so `Colors__2` is still titled
// `Colors`.
func themeSchema(tree Tree, theme string) *jsonschema.Object {
	return objectSchema(ThemeDefName(theme), tree, tree.Root, theme, Overrides(theme))
}

func modelSchema(tree Tree, model, theme string) *jsonschema.Object {
	return objectSchema(model, tree, model, theme, overrideFor(tree, model, theme))
}

// overrideFor finds the override mapping that produced this (model, theme) pair,
// by walking the same path Ordinals walked. A model appears at exactly one place
// in the tree, so the first hit is the answer.
func overrideFor(tree Tree, model, theme string) map[string]any {
	var walk func(current string, override map[string]any) (map[string]any, bool)
	walk = func(current string, override map[string]any) (map[string]any, bool) {
		if current == model {
			return override, true
		}
		for _, field := range tree.Models[current].Fields {
			if field.Kind != KindNested {
				continue
			}
			nested, _ := override[field.Name].(map[string]any)
			if found, ok := walk(field.Nested, nested); ok {
				return found, true
			}
		}
		return nil, false
	}
	found, _ := walk(tree.Root, Overrides(theme))
	return found
}

func objectSchema(title string, tree Tree, model, theme string, override map[string]any) *jsonschema.Object {
	properties := jsonschema.NewObject()
	for _, field := range tree.Models[model].Fields {
		properties.Set(field.Name, fieldSchema(field, theme, override))
	}

	// No `required` key: every field has a default, so nothing is required, and
	// pydantic omits the key entirely rather than emitting an empty list
	// (spec 005 §5 behavior 16).
	return jsonschema.NewObject().
		Set("additionalProperties", false).
		Set("properties", properties).
		Set("title", title).
		Set("type", "object").
		Sort()
}

// fieldSchema renders one field against one theme's overrides.
//
// **An overridden field loses its `examples`.** `create_simple_field_spec`
// (variant_pydantic_model_generator.py:428-441) rebuilds the `FieldInfo` with
// default, description and title only, so `Colors.body` carries four examples in
// `Colors__1` and none in `Colors__2`. A field the theme does not name is
// inherited whole and keeps them. Measured; it is the difference between 161
// entries and 161 entries that look right.
func fieldSchema(field Field, theme string, override map[string]any) *jsonschema.Object {
	value, overridden := override[field.Name]
	if field.Kind == KindThemeTag {
		// The discriminator is pinned per variant rather than overridden, and it
		// carries no description at all.
		return jsonschema.NewObject().
			Set("const", theme).
			Set("default", theme).
			Set("title", jsonschema.TitleFor(field.Name)).
			Set("type", "string").
			Sort()
	}

	defaultValue := field.Default
	description := field.Description
	examples := field.Examples
	if overridden && field.Kind != KindNested {
		description = respliceDefault(description, field.Default, value)
		defaultValue = value
		examples = nil
	}

	object := jsonschema.NewObject()
	switch field.Kind {
	case KindNested:
		object.Set("$ref", "#/$defs/"+DefNameOf(field.Nested, theme))
	case KindTypstDimension, KindLiteral:
		if field.Ref != "" {
			object.Set("$ref", "#/$defs/"+field.Ref)
		} else {
			object.Set("enum", anySlice(field.Members)).Set("type", "string")
		}
	case KindColor:
		object.Set("format", "color").Set("type", "string")
	case KindBool:
		object.Set("type", "boolean")
	case KindString:
		object.Set("type", "string")
	case KindStringList:
		object.Set("items", jsonschema.NewObject().Set("type", "string")).Set("type", "array")
	case KindOptionalString:
		object.Set("anyOf", []any{
			jsonschema.NewObject().Set("type", "string"),
			jsonschema.NewObject().Set("type", "null"),
		})
	case KindFontFamily:
		object.Set("anyOf", []any{
			jsonschema.Ref(qualify("FontFamily")),
			jsonschema.Ref(fontFamilyEnumDef),
		})
	case KindThemeTag:
		// Returned above, before any override is considered.
	}

	// **A field with no declared default gets no `default` key**, even when a
	// theme overrides it. `typography.font_family` is the only one: upstream
	// builds it with a `default_factory`, and `create_nested_field_spec` keeps
	// the factory rather than turning the override into a literal default. So
	// `Typography__6` carries no `default` for it despite ember naming it.
	if field.Kind != KindNested && field.Default != nil {
		object.Set("default", schemaDefault(defaultValue))
	}
	if description != "" {
		object.Set("description", description)
	}
	if len(examples) > 0 {
		object.Set("examples", examples)
	}
	if title := fieldTitle(field); title != "" {
		object.Set("title", title)
	}
	return object.Sort()
}

// fieldTitle applies the rule spec 005 §3.2 derived: pydantic omits a field's
// title when the schema is exactly one `$ref`. A `$ref` in an `anyOf` keeps its
// title, which is why `font_family` has one and `page` does not.
func fieldTitle(field Field) string {
	switch field.Kind {
	case KindNested:
		return ""
	case KindTypstDimension, KindLiteral:
		if field.Ref != "" {
			return ""
		}
	case KindString, KindOptionalString, KindStringList, KindBool, KindColor,
		KindFontFamily, KindThemeTag:
		// Every other kind keeps its title: the omission rule is about the
		// schema being exactly one `$ref`, not about the field.
	}
	if field.Title != "" {
		return field.Title
	}
	return jsonschema.TitleFor(field.Name)
}

func schemaDefault(value any) any {
	if list, ok := value.([]string); ok {
		return anySlice(list)
	}
	return value
}

func anySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

// respliceDefault is `update_description_with_new_default`
// (variant_pydantic_model_generator.py:196-218): the old default in backticks
// replaced by the new one, both through Python's `str`.
//
// **The bool case is a no-op and that is upstream's behavior, not a bug here.**
// `str(False)` is `False` and the descriptions are written with a lowercase
// `false`, so nothing matches — `SmallCaps__2.headline` defaults to `true` while
// its description still says `false`. A port that lowercased to make the
// sentence true would produce eleven wrong strings that read correctly.
func respliceDefault(description string, oldDefault, newDefault any) string {
	return strings.ReplaceAll(description,
		"`"+pythonRepr(oldDefault)+"`", "`"+pythonRepr(newDefault)+"`")
}

// pythonRepr is `str(x)` for the three types a design default can be.
func pythonRepr(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case []string:
		quoted := make([]string, 0, len(typed))
		for _, item := range typed {
			quoted = append(quoted, "'"+item+"'")
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	case nil:
		return "None"
	}
	return fmt.Sprint(value)
}
