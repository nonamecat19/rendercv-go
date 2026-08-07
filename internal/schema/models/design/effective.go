package design

// Effective resolves a theme's option tree to concrete values — iteration 6's
// T10, cut to iteration 9 because the renderer is its first consumer.
//
// **Nothing validates a default**, which is why this was not needed earlier: a
// theme's overrides change what a field *defaults to*, and validation only ever
// looks at what the document *says*. The renderer needs every value, defaulted
// or not, so this is where the three layers finally combine:
//
//	ClassicTheme's declared defaults      the base tree
//	  ← the theme's override mapping      other_themes/<theme>.yaml
//	    ← the document's own `design`     "sb2nov, but with this margin"
//
// The merge is **deep** at every layer: `create_nested_model_variant_model`
// (variant_pydantic_model_generator.py:280-315) recurses into a nested mapping
// and replaces only the keys the override supplies, so a theme that sets
// `typography.font_size.body` keeps the other four sizes. A shallow merge would
// silently reset every sibling to the base's value — plausible output, wrong
// document.
func Effective(theme string, document map[string]any) map[string]any {
	return EffectiveWithScript(theme, nil, document)
}

// EffectiveWithScript is Effective with a custom theme's declaration merged in
// (spec 014 §4, criterion 3).
//
// **The script sits between the theme and the document**, which is the whole
// ordering question a scripted theme raises: its declarations are *defaults*, so
// they must lose to anything the document says. Upstream gets this for free —
// the script declares a pydantic model and the document's values override its
// field defaults — and the port has to place the layer deliberately.
//
// A nil script is the built-in case and merges nothing.
func EffectiveWithScript(theme string, script, document map[string]any) map[string]any {
	tree := baseTree()
	values := defaultsOf(tree, tree.Root)

	values = deepMerge(values, Overrides(theme))
	values = deepMerge(values, script)
	values = deepMerge(values, document)

	// The discriminator is the theme's own name, not the base's.
	values["theme"] = theme

	// **The two coercions of spec 006 §3.2 run here**, after the merge and before
	// the renderer sees anything. Upstream runs them in field validators, so by
	// the time `process_model` reads the design they have already happened —
	// putting them any later would make every consumer repeat them, and putting
	// them in `deepMerge` would make it lossy for a document that overrides one
	// element on top of a theme's bare string.
	if typography, ok := values["typography"].(map[string]any); ok {
		if name, isString := typography["font_family"].(string); isString {
			widened := make(map[string]any, len(FontFamilyElements))
			for element, value := range WidenFontFamily(name) {
				widened[element] = value
			}
			typography["font_family"] = widened
		}
	}
	if sections, ok := values["sections"].(map[string]any); ok {
		sections["show_time_spans_in"] = SnakeCaseSectionTitles(
			EffectiveStrings(values, "sections", "show_time_spans_in"))
	}

	resolveNulls(tree, tree.Root, values)
	normalizeColors(tree, tree.Root, values)
	return values
}

// resolveNulls decides what a `null` in the merged tree means, which depends
// entirely on the field's declared type.
//
// **One field in the whole design tree is nullable with a non-null default** —
// `templates.education_entry.degree_column`, `str | None = "**DEGREE**"` — and
// for it a null is the documented way to turn the degree column off. Every other
// field would have been rejected by pydantic, so a null there is not a value:
// the declared default is restored rather than a zero being handed to the
// renderer, which would otherwise turn a document upstream *errors* on into one
// the port renders silently wrong.
func resolveNulls(tree Tree, model string, values map[string]any) {
	for _, declared := range tree.Models[model].Fields {
		if declared.Kind == KindNested {
			if nested, ok := values[declared.Name].(map[string]any); ok {
				resolveNulls(tree, declared.Nested, nested)
			}
			continue
		}

		value, present := values[declared.Name]
		if !present || value != nil {
			continue
		}
		if declared.Kind == KindOptionalString {
			continue
		}
		if declared.Default != nil {
			values[declared.Name] = declared.Default
			continue
		}
		delete(values, declared.Name)
	}
}

// normalizeColors puts every colour-typed value through `Color.String()`, which
// is `as_rgb()` — the one string the Typst templates ever see, because upstream's
// colour type has no other `__str__`.
//
// **The declared defaults are already normalized and the overrides are not.**
// `other_themes/sb2nov.yaml` writes `rgb(0,0,0)` and upstream renders
// `rgb(0, 0, 0)`; the theme's YAML text reaches the template unchanged without
// this, and three corpus themes differ by exactly those two spaces. A document
// writing `name: Black` is the same case one layer further down.
//
// It walks the base tree rather than guessing from the value, because `Black`
// and `justified` are both strings and only one of them is a colour.
func normalizeColors(tree Tree, model string, values map[string]any) {
	for _, declared := range tree.Models[model].Fields {
		switch declared.Kind {
		case KindNested:
			if nested, ok := values[declared.Name].(map[string]any); ok {
				normalizeColors(tree, declared.Nested, nested)
			}
		case KindColor:
			text, isText := values[declared.Name].(string)
			if !isText {
				continue
			}
			if color, err := ParseColor(text); err == nil {
				values[declared.Name] = color.String()
			}
		default:
		}
	}
}

// defaultsOf reads one model's declared defaults, recursing into nested models.
//
// A nested field contributes a mapping rather than a value, which is what makes
// the merge below deep by construction — there is nothing to merge *into*
// otherwise.
func defaultsOf(tree Tree, model string) map[string]any {
	values := make(map[string]any, len(tree.Models[model].Fields))
	for _, field := range tree.Models[model].Fields {
		if field.Kind == KindNested {
			values[field.Name] = defaultsOf(tree, field.Nested)
			continue
		}
		if field.Kind == KindFontFamily {
			// `typography.font_family` has no declared default — it is a
			// `default_factory` over the five-element model — so its value comes
			// from that model's own defaults (spec 006 §4).
			values[field.Name] = defaultsOf(tree, "FontFamily")
			continue
		}
		if field.Default != nil {
			values[field.Name] = field.Default
		}
	}
	return values
}

// deepMerge returns `base` with `over` applied, recursing where both sides are
// mappings.
//
// **A string over a mapping replaces it**, which is `font_family: Roboto` — and
// the widening of spec 006 §3.2 behavior 14 then turns it back into the
// five-element form. Doing the widening here instead would make the merge lossy
// for a document that sets one element.
func deepMerge(base, over map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for key, value := range base {
		out[key] = value
	}

	for key, value := range over {
		nested, isMapping := value.(map[string]any)
		if !isMapping {
			out[key] = value
			continue
		}
		existing, wasMapping := out[key].(map[string]any)
		if !wasMapping {
			out[key] = deepMerge(map[string]any{}, nested)
			continue
		}
		out[key] = deepMerge(existing, nested)
	}
	return out
}

// EffectiveString reads one dotted path out of a resolved tree, which is what
// the renderer's field lookups need — `header.connections.separator`.
//
// It returns the empty string for a missing path rather than reporting one,
// because every path the renderer asks for comes from the tree it just built.
func EffectiveString(values map[string]any, path ...string) string {
	value := lookup(values, path)
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// EffectiveBool is EffectiveString for a boolean option.
func EffectiveBool(values map[string]any, path ...string) bool {
	value, _ := lookup(values, path).(bool)
	return value
}

// EffectiveStrings is EffectiveString for `sections.show_time_spans_in`, the one
// list-valued option.
func EffectiveStrings(values map[string]any, path ...string) []string {
	switch typed := lookup(values, path).(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

func lookup(values map[string]any, path []string) any {
	var current any = values
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = mapping[key]
	}
	return current
}
