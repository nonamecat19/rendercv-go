package design

import (
	"errors"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
)

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
	return EffectiveWithScript(theme, nil, document, false)
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
// A nil script is the built-in case, or a custom theme whose script failed to
// load, parse, run or validate — `hasScript` is what tells those two apart.
func EffectiveWithScript(theme string, script, document map[string]any, hasScript bool) map[string]any {
	tree := baseTree()
	values := defaultsOf(tree, tree.Root)

	values = deepMerge(values, Overrides(theme))
	// **An empty Lua table at a `KindStringList` field must become an empty
	// list before this merge, not after.** Lua has one table type for both a
	// sequence and a mapping, so `luatheme.Options` turns `{}` into an empty
	// Go map; `ValidateScript` already carves out that ambiguity (see its own
	// comment above), but this merge is a second, independent place the same
	// empty map does damage — deepMerge writes it over the tree's `[]string`
	// base default, so `withoutTreeConflictsAt` below then sees a **map** at
	// that path and drops the document's real list override as a shape
	// conflict, silently losing a legitimate `show_time_spans_in` override.
	// Found by a fresh-context verifier (iteration 14's tenth re-verification).
	normalizeScriptStringLists(tree, tree.Root, script)
	values = deepMerge(values, script)

	// **A theme with genuinely no script file discards the whole document
	// `design` block, not just its own (nonexistent) options.** Upstream's
	// fallback constructs `ThemeOptionsAreNotProvided(theme=theme_name)`
	// (`design.py:139-142`) — nothing but `theme` survives, so a document
	// overriding, say, `design.colors.name` on a theme with no `init.lua` is
	// silently ignored upstream: the artifact still carries classic's own
	// default. A **built-in** theme is unaffected — `IsBuiltinTheme` guards it
	// regardless of `hasScript`.
	//
	// **This must key on whether a script *file* exists, not on whether `script`
	// is nil.** A script that exists but fails to parse, run or validate also
	// hands this function a nil `script` — conflating the two used to discard a
	// user's whole document on a theme with a merely-broken script, which is a
	// worse outcome than upstream's (upstream refuses to render at all; this
	// port would render with classic's silently-substituted colours instead of
	// the ones the user asked for). That failure mode already has its own open
	// finding (spec 014 §2 behavior 9 — a broken script should be reported, not
	// silently ignored); this function does not compound it. Found by a
	// fresh-context verifier (`specs/STATE.md`, iteration 14's third
	// re-verification).
	if !hasScript && !IsBuiltinTheme(theme) {
		document = nil
	}

	// **A document value that conflicts with what the script declared is
	// dropped, not merged.** `ValidateScript` above only checks the script's own
	// shapes against the base tree; it says nothing about a script-*invented*
	// option, which has no tree shape to check against at all — that is
	// `luatheme.Validate`'s job (spec 014 §4 criterion 2's other half, dead code
	// until this call). Without it, `custom_note: {a: 1}` against a script
	// default of `custom_note = "hello"` would merge the map straight through
	// and print a Go type name into the artifact, the same failure mode
	// `ValidateScript` exists to prevent — merging blindly at the end would have
	// undone that check for every conflicting key anyway.
	//
	// **This only catches conflicts against what the script declared.** A
	// document setting a *base-tree* field — `page.size: {a: 1}` — that the
	// script never mentions (an empty `return {}`, which is what `create-theme`
	// itself writes, declares nothing at all) still merges straight through
	// `deepMerge` below and leaks the same Go type name. `withoutTreeConflicts`
	// closes that second path by checking the document against the tree-typed
	// values already assembled above, for custom themes only — built-in themes
	// go through `validateModel`'s real, error-producing check instead
	// (`validate.go`), so this is deliberately silent-drop rather than a
	// rejection: reproducing upstream's exit-1 `theme_data_model_class(**design)`
	// forbid-extra validation needs the script loaded during *validation*, not
	// only here at render time, and stays open (`specs/STATE.md`).
	if !IsBuiltinTheme(theme) {
		document = withoutTreeConflicts(document, values)
	}
	resolvedDocument := withoutConflicts(script, document, luatheme.Validate(script, document))
	values = deepMerge(values, resolvedDocument)

	// **A partial `font_family` mapping is not a deep merge.** Measured on
	// `theme: opal` (whose own font is Lato) plus `font_family.body: Charter`:
	// upstream emits Charter for `body` and **`Source Sans 3`** — the *base*
	// `FontFamily` default — for the other four, not Lato. pydantic builds a new
	// `FontFamily` from the document's mapping, so whatever the theme or script
	// declared is replaced wholesale rather than merged into. `deepMerge` above
	// cannot express that on its own: by the time it runs, the existing value at
	// `typography.font_family` may already be a bare string (the theme's or the
	// script's), and merging a map "into" a string produces a merge onto an
	// **empty** map, not onto `FontFamily`'s own defaults — dropping the four
	// sibling fields to the zero value instead of their declared default. So a
	// document mapping override is re-applied here, onto a fresh set of base
	// defaults, overriding whatever `deepMerge` just did for that one field.
	if docTypography, ok := resolvedDocument["typography"].(map[string]any); ok {
		if docFontFamily, ok := docTypography["font_family"].(map[string]any); ok {
			if typography, ok := values["typography"].(map[string]any); ok {
				typography["font_family"] = deepMerge(defaultsOf(tree, "FontFamily"), docFontFamily)
			}
		}
	}

	// The discriminator is the theme's own name, not the base's.
	values["theme"] = theme

	// **The two coercions of spec 006 §3.2 run here**, after the merge and before
	// the renderer sees anything. Upstream runs them in field validators, so by
	// the time `process_model` reads the design they have already happened —
	// putting them any later would make every consumer repeat them, and putting
	// them in `deepMerge` would make it lossy for a document that overrides one
	// element on top of a theme's bare string.
	widenFontFamilyIn(values)
	if sections, ok := values["sections"].(map[string]any); ok {
		sections["show_time_spans_in"] = SnakeCaseSectionTitles(
			EffectiveStrings(values, "sections", "show_time_spans_in"))
	}

	resolveNulls(tree, tree.Root, values)
	normalizeColors(tree, tree.Root, values)
	normalizeBools(tree, tree.Root, values)
	return values
}

