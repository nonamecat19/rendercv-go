package locale

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
)

// phrasesModule is the module the colliding `Phrases` classes are qualified
// with (spec 007 §3.3 behavior 12).
//
// **It is `english_locale`, not `locale`**, for every one of the twenty-two —
// including the twenty-one generated variants. `create_nested_field_spec` builds
// each variant with `__module__=base_model_class.__module__`
// (variant_pydantic_model_generator.py:322), and the base class is
// `english_locale.Phrases`. So the qualified name says English even for Danish,
// and only the `__N` suffix distinguishes them.
const phrasesModule = "rendercv.schema.models.locale.english_locale"

// The ten description templates of `EnglishLocale` (english_locale.py:22-95).
//
// They are **templates**, not rendered strings: each variant is generated with
// `update_description_with_new_default`, which splices its own default into the
// English text (variant_pydantic_model_generator.py:196-218). Danish's
// `last_updated` therefore reads "… The default value is `Senest opdateret`."
// A port that reused English's rendered descriptions would emit 44 strings that
// look right and are wrong (spec 007 §3.3 behavior 12).
const (
	descLanguage    = "The language for your CV. The default value is `english`."
	descLastUpdated = `Translation of "Last updated in". The default value is ` +
		"`Last updated in`."
	descMonth   = `Translation of "month" (singular). The default value is ` + "`month`."
	descMonths  = `Translation of "months" (plural). The default value is ` + "`months`."
	descYear    = `Translation of "year" (singular). The default value is ` + "`year`."
	descYears   = `Translation of "years" (plural). The default value is ` + "`years`."
	descPresent = `Translation of "present" for ongoing dates. The default value is ` +
		"`present`."
	descPhrases            = "Locale-specific phrases used in entry templates as placeholders."
	descMonthAbbreviations = "Month abbreviations (Jan-Dec)."
	descMonthNames         = "Full month names (January-December)."
	descDegreeWithArea     = "Template for combining degree and area in education" +
		" entries. Available placeholders: DEGREE, AREA. The default value is" +
		" `DEGREE in AREA`."
)

// SchemaDefs is the 45 locale `$defs`: `Locale`, twenty-two `<Language>Locale`
// models, and the twenty-two colliding `Phrases` models.
func SchemaDefs() map[string]*jsonschema.Object {
	defs := map[string]*jsonschema.Object{"Locale": UnionSchema()}

	catalogs := Catalogs()
	phrases := PhrasesDefNames()
	for i, name := range Languages {
		defs[DefNameFor(name)] = CatalogSchema(catalogs[name], phrases[i])
		defs[phrases[i]] = PhrasesSchema(catalogs[name])
	}
	return defs
}

// PhrasesDefNames is the twenty-two `Phrases` keys, numbered in emission order.
//
// Emission order is `Languages`' order — English first, then sorted filename
// order — which is why Danish's `phrases` points at `__3` and not at the third
// name alphabetically (spec 005 §3.3 behavior 13).
func PhrasesDefNames() []string {
	return jsonschema.SuffixedNames("Phrases", phrasesModule, Languages)
}

// DefNameFor is a language's `$defs` key.
//
// Two steps, and the second is the one that is not guessable. First
// `generate_model_name` (variant_pydantic_model_generator.py:171-184) joins the
// underscore-separated words after `str.capitalize()` and appends `Locale`.
// Then pydantic sanitizes the class name for use as a `$defs` key by replacing
// every character outside `[a-zA-Z0-9.\-_]` with a single `_` — so
// `norwegian_bokmål` becomes `NorwegianBokm_lLocale`, the `å` replaced rather
// than transliterated to `a` or dropped (spec 007 §3.3 behavior 11).
func DefNameFor(languageName string) string {
	words := strings.Split(languageName, "_")
	for i, word := range words {
		words[i] = capitalize(word)
	}
	return sanitizeDefName(strings.Join(words, "") + "Locale")
}

// UnionSchema is `Locale`, the discriminated union (locale.py:43-46).
//
// `oneOf` is in union order and `mapping` is sorted by tag; the two orders
// differ, which is visible in upstream's file — `english` is the first arm and
// the fourth mapping entry.
func UnionSchema() *jsonschema.Object {
	arms := make([]any, 0, len(Languages))
	for _, name := range Languages {
		arms = append(arms, jsonschema.Ref(DefNameFor(name)))
	}

	tags := append([]string(nil), Languages...)
	sort.Strings(tags)
	mapping := jsonschema.NewObject()
	for _, tag := range tags {
		mapping.Set(tag, "#/$defs/"+DefNameFor(tag))
	}

	return jsonschema.NewObject().
		Set("discriminator", jsonschema.NewObject().
			Set("mapping", mapping).
			Set("propertyName", "language")).
		Set("oneOf", arms).
		Sort()
}

