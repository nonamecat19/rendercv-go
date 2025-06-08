package design

import "fmt"

// classicThemeModule qualifies every nested option model's `$defs` key. The bare
// names collide with the CV entry models of spec 003 §3.1 — `EducationEntry`
// exists in both trees and means different things — which is what forces
// qualification on **both** sets (spec 006 §2 behavior 4).
const classicThemeModule = "rendercv.schema.models.design.classic_theme"

// Ordinals assigns every (model, theme) pair its `$defs` number.
//
// **The rule is measured, and it is not "nine per model"** (plan §4). Walking
// the nine themes in union order: a theme whose override mapping names a nested
// field produces a **new** class for that field's model, and a theme that omits
// it points back at the base's `__1`. So `Page` has six numbers and `Links`
// eight, and eleven of the models have fewer than nine.
//
// The walk is depth-first inside the theme loop, and it descends only where the
// override descends: a theme that names `typography` but not `typography.bold`
// gets a new `Typography` and reuses `Bold__1`. That is
// `create_nested_model_variant_model`'s own recursion
// (variant_pydantic_model_generator.py:280-315), which only visits keys the
// override supplies.
func Ordinals() map[string]map[string]int {
	tree := baseTree()
	assigned := map[string]map[string]int{}
	counters := map[string]int{}

	assign := func(model, theme string) int {
		counters[model]++
		if assigned[model] == nil {
			assigned[model] = map[string]int{}
		}
		assigned[model][theme] = counters[model]
		return counters[model]
	}

	// base is the whole tree, which is what ClassicTheme contributes: every
	// nested model exists once before any override is applied, and those are the
	// `__1`s the other eight fall back to.
	var walk func(model, theme string, override map[string]any, base bool)
	walk = func(model, theme string, override map[string]any, base bool) {
		assign(model, theme)
		for _, field := range tree.Models[model].Fields {
			if field.Kind != KindNested {
				continue
			}
			if base {
				walk(field.Nested, theme, nil, true)
				continue
			}
			nested, overridden := override[field.Name].(map[string]any)
			if !overridden {
				// Not named by this theme, so no new class: the reference points
				// back at the base's. Descending anyway would number models the
				// theme never touched.
				continue
			}
			walk(field.Nested, theme, nested, false)
		}
	}

	// **The root is not numbered.** `ClassicTheme`, `EmberTheme` and the rest are
	// distinct class names, so pydantic never has to qualify or number them —
	// ThemeDefName is their key. Numbering them would produce nine entries that
	// do not exist.
	for _, theme := range Themes() {
		isBase := theme == BaseTheme
		for _, field := range tree.Models[tree.Root].Fields {
			if field.Kind != KindNested {
				continue
			}
			if isBase {
				walk(field.Nested, theme, nil, true)
				continue
			}
			if nested, overridden := Overrides(theme)[field.Name].(map[string]any); overridden {
				walk(field.Nested, theme, nested, false)
			}
		}
	}
	return assigned
}

// ModelCounts is how many classes each nested model produced.
func ModelCounts() map[string]int {
	counts := map[string]int{}
	for model, byTheme := range Ordinals() {
		counts[model] = len(byTheme)
	}
	return counts
}

// DefNameOf is a nested option model's `$defs` key for one theme.
//
// **A model only one theme produced carries no suffix at all.** `OneLineEntry`
// and `PublicationEntry` are the two: no override file names them, so the
// qualified name is already unique and pydantic numbers nothing. An
// implementation that suffixed unconditionally would emit
// `…OneLineEntry__1` where upstream emits `…OneLineEntry` — two of the 161
// wrong, and the other 159 right.
func DefNameOf(model, theme string) string {
	ordinals := Ordinals()
	number, ok := ordinals[model][theme]
	if !ok {
		// The theme did not produce this model, so it reuses the base's, which
		// is always the first.
		number = ordinals[model][BaseTheme]
	}

	qualified := qualify(model)
	if len(ordinals[model]) == 1 {
		return qualified
	}
	return fmt.Sprintf("%s__%d", qualified, number)
}

// ThemeDefName is a theme model's own key, which is bare rather than qualified:
// `ClassicTheme`, `EmberTheme`, … Nothing else in the tree is called that, so
// pydantic never has to qualify them.
func ThemeDefName(theme string) string {
	return capitalizeASCII(theme) + "Theme"
}

func qualify(model string) string {
	return replaceDots(classicThemeModule) + "__" + model
}

func replaceDots(module string) string {
	out := make([]byte, 0, len(module)*2)
	for i := 0; i < len(module); i++ {
		if module[i] == '.' {
			out = append(out, '_', '_')
			continue
		}
		out = append(out, module[i])
	}
	return string(out)
}

// capitalizeASCII is `generate_model_name`'s per-word `str.capitalize()`
// (variant_pydantic_model_generator.py:171-184), which is why
// `engineeringclassic` becomes `Engineeringclassic` and not
// `EngineeringClassic` — the name is one word to Python because it has no
// underscore.
//
// Every theme name is ASCII and single-word, so this needs none of the Unicode
// care `locale.DefNameFor` does; a name that needed it would be a name upstream
// mangles, and the schema differential is what would say so.
func capitalizeASCII(name string) string {
	if name == "" {
		return name
	}
	first := name[0]
	if first >= 'a' && first <= 'z' {
		first -= 'a' - 'A'
	}
	return string(first) + name[1:]
}