// fontFamilyPath is the one field in the whole tree where a bare scalar is a
// **documented** override of a mapping — `font_family: Roboto` replaces the
// five-element `FontFamily` model wholesale (`deepMerge`'s own comment above,
// spec 006 §3.2 behavior 14) and `widenFontFamilyIn` turns it back into that
// shape afterwards. `withoutTreeConflicts` must not treat that as a conflict,
// or a legitimate `typography.font_family: Charter` override on a custom
// theme is silently discarded — a regression a verifier caught the first time
// this function shipped.
const fontFamilyPath = "typography.font_family"

// withoutTreeConflicts drops document keys whose shape disagrees with the
// tree-typed value already assembled at that path — a group or a list where a
// scalar belongs, or the reverse — leaving the tree's (or the theme's, or the
// script's) own value in place instead of merging the mismatch through.
//
// A key `values` does not know about at all is left alone: it is neither a
// tree field nor something a script declared, so there is no typed value to
// conflict with and no template reads it. This is not upstream's forbid-extra
// rejection (spec 014's Finding 3, still open) — only the narrower leak this
// exists to close, where a *typed* field gets overridden with the wrong shape
// and reaches a template as a Go type name.
func withoutTreeConflicts(document, values map[string]any) map[string]any {
	return withoutTreeConflictsAt(document, values, "")
}

