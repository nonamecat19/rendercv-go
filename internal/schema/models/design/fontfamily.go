package design

// FontFamilies is `available_font_families` (design/font_family.py:5-27), in the
// order that reaches the schema.
//
// **`sorted()`, not source order.** Upstream writes the list in two commented
// groups — three Typst built-ins, then fourteen RenderCV-bundled — and wraps the
// whole thing in `sorted()`. A port that carried the source order would produce
// a seventeen-member `enum` with every member present and none in the right
// place (spec 006 §3.1 behavior 13).
var FontFamilies = []string{
	"DejaVu Sans Mono",
	"EB Garamond",
	"Fontin",
	"Gentium Book Plus",
	"Lato",
	"Libertinus Serif",
	"Mukta",
	"New Computer Modern",
	"Noto Sans",
	"Open Sans",
	"Open Sauce Sans",
	"Poppins",
	"Raleway",
	"Roboto",
	"Source Sans 3",
	"Ubuntu",
	"XCharter",
}

// FontFamilyElements is the five keys of `classic_theme.FontFamily`
// (classic_theme.py), in declaration order. They are also the elements a bare
// string widens into — see WidenFontFamily.
var FontFamilyElements = []string{"body", "name", "headline", "connections", "section_titles"}

// DefaultFontFamily is the value every element of `ClassicTheme`'s
// `typography.font_family` carries.
const DefaultFontFamily = "Source Sans 3"

// ValidFontFamily reports whether a font name is accepted, which is **always**.
//
// `FontFamily` is `SkipJsonSchema[str] | Literal[*available_font_families]`
// (font_family.py:30): a free string arm hidden from the schema, unioned with
// the seventeen. So any system font validates, the seventeen exist only to drive
// editor completion, and there is no failure message to match
// (spec 006 §3.1 behavior 12).
//
// The function exists rather than being inlined as `true` because the schema and
// the validator disagree here on purpose, and a reader who finds the seventeen
// in `FontFamilies` will look for the check that rejects the other names.
func ValidFontFamily(string) bool { return true }

// IsKnownFontFamily reports membership of the seventeen. It drives the schema's
// `enum`, never a rejection.
func IsKnownFontFamily(name string) bool {
	for _, known := range FontFamilies {
		if known == name {
			return true
		}
	}
	return false
}

// WidenFontFamily is `validate_font_family`'s string branch
// (classic_theme.py:280-300, `mode="plain"`): a bare string becomes the full
// five-element object with every element set to it.
//
// **A coercion, not a validation** (spec 006 §3.2 behavior 14). It raises
// nothing, and it is observable only in rendered output — so the test that
// really pins it is the renderer's, and this one only fixes the shape.
func WidenFontFamily(name string) map[string]string {
	widened := make(map[string]string, len(FontFamilyElements))
	for _, element := range FontFamilyElements {
		widened[element] = name
	}
	return widened
}
