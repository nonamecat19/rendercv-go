package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

// The mangled class name of spec 007 §3.3 behavior 11.
//
// The `å` becomes one `_`. Both wrong answers read plausibly — transliterating
// to `a` gives `NorwegianBokmalLocale`, dropping gives `NorwegianBokmlLocale` —
// so the case is asserted by name rather than left to the differential, which
// would report it as one of forty-five failures with no clue which rule broke.
func TestDefNameForManglesNonASCII(t *testing.T) {
	cases := map[string]string{
		"english":           "EnglishLocale",
		"mandarin_chinese":  "MandarinChineseLocale",
		"norwegian_bokmål":  "NorwegianBokm_lLocale",
		"norwegian_nynorsk": "NorwegianNynorskLocale",
	}
	for language, want := range cases {
		t.Run(language, func(t *testing.T) {
			if got := locale.DefNameFor(language); got != want {
				t.Errorf("DefNameFor(%q) = %q, want %q", language, got, want)
			}
		})
	}
}

// The `title` keeps the character the `$defs` key loses.
func TestCatalogTitleKeepsNonASCII(t *testing.T) {
	schema := locale.CatalogSchema(locale.Catalogs()["norwegian_bokmål"], "irrelevant")
	properties, _ := schema.Get("title")
	if properties != "NorwegianBokmålLocale" {
		t.Errorf("title = %v, want NorwegianBokmålLocale", properties)
	}
}

// Each variant's descriptions carry **its own** default, not English's
// (spec 007 §3.3 behavior 12).
//
// This is the finding that makes a wrong port look right: reusing English's
// rendered descriptions produces forty-four well-formed English sentences.
func TestDescriptionsCarryTheVariantsOwnDefault(t *testing.T) {
	schema := locale.CatalogSchema(locale.Catalogs()["danish"], "irrelevant")
	properties := objectAt(t, schema, "properties")

	cases := map[string]string{
		"last_updated": `Translation of "Last updated in". The default value is ` +
			"`Senest opdateret`.",
		"month":   "Translation of \"month\" (singular). The default value is `måned`.",
		"years":   "Translation of \"years\" (plural). The default value is `år`.",
		"present": "Translation of \"present\" for ongoing dates. The default value is `nuværende`.",
	}
	for field, want := range cases {
		t.Run(field, func(t *testing.T) {
			got, _ := objectAt(t, properties, field).Get("description")
			if got != want {
				t.Errorf("description = %q, want %q", got, want)
			}
		})
	}
}

// The twelve-element bound reaches the schema for English only.
//
// `create_simple_field_spec` rebuilds a variant's field from
// `base_field_info.annotation`, which has already lost the `Annotated` metadata,
// so the twenty-one variants keep the constraint in validation and lose it in
// their schema.
func TestMonthBoundsAreEnglishOnly(t *testing.T) {
	for _, language := range []string{"english", "danish"} {
		t.Run(language, func(t *testing.T) {
			properties := objectAt(t,
				locale.CatalogSchema(locale.Catalogs()[language], "irrelevant"), "properties")
			field := objectAt(t, properties, "month_abbreviations")

			_, hasMin := field.Get("minItems")
			_, hasMax := field.Get("maxItems")
			want := language == "english"
			if hasMin != want || hasMax != want {
				t.Errorf("minItems=%v maxItems=%v, want both %v", hasMin, hasMax, want)
			}
		})
	}
}

// The `Phrases` keys are numbered in emission order, so Danish is `__3` — third
// in `Languages`, not third alphabetically, which would be Dutch.
func TestPhrasesAreNumberedInEmissionOrder(t *testing.T) {
	names := locale.PhrasesDefNames()
	if len(names) != len(locale.Languages) {
		t.Fatalf("%d names, want %d", len(names), len(locale.Languages))
	}

	const prefix = "rendercv__schema__models__locale__english_locale__Phrases__"
	for i, language := range []string{"english", "arabic", "danish"} {
		if locale.Languages[i] != language {
			t.Fatalf("Languages[%d] = %q, want %q", i, locale.Languages[i], language)
		}
		want := prefix + [...]string{"1", "2", "3"}[i]
		if names[i] != want {
			t.Errorf("%s -> %q, want %q", language, names[i], want)
		}
	}

	// Every variant's `phrases` points at its own entry.
	for i, language := range locale.Languages {
		properties := objectAt(t,
			locale.CatalogSchema(locale.Catalogs()[language], names[i]), "properties")
		got, _ := objectAt(t, properties, "phrases").Get("$ref")
		if got != "#/$defs/"+names[i] {
			t.Errorf("%s phrases $ref = %v, want #/$defs/%s", language, got, names[i])
		}
	}
}

// The union's two orders differ: `oneOf` is English-first, `mapping` is sorted.
func TestUnionOrdersDiffer(t *testing.T) {
	schema := locale.UnionSchema()

	arms, _ := schema.Get("oneOf")
	first, _ := arms.([]any)[0].(*jsonschema.Object).Get("$ref")
	if first != "#/$defs/EnglishLocale" {
		t.Errorf("first oneOf arm = %v, want #/$defs/EnglishLocale", first)
	}

	mapping := objectAt(t, objectAt(t, schema, "discriminator"), "mapping")
	keys := mapping.Keys()
	if keys[0] != "arabic" || keys[3] != "english" {
		t.Errorf("mapping order = %v, want a sorted list with english fourth",
			strings.Join(keys[:4], ", "))
	}
}

func objectAt(t *testing.T, object *jsonschema.Object, key string) *jsonschema.Object {
	t.Helper()
	value, ok := object.Get(key)
	if !ok {
		t.Fatalf("no key %q", key)
	}
	inner, ok := value.(*jsonschema.Object)
	if !ok {
		t.Fatalf("%q is %T, want *jsonschema.Object", key, value)
	}
	return inner
}