func withoutTreeConflictsAt(document, values map[string]any, prefix string) map[string]any {
	if len(document) == 0 {
		return document
	}
	out := make(map[string]any, len(document))
	for key, docValue := range document {
		treeValue, present := values[key]
		if !present {
			out[key] = docValue
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		// **`font_family` accepts either shape at every layer** (spec 006 §3.1
		// behavior 12: a bare name, or the five-element mapping), so neither
		// direction is a conflict here — only checking the string-over-mapping
		// direction let a *mapping* document override vanish against a
		// *scalar* script declaration, since `values[fontFamilyPath]` is still
		// the script's un-widened string at this point in the pipeline
		// (`widenFontFamilyIn` runs after this merge). Found by a
		// fresh-context verifier (iteration 14's fourth re-verification).
		if path == fontFamilyPath {
			// **The mapping form is still a `FontFamily`, whose five fields are
			// each a scalar string** — this carve-out used to let the whole
			// mapping through unchecked, so `typography.font_family.body: {x:
			// 1}` reached `deepMerge` (via the re-apply block above) and printed
			// a Go type name into the artifact on a custom theme, where the
			// same document against `classic` correctly fails validation.
			// Found by a fresh-context verifier (iteration 14's eighth
			// re-verification).
			// **A list is neither of `font_family`'s two legal shapes.** The
			// map and scalar arms above were the only ones this carve-out ever
			// considered; a list fell to `out[key] = docValue` same as a
			// scalar, reaching `deepMerge` and then pongo2 as a slice where a
			// dict-or-string was expected — a raw template-engine error, not
			// the panel upstream and the `classic`-theme path both produce.
			// Found by a fresh-context verifier (iteration 14's ninth
			// re-verification).
			switch shapeKind(docValue) {
			case "map":
				docNested := docValue.(map[string]any)
				filtered := make(map[string]any, len(docNested))
				for element, elementValue := range docNested {
					if shapeKind(elementValue) == "scalar" {
						filtered[element] = elementValue
					}
				}
				out[key] = filtered
			case "scalar":
				out[key] = docValue
			}
			continue
		}

		docNested, docIsMap := docValue.(map[string]any)
		treeNested, treeIsMap := treeValue.(map[string]any)
		if docIsMap && treeIsMap {
			out[key] = withoutTreeConflictsAt(docNested, treeNested, path)
			continue
		}
		if shapeKind(docValue) != shapeKind(treeValue) {
			// **A colour tuple is a legal alternate shape for a colour
			// field**, the same way a mapping is for `font_family` above —
			// but this function has no tree Kind to check against, only the
			// shape of the value already assembled at this path. A colour
			// field's tree value is always a scalar `Color.String()`, so
			// "the tree value is a string a colour parses" is a safe proxy:
			// a document override list against, say, `page.size` (a
			// literal) would have to be a list of one of its own members to
			// pass `ParseColor`, which none of `page.size`'s literals are.
			// A document list here already validated at the field's real
			// declared shape (`validColorNode`), before it ever reached this
			// merge — this only keeps it from being pruned on the way in.
			// Found by a fresh-context verifier (iteration 14's thirteenth
			// re-verification).
			if treeText, isText := treeValue.(string); isText && shapeKind(docValue) == "list" && ValidColor(treeText) {
				out[key] = docValue
				continue
			}
			// **The reverse direction is just as real.** A scripted theme
			// can declare a colour as a tuple (`colors.name = {1, 2, 3}`),
			// which by this point in the merge is the *tree* value at this
			// path — so a document overriding it with an ordinary string
			// (`colors: {name: red}`) hits the same shape mismatch from the
			// other side and was silently dropped, keeping the script's
			// colour instead of the document's. Found by a fresh-context
			// verifier (iteration 14's fourteenth re-verification).
			if docText, isText := docValue.(string); isText && shapeKind(treeValue) == "list" && ValidColor(docText) {
				out[key] = docValue
				continue
			}
			// Dropped: the shapes disagree, so the typed value beneath survives.
			continue
		}
		out[key] = docValue
	}
	return out
}

// shapeKind classifies a value the way a template's Typst emission would —
// a map, a list, or anything else — which is coarser than a full type match
// but catches every shape that prints as a Go type name rather than a value.
func shapeKind(value any) string {
	switch value.(type) {
	case map[string]any:
		return "map"
	case []string, []any:
		return "list"
	default:
		return "scalar"
	}
}

// withoutConflicts drops the document keys `luatheme.Validate` flagged,
// leaving the script's (or the base tree's) default for that path in place —
// the merge equivalent of `ValidateScript`'s whole-script drop, scoped to just
// the offending key so a document error in one option cannot suppress a
// correct one beside it.
func withoutConflicts(script, document map[string]any, conflicts []error) map[string]any {
	if len(conflicts) == 0 {
		return document
	}
	paths := make(map[string]bool, len(conflicts))
	for _, err := range conflicts {
		var typeErr *luatheme.TypeError
		if !errors.As(err, &typeErr) {
			continue
		}
		if typeErr.Path == fontFamilyPath {
			// **`luatheme.Validate` does not know about the one field that
			// accepts either shape.** A script declaring `font_family = "Lato"`
			// (a scalar) against a document overriding it with the five-element
			// mapping form is not a real conflict — both are valid `font_family`
			// shapes (spec 006 §3.1 behavior 12) — but `kindOf` classifies them
			// as "a value" and "a group of options" and flags it anyway.
			// `withoutTreeConflicts` above already carries this same exemption;
			// this is the second place a font_family conflict can be pruned from
			// and it needs the same carve-out or the document's override is
			// dropped right back out here. Found by a fresh-context verifier
			// (iteration 14's fourth re-verification).
			continue
		}
		// **A colour conflict here is the same false positive as
		// font_family's, one level more generic.** `luatheme.Validate` has
		// no design-tree Kind to check against — it only compares Go
		// shapes — so a script declaring `colors.name` as a string against
		// a document overriding it with a tuple (or the reverse) reads as
		// "a value" vs "a list" and gets flagged, even though both are
		// legal `Color` shapes. `withoutTreeConflictsAt` already carves
		// this out for the *tree's* default; this is the second site,
		// symmetric with font_family's, for a *script's* declared colour.
		// Found by a fresh-context verifier (iteration 14's fourteenth
		// re-verification).
		if looksLikeColorConflict(script, document, typeErr.Path) {
			continue
		}
		paths[typeErr.Path] = true
	}
	return prunePaths(document, "", paths)
}

// looksLikeColorConflict reports whether a script/document mismatch at path
// is a colour field's two legal shapes talking past each other — a scalar
// `Color` string on one side and a tuple on the other. `luatheme` cannot
// import this package's `ParseColor`/`ParseColorTuple` (this package already
// imports `luatheme`), so this is a narrower proxy: one side must be a list
// and the other a string `ValidColor` accepts, which no other field's legal
// document value satisfies (a `page.size` literal like `a4` is not a colour
// name).
func looksLikeColorConflict(script, document map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	scriptValue := lookup(script, parts)
	docValue := lookup(document, parts)

	scriptIsColorScalar := isColorScalar(scriptValue)
	docIsColorScalar := isColorScalar(docValue)
	scriptIsList := shapeKind(scriptValue) == "list"
	docIsList := shapeKind(docValue) == "list"

	return (scriptIsList && docIsColorScalar) || (docIsList && scriptIsColorScalar)
}

func isColorScalar(value any) bool {
	text, ok := value.(string)
	return ok && ValidColor(text)
}

func prunePaths(document map[string]any, prefix string, paths map[string]bool) map[string]any {
	out := make(map[string]any, len(document))
	for key, value := range document {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if paths[path] {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = prunePaths(nested, path, paths)
			continue
		}
		out[key] = value
	}
	return out
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
			// **A sequence-valued colour reached the template unconverted.**
			// `colors.name: [1, 2, 3]` validates (once `validColorNode`
			// range-checks it) but `bridge.mappingOf` projects any YAML
			// sequence as `[]string`, and this function only ever looked for
			// a `string` — so the raw slice went straight to the Typst
			// emitter, which printed the Go value rather than `rgb(1, 2,
			// 3)`. Found by a fresh-context verifier (iteration 14's
			// twelfth re-verification).
			switch value := values[declared.Name].(type) {
			case string:
				if color, err := ParseColor(value); err == nil {
					values[declared.Name] = color.String()
				}
			case []string:
				if color, err := ParseColorTuple(value); err == nil {
					values[declared.Name] = color.String()
				}
			}
		default:
		}
	}
}