// CatalogSchema is one `<Language>Locale` model.
//
// **The month lists carry `minItems`/`maxItems` only for English.** The bound
// lives in the `Annotated` metadata of `english_locale.py:60` and `:79`, and
// `create_simple_field_spec` rebuilds each variant's field from
// `base_field_info.annotation` (variant_pydantic_model_generator.py:428-431),
// which is the bare `list[str]` with the metadata already stripped off. So the
// twenty-one variants lose the constraint in their schema while keeping it in
// their validation — an asymmetry that is upstream's, not the port's.
func CatalogSchema(catalog Catalog, phrasesDef string) *jsonschema.Object {
	english := English()
	isEnglish := catalog.Language == english.Language

	properties := jsonschema.NewObject().
		Set("language", jsonschema.NewObject().
			Set("const", catalog.Language).
			Set("default", catalog.Language).
			Set("description", respliced(descLanguage, english.Language, catalog.Language)).
			Set("title", jsonschema.TitleFor("language")).
			Set("type", "string").
			Sort()).
		Set("last_updated", stringField("last_updated", descLastUpdated,
			english.LastUpdated, catalog.LastUpdated)).
		Set("month", stringField("month", descMonth, english.Month, catalog.Month)).
		Set("months", stringField("months", descMonths, english.Months, catalog.Months)).
		Set("year", stringField("year", descYear, english.Year, catalog.Year)).
		Set("years", stringField("years", descYears, english.Years, catalog.Years)).
		Set("present", stringField("present", descPresent, english.Present, catalog.Present)).
		Set("phrases", jsonschema.Ref(phrasesDef).
			Set("description", descPhrases).
			Sort()).
		Set("month_abbreviations", monthListField("month_abbreviations",
			descMonthAbbreviations, catalog.MonthAbbreviations, isEnglish)).
		Set("month_names", monthListField("month_names", descMonthNames,
			catalog.MonthNames, isEnglish))

	return jsonschema.NewObject().
		Set("additionalProperties", false).
		Set("properties", properties).
		Set("title", modelNameFor(catalog.Language)).
		Set("type", "object").
		Sort()
}

// PhrasesSchema is one `Phrases` model: a single field, its default spliced into
// the English template.
func PhrasesSchema(catalog Catalog) *jsonschema.Object {
	properties := jsonschema.NewObject().
		Set("degree_with_area", stringField("degree_with_area", descDegreeWithArea,
			English().DegreeWithArea, catalog.DegreeWithArea))

	return jsonschema.NewObject().
		Set("additionalProperties", false).
		Set("properties", properties).
		// **Not the `$defs` key.** All twenty-two are titled `Phrases`; only the
		// key carries the `__N` that tells them apart.
		Set("title", "Phrases").
		Set("type", "object").
		Sort()
}

func stringField(name, template, englishDefault, value string) *jsonschema.Object {
	return jsonschema.NewObject().
		Set("default", value).
		Set("description", respliced(template, englishDefault, value)).
		Set("title", jsonschema.TitleFor(name)).
		Set("type", "string").
		Sort()
}

// monthListField is a twelve-element list. `bounded` is English only; see
// CatalogSchema.
func monthListField(name, description string, values []string, bounded bool) *jsonschema.Object {
	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}

	object := jsonschema.NewObject().
		Set("default", items).
		Set("description", description).
		Set("items", jsonschema.NewObject().Set("type", "string")).
		Set("title", jsonschema.TitleFor(name)).
		Set("type", "array")
	if bounded {
		object.Set("maxItems", MonthCount).Set("minItems", MonthCount)
	}
	return object.Sort()
}

// respliced is `update_description_with_new_default`
// (variant_pydantic_model_generator.py:196-218): the old default, in backticks,
// replaced by the new one. Nothing else in the description changes, which is why
// `Month abbreviations (Jan-Dec).` is identical in all twenty-two — the list's
// `str()` never appears in its own text.
func respliced(description, oldDefault, newDefault string) string {
	return strings.ReplaceAll(description, "`"+oldDefault+"`", "`"+newDefault+"`")
}

// modelNameFor is the class name before pydantic sanitizes it, which is what
// `title` carries: `NorwegianBokmålLocale` keeps its `å` while the `$defs` key
// loses it.
func modelNameFor(languageName string) string {
	words := strings.Split(languageName, "_")
	for i, word := range words {
		words[i] = capitalize(word)
	}
	return strings.Join(words, "") + "Locale"
}

// sanitizeDefName is pydantic's normalization of a class name into a `$defs`
// key: every character outside `[a-zA-Z0-9.\-_]` becomes one `_`.
func sanitizeDefName(name string) string {
	var out strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	return out.String()
}

// firstRuneTitler and capitalize are Python's `str.capitalize()`, the same pair
// `cv/section.go` needs and for the same reason: `unicode.ToTitle` is
// rune-to-rune and cannot express the mappings where one rune becomes several.
// No language name exercises those today, and writing the weaker version would
// make a twenty-third that did fail silently.
var firstRuneTitler = cases.Title(language.Und, cases.NoLower)

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	var rest strings.Builder
	for i := 1; i < len(runes); i++ {
		rest.WriteRune(unicode.ToLower(runes[i]))
	}
	return firstRuneTitler.String(string(runes[0])) + rest.String()
}
