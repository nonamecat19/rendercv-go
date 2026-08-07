package design

// The option tree's shape. The data that fills it is generated
// (`tools/designprobe`); this file is what the generator targets, what the
// validator walks, and what the schema is projected from.
//
// **One tree, not nine.** `ClassicTheme` is the only model upstream declares;
// the eight other built-in themes are `create_variant_pydantic_model` applied to
// override dicts (spec 006 §1 behavior 1). Mirroring that is what keeps the
// port's transcription risk at one tree rather than nine.

// Kind is what a field holds. It decides three things at once — how the
// validator checks a value, how the schema renders the field, and how a variant
// re-splices the field's description — which is why it is one enumeration rather
// than three.
//
// The vocabulary is closed and small: twenty-one models over roughly a hundred
// fields use eleven kinds, measured by enumerating every distinct annotation in
// `classic_theme.py`.
type Kind int

const (
	// KindNested is a sub-model. The field carries no default of its own — the
	// nested model's own defaults are the default — which is why such a field's
	// schema is a bare `$ref` with no `default` key.
	KindNested Kind = iota

	// KindString is a plain `str`. Most of them are template strings, which this
	// iteration models as data and iteration 8 interprets (spec 006 §4.1).
	KindString

	// KindOptionalString is `str | None`. `templates.education_entry.degree_column`
	// is the only one, and its `null` means "no degree column" rather than
	// "unset".
	KindOptionalString

	// KindStringList is `list[str]`: `sections.show_time_spans_in`.
	KindStringList

	// KindBool is `bool`.
	KindBool

	// KindTypstDimension is the pattern of typstdimension.go.
	KindTypstDimension

	// KindColor is the parser of color.go.
	KindColor

	// KindLiteral is a `Literal[...]` union. Ref names its `$defs` entry when it
	// has one; a union pydantic **inlines** carries its members instead — see
	// Field.Ref.
	KindLiteral

	// KindFontFamily is `typography.font_family`, the union of a mapping and a
	// bare name that spec 006 §3.2 behavior 14 widens.
	KindFontFamily

	// KindThemeTag is the discriminator, `Literal["<theme>"]`. It is a kind of
	// its own because its schema carries `const` and its value is pinned per
	// variant rather than overridden like an ordinary default.
	KindThemeTag
)

// Field is one option.
type Field struct {
	// Name is the YAML key.
	Name string

	Kind Kind

	// Nested is the sub-model's name, for KindNested.
	Nested string

	// Ref is the `$defs` key a KindLiteral or KindTypstDimension field points
	// at, empty when the union is inlined.
	//
	// **Whether a union gets an entry is usage count, not whether it is named**
	// (spec 006 §2 behavior 5). `Alignment` is reached twice and has one;
	// `BodyAlignment` is named, reached once, and is inlined; `photo_position`
	// is an anonymous `Literal` and is inlined too. So the port states the
	// answer per field rather than deriving it from the type.
	Ref string

	// Members are a KindLiteral's values, in **declaration** order, for the
	// inlined case and for the `literal_error` message in both cases.
	Members []string

	// Default is the field's default, in the form the schema shows it: a string,
	// a bool, or a []string. A KindColor's is the **rendered** colour —
	// `rgb(0, 0, 0)` and not `black` — because pydantic serializes the default
	// through `Color.__str__`.
	Default any

	// Description is the template, with the base's default already spliced in.
	// A variant re-splices it; see spec 007 §3.3 behavior 12, which measured the
	// same rule on locale.
	Description string

	// Examples is `Field(examples=…)`, omitted when empty.
	Examples []any

	// Title is an explicit `Field(title=…)`. Empty means pydantic's derived
	// title, which is not the same as no title at all — a bare `$ref` field has
	// none either way.
	Title string
}

// Model is one nested option model. Every one is a `BaseModelWithoutExtraKeys`,
// so every one rejects unknown keys (spec 006 §2 behavior 4); there is no flag
// because there is no exception.
type Model struct {
	// Name is the Python class name, which is also the schema's `title` and the
	// stem of its `$defs` key.
	Name string

	// Fields are in **declaration** order, which `properties` preserves and
	// which decides the order errors are reported in.
	Fields []Field
}

// Tree is the whole option tree, keyed by model name, with Root naming the
// entry point.
type Tree struct {
	Root   string
	Models map[string]Model
}

// Field looks up one field of one model, reporting whether it is there.
func (t Tree) Field(model, field string) (Field, bool) {
	for _, candidate := range t.Models[model].Fields {
		if candidate.Name == field {
			return candidate, true
		}
	}
	return Field{}, false
}