// normalizeBools coerces a `KindBool` field's value the way pydantic's
// `str_as_bool` does: a document (or a script) can write the value as a
// YAML word-form boolean (`show_footer: no`) or as `0`/`1`, and those reach
// this map as the raw text or number rather than a Go `bool` — the design
// block's projection keeps every scalar's source text (`mappingOf`,
// `internal/renderer/bridge/model.go`), and neither `deepMerge` nor
// `luatheme.Validate`'s type check converts it. Left uncoerced, the Typst
// emitter interpolates the literal text `no` where a `false` token belongs,
// which does not compile — measured on both a scripted and a built-in theme.
func normalizeBools(tree Tree, model string, values map[string]any) {
	for _, declared := range tree.Models[model].Fields {
		switch declared.Kind {
		case KindNested:
			if nested, ok := values[declared.Name].(map[string]any); ok {
				normalizeBools(tree, declared.Nested, nested)
			}
		case KindBool:
			switch value := values[declared.Name].(type) {
			case bool:
				// already the right shape
			case string:
				if boolFalsy[strings.ToLower(value)] {
					values[declared.Name] = false
				} else if boolWords[strings.ToLower(value)] {
					values[declared.Name] = true
				} else if number, ok := parseNumericText(value, true); ok {
					// **A numeric spelling has to be coerced by its value.**
					// pydantic accepts any number worth 0 or 1 at a bool field
					// (`validBoolNode`), and `mappingOf` hands this map the
					// source text — so `links.underline: 0o0` arrives as the
					// string `"0o0"`, which is neither of the word sets above
					// and used to reach the Typst emitter as that literal text.
					// Upstream emits `links-underline: false,` for it. Like
					// `ParseColorTuple`, the merge layer has lost the node's
					// `yamldoc.Kind` and so coerces unconditionally; a quoted
					// `"0o0"` never gets this far because `validBoolNode` — which
					// does still have the kind — rejects it first. Found by a
					// fresh-context verifier (iteration 14's twenty-second
					// re-verification).
					values[declared.Name] = number != 0
				}
			case int:
				values[declared.Name] = value != 0
			}
		default:
		}
	}
}

