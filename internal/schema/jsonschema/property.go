package jsonschema

import "strings"

// Property describes one model field's schema, in the form the port states it.
//
// It exists so the eighteen `$defs` are written as data rather than as
// assembled objects: the key ordering, the `anyOf`-with-null shape for an
// optional field, and the title rule of TitleFor are the same for every field,
// and repeating them eighteen times is eighteen chances to differ.
type Property struct {
	// Name is the YAML key, which is also what Title is derived from.
	Name string

	// Type is the JSON type of a plain scalar field: "string", "integer".
	// Empty when Ref or Arms carries the shape instead.
	Type string

	// Ref names a `$defs` entry this field is exactly one reference to.
	Ref string

	// Arms is the non-null members of an `anyOf`, for a field that is neither a
	// plain scalar nor a single reference. `end_date` is the only entry field
	// that needs it.
	Arms []any

	// Optional adds the `null` member and `"default": null`.
	Optional bool

	// Description and Examples are pydantic's `Field(...)` metadata, omitted
	// when empty.
	Description string
	Examples    []any
}

// Schema renders the property.
//
// Keys are sorted, which is pydantic's rule for everything but `properties`
// itself (spec 005 §6 rule 2).
func (p Property) Schema() *Object {
	object := NewObject()

	switch {
	case len(p.Arms) > 0:
		object.Set("anyOf", p.anyOf(p.Arms))
	case p.Ref != "" && p.Optional:
		object.Set("anyOf", p.anyOf([]any{ref(p.Ref)}))
	case p.Ref != "":
		object.Set("$ref", "#/$defs/"+p.Ref)
	case p.Optional:
		object.Set("anyOf", p.anyOf([]any{NewObject().Set("type", p.Type)}))
	default:
		object.Set("type", p.Type)
	}

	if p.Optional {
		object.Set("default", nil)
	}
	if p.Description != "" {
		object.Set("description", p.Description)
	}
	if len(p.Examples) > 0 {
		object.Set("examples", p.Examples)
	}
	if title := p.title(); title != "" {
		object.Set("title", title)
	}
	return object.Sort()
}

// anyOf appends the null member for an optional field.
func (p Property) anyOf(arms []any) []any {
	members := append([]any(nil), arms...)
	if p.Optional {
		members = append(members, NewObject().Set("type", "null"))
	}
	return members
}

// title is pydantic's auto-generated field title, and the rule for **omitting**
// it, which is the irregularity in upstream's output.
//
// Measured: `date` and `start_date` carry no title while `end_date`, `location`,
// `summary` and `highlights` do — and none of the four is given one in the
// Python source. The distinguishing property is the schema's shape rather than
// the field: pydantic omits the title when the schema is exactly **one `$ref`
// plus `null`**, because a title there would shadow the referenced model's.
// `end_date` has a third member (`const: present`) and so keeps its own.
//
// Deriving it rather than listing the four fields matters because iterations 6
// and 7 add many more optional-reference fields, and a list would silently miss
// them.
func (p Property) title() string {
	if p.Ref != "" && len(p.Arms) == 0 {
		return ""
	}
	return TitleFor(p.Name)
}

// TitleFor is pydantic's `field_title_generator` default: underscores become
// spaces and each word is capitalized. `reversed_number` → `Reversed Number`.
func TitleFor(field string) string {
	words := strings.Split(field, "_")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func ref(name string) *Object {
	return NewObject().Set("$ref", "#/$defs/"+name)
}

// Ref is a bare reference object, for an `anyOf` arm or a whole `$defs` entry.
func Ref(name string) *Object { return ref(name) }

// Model renders a model's `$defs` entry: the uniform envelope every entry type
// and every `cv` model shares (spec 005 §5 behaviors 17 and 18).
//
// `description` is set to nil rather than omitted, because a model without a
// docstring emits `"description": null`. `additionalProperties` is explicit in
// both directions.
func Model(title string, allowExtra bool, properties []Property) *Object {
	props := NewObject()
	var required []any
	for _, property := range properties {
		props.Set(property.Name, property.Schema())
		if !property.Optional {
			required = append(required, property.Name)
		}
	}

	object := NewObject().
		Set("additionalProperties", allowExtra).
		Set("description", nil).
		Set("properties", props).
		Set("title", title).
		Set("type", "object")
	if len(required) > 0 {
		object.Set("required", required)
	}
	return object.Sort()
}