// normalizeScriptStringLists rewrites an empty map at a `KindStringList`
// field to an empty list, in place, so `deepMerge` writes the shape the tree
// actually expects rather than the ambiguous Lua encoding of an empty table.
// A non-empty map at that path is left alone — `validateScript` already
// rejects it as a real conflict, and this function is not the one that
// enforces that.
func normalizeScriptStringLists(tree Tree, model string, script map[string]any) {
	if script == nil {
		return
	}
	for _, declared := range tree.Models[model].Fields {
		switch declared.Kind {
		case KindNested:
			if nested, ok := script[declared.Name].(map[string]any); ok {
				normalizeScriptStringLists(tree, declared.Nested, nested)
			}
		case KindStringList:
			if nested, ok := script[declared.Name].(map[string]any); ok && len(nested) == 0 {
				script[declared.Name] = []string{}
			}
		default:
		}
	}
}

// widenFontFamilyIn turns a bare `font_family` string into the five-element
// mapping (spec 006 §3.2 behavior 14), in place.
func widenFontFamilyIn(values map[string]any) {
	typography, ok := values["typography"].(map[string]any)
	if !ok {
		return
	}
	name, isString := typography["font_family"].(string)
	if !isString {
		return
	}
	widened := make(map[string]any, len(FontFamilyElements))
	for element, value := range WidenFontFamily(name) {
		widened[element] = value
	}
	typography["font_family"] = widened
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

// ScriptConflict is a theme script declaring a shape the design tree cannot
// hold — a group where a value belongs, or the reverse.
type ScriptConflict struct {
	Path     string
	Declared string
	Wanted   string
}

func (c *ScriptConflict) Error() string {
	return "design." + c.Path + " is " + c.Declared + " in this theme's script, but should be " + c.Wanted
}

// ValidateScript checks a theme script's options against the base tree's shapes
// (spec 014 §4, criterion 2).
//
// **Without it a mis-typed option reaches the template and prints a Go type
// name**: a script declaring `page = { size = { a = 1 } }` produced
// `page-size: "<map[string]interface {} Value>"` in the artifact, at exit 0,
// under "Your CV is ready". A fresh-context verifier measured that; it is the
// failure mode this port keeps finding, where wrong output is more expensive
// than no output.
//
// Only options the tree **declares** are checked. An option a script invents is
// the tree's business to carry and `luatheme.Validate`'s to type against the
// document, because the tree has no shape for it.
func ValidateScript(script map[string]any) []error {
	var errs []error
	validateScript(baseTree(), baseTree().Root, script, "", &errs)
	return errs
}

func validateScript(tree Tree, model string, script map[string]any, prefix string, errs *[]error) {
	for _, declared := range tree.Models[model].Fields {
		value, present := script[declared.Name]
		if !present || value == nil {
			continue
		}
		path := declared.Name
		if prefix != "" {
			path = prefix + "." + declared.Name
		}

		nested, isNested := value.(map[string]any)
		if declared.Kind == KindNested {
			if !isNested {
				*errs = append(*errs, &ScriptConflict{
					Path: path, Declared: "a value", Wanted: "a group of options",
				})
				continue
			}
			validateScript(tree, declared.Nested, nested, path, errs)
			continue
		}
		// **`font_family` accepts either shape** (spec 006 §3.1 behavior 12: a
		// bare name, or the five-element mapping) — the same carve-out
		// `withoutTreeConflicts` and `withoutConflicts` already give a
		// *document*'s override. A script declaring the mapping form used to be
		// flagged here as a value-where-a-group-belongs conflict, and
		// `ValidateScript` drops the **whole script** on any conflict, so a
		// theme script setting `typography.font_family` as a table alongside
		// unrelated options lost all of them. Found by a fresh-context verifier
		// (iteration 14's seventh re-verification).
		// **The mapping form is still a `FontFamily`, not an arbitrary table.**
		// A script declaring `typography.font_family = { body = { x = 1 } }`
		// used to be waved through by this `continue` with no check at all,
		// reaching the artifact as a Go type name one field deeper than the
		// carve-out was meant to reach. Found by a fresh-context verifier
		// (iteration 14's eighth re-verification).
		if declared.Kind == KindFontFamily && isNested {
			validateScript(tree, "FontFamily", nested, path, errs)
			continue
		}
		// **`KindStringList` must be checked before the generic map conflict
		// below.** Lua has one table type for both a sequence and a mapping,
		// and an *empty* table (`{}` — literally what `create-theme` writes
		// for every field it doesn't set) has no elements to distinguish the
		// two by, so `luatheme.Options` converts it to an empty Go map rather
		// than an empty `[]string`. The generic `isNested` check below would
		// otherwise flag that as "a group of options" and `themeScript` drops
		// the whole script. An empty map here is the one ambiguous case;
		// nothing else does. Found by a fresh-context verifier (iteration 14's
		// ninth re-verification).
		if declared.Kind == KindStringList {
			switch shapeKind(value) {
			case "list":
			case "map":
				if len(value.(map[string]any)) != 0 {
					*errs = append(*errs, &ScriptConflict{
						Path: path, Declared: "a group of options", Wanted: "a list of items",
					})
				}
			default:
				*errs = append(*errs, &ScriptConflict{
					Path: path, Declared: "a value", Wanted: "a list of items",
				})
			}
			continue
		}
		// **A colour accepts a tuple, not just a string.** `Colors.name = (1,
		// 2, 3)` is legal upstream (`color.py`'s `Color.__init__` takes a
		// tuple or a list directly), so a script declaring one used to be
		// flagged as a value where a group belongs — `isNested` is false for
		// a Lua sequence turned `[]string`, so it actually fell to the
		// list-check below and was rejected there instead, but the effect is
		// the same: the whole script dropped over a legal shape. Found by a
		// fresh-context verifier (iteration 14's thirteenth
		// re-verification).
		if declared.Kind == KindColor && shapeKind(value) == "list" {
			// **The carve-out must still check the tuple, not just its
			// shape.** A shape-only pass let `colors.name = {300, 2, 3}`
			// and `colors.name = {1, 2}` through unchecked, so an invalid
			// tuple reached `normalizeColors`, failed to parse there too,
			// and was left as the raw `[]string` — a Go type name in the
			// artifact, the exact failure this whole family of carve-outs
			// exists to prevent. Found by a fresh-context verifier
			// (iteration 14's fourteenth re-verification).
			if list, ok := value.([]string); ok {
				if _, err := ParseColorTuple(list); err != nil {
					*errs = append(*errs, &ScriptConflict{
						Path: path, Declared: "an invalid color tuple", Wanted: "a valid color",
					})
				}
			}
			continue
		}
		if isNested {
			*errs = append(*errs, &ScriptConflict{
				Path: path, Declared: "a group of options", Wanted: "a value",
			})
			continue
		}
		// **A list where a scalar belongs prints a Go type name the same way a
		// map does** — `page.size = {"a4"}` in Lua is a sequence, which
		// `luatheme.Options` turns into `[]string`, and nothing here checked
		// list shapes at all. Every kind reaching this point other than
		// `KindStringList` (handled above) wants a scalar. Found by a
		// fresh-context verifier (iteration 14's eighth re-verification).
		if shapeKind(value) == "list" {
			*errs = append(*errs, &ScriptConflict{
				Path: path, Declared: "a list of items", Wanted: "a value",
			})
			continue
		}
		// **Shape is not type.** Every check above asks only whether the script
		// declared a map, a list or a scalar where the tree wanted one — never
		// whether the scalar it declared is a value the field can *hold*.
		// Upstream asks: `BaseModelWithoutExtraKeys` sets `validate_default=True`
		// (`schema/models/base.py:5`) and `design.py:135` constructs
		// `theme_data_model_class(**design)`, so a script whose default is
		// `page.size = "bogus"` is exit 1 with a validation panel. Here all of
		// these passed clean and reached the artifact: `page-size: "bogus"`,
		// `colors-name: True` (which kills the Typst engine outright), and
		// `font_family.body = true` typesetting a whole document in a font named
		// "True" at exit 0. Found by a fresh-context verifier (iteration 14's
		// twenty-first re-verification).
		if message := scriptValueMessage(declared, value); message != "" {
			*errs = append(*errs, &ScriptValueError{Path: path, Message: message})
		}
	}
}

// ScriptValueError is a theme script declaring a value the field's own type
// rejects — as opposed to `ScriptConflict`, which is the wrong *shape* at that
// field.
//
// **The message is upstream's own**, produced by the same rules a document's
// value goes through (`validateField`), so the two layers cannot drift into
// disagreeing about what `page.size` accepts. It is not yet printed: like every
// other `ValidateScript` finding, `bridge.themeScript` drops the script and
// falls back to the theme's defaults, because surfacing script diagnostics is
// still the human-gated wording question of spec 014 §2 behavior 9.
type ScriptValueError struct {
	Path    string
	Message string
}

func (e *ScriptValueError) Error() string {
	return "design." + e.Path + " is invalid in this theme's script: " + e.Message
}

// messageStringType is pydantic's `string_type` text. A Lua number or boolean
// at a `str`-typed field is this, not a coercion: pydantic's lax mode still
// refuses an `int` where a `str` is declared.
const messageStringType = "Input should be a valid string"

// scriptValueMessage is what upstream's validator would say about `value` at a
// field of this kind, or "" when the value is one the field accepts. It mirrors
// `validateField` arm for arm — the difference is only the input, a Go value
// out of `luatheme.Options` rather than a YAML node.
func scriptValueMessage(field Field, value any) string {
	switch field.Kind {
	case KindString, KindOptionalString, KindThemeTag:
		if _, ok := value.(string); !ok {
			return messageStringType
		}

	case KindTypstDimension:
		text, ok := value.(string)
		if !ok {
			return messageStringType
		}
		if !ValidTypstDimension(text) {
			return MessageBadTypstDimension
		}

	case KindColor:
		text, ok := value.(string)
		if !ok {
			// A tuple is handled by the caller's carve-out; anything else
			// reaching here is a bool or a number, which `Color` refuses.
			return errColorNotAValue.Error()
		}
		if _, err := ParseColor(text); err != nil {
			return err.Error()
		}

	case KindLiteral:
		text, isText := value.(string)
		// A font slot is an open `str` upstream (`font_family.py:30`), so any
		// name passes and only a non-string fails — the same exemption
		// `validateField` makes.
		if isFontFamilyField(field) {
			if !isText {
				return messageStringType
			}
			return ""
		}
		if !isText || !contains(field.Members, text) {
			return binder.LiteralMessage(field.Members, "Input should be a valid value")
		}

	case KindFontFamily:
		// The mapping form recursed above; a scalar here must be a font name.
		if _, ok := value.(string); !ok {
			return modelTypeMessage("FontFamily")
		}

	case KindBool:
		return scriptBoolMessage(value)

	case KindNested, KindStringList:
		// Shapes, not values — the caller's own checks own both.
	}
	return ""
}

// scriptBoolMessage is `validBoolNode` over a Lua value: a real boolean, one of
// pydantic's word forms, or a number worth `0`/`1`. A Lua number arrives here as
// an `int` whenever it is whole (`luatheme.convert`), so the `int` arm is the
// whole-number arm and a `float64` is by construction fractional.
//
// **A whole number that is not 0 or 1 is `bool_parsing`, the same rule
// `validBoolNode` follows on a document**: pydantic narrows it to an int and
// fails reading it as a bool. Only the fractional case is `bool_type`.
func scriptBoolMessage(value any) string {
	switch typed := value.(type) {
	case bool:
		return ""
	case string:
		if boolWords[strings.ToLower(typed)] {
			return ""
		}
		return errBoolParsing.Error()
	case int:
		if typed == 0 || typed == 1 {
			return ""
		}
		return errBoolParsing.Error()
	}
	return errBoolType.Error()
}
