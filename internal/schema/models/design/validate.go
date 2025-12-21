package design

import (
	"errors"
	"math"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// relativeTo is what a custom theme's folder is resolved against: the input
// file's directory, or the working directory when there is no input file
// (`design.py:55-56`).
//
// **`uncleanedDir`, not `filepath.Dir`.** `Dir` calls `Clean`, which
// collapses every `..` segment — Python's `PurePath.parent` does not, so
// `rendercv render ./bb/../bb/CV.yaml` resolves a theme against
// `./bb/../bb`, not the fully-simplified `bb`. `absoluteLikePython`
// (customtheme.go) only stopped the *next* cleaning step; this one runs
// first and had already thrown the information away. Found by a
// fresh-context verifier (iteration 14's nineteenth re-verification).
func relativeTo(ctx *valctx.ValidationContext) string {
	path, ok := ctx.InputPath()
	if !ok {
		return "."
	}
	return uncleanedDir(path)
}

// uncleanedDir is `PurePath.parent`: `pathlib`'s own parse, which drops
// empty and `.` segments but — unlike `filepath.Clean` — never resolves a
// `..` segment against its neighbor. `Path("./bb/../bb/CV.yaml").parent` is
// `bb/../bb`, not `bb` and not `./bb/../bb`.
func uncleanedDir(path string) string {
	absolute, parts := pathlibParts(path)
	if len(parts) == 0 {
		return "."
	}
	return pathlibJoin(absolute, parts[:len(parts)-1])
}

// pathlibParts is `pathlib`'s POSIX parser: split on `/`, drop empty and
// `.` segments, keep everything else — `..` included — in order.
func pathlibParts(path string) (absolute bool, parts []string) {
	absolute = strings.HasPrefix(path, "/")
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." {
			continue
		}
		parts = append(parts, segment)
	}
	return absolute, parts
}

// pathlibJoin reassembles what `pathlibParts` split, the way `pathlib`
// prints a path: an empty, relative result is `.`, never `""`.
func pathlibJoin(absolute bool, parts []string) string {
	joined := strings.Join(parts, "/")
	if absolute {
		return "/" + joined
	}
	if joined == "" {
		return "."
	}
	return joined
}

// CodeModelAttributesType is a `design` that is not a mapping. It is the only
// block-level failure `design` has — and **not** the pair `locale` has, which is
// the finding below.
const CodeModelAttributesType schemaerr.Code = "model_attributes_type"

const messageNotAMapping = "Input should be a valid dictionary or object to" +
	" extract fields from"

// The bool codes, which differ by what failed to coerce.
const (
	CodeBoolParsing schemaerr.Code = "bool_parsing"
	CodeBoolType    schemaerr.Code = "bool_type"
)

// The four shape failures, as sentinel errors.
//
// They are `var`s rather than `errors.New` at the raise site so the capital
// letters — pydantic's, not the port's — need explaining once. `staticcheck`'s
// ST1005 wants lowercase error strings; lowercasing these would be a parity
// break for a style rule.
//
//nolint:staticcheck // ST1005: upstream's literals, capital and all.
var (
	errBoolParsing = errors.New("Input should be a valid boolean, unable to interpret input")
	errBoolType    = errors.New("Input should be a valid boolean")

	errColorNotAValue   = errors.New("value is not a valid color: value must be a tuple, list or string")
	errColorTupleLength = errors.New("value is not a valid color: tuples must have length 3 or 4")
)

// Validate is the whole `design` block: the discriminator, then the theme's
// option tree.
//
// **The two steps do not both run** (spec 006 §3 behavior 6). `validate_design`
// resolves the theme first and re-raises anything that is not a discriminator
// failure unchanged, so a built-in theme with a bad option reports **that
// option** and not "unknown theme".
//
// A theme name that is not built in is a custom theme, and `ValidateTheme` owns
// the name-shape check that iteration 4 ported, followed by the two folder
// checks of `design.py:72-86`. A custom theme's *options* are its own, so the
// built-in option tree is not applied to it.
func Validate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	ctx *valctx.ValidationContext,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}
	if node.Kind != yamldoc.KindMapping {
		return []schemaerr.ValidationError{
			blockError(node, CodeModelAttributesType, messageNotAMapping, location, source),
		}
	}

	// **A `design` block with no `theme` key crashes upstream.**
	// `validate_design` runs before the discriminated union and does
	// `str(design["theme"])` unguarded (design.py:57), so `{design: {page: …}}`
	// raises `KeyError: 'theme'` — an unhandled exception, not a validation
	// error. `locale`, whose union pydantic resolves itself, gives
	// `union_tag_not_found` for the same shape; the two blocks differ because one
	// has a wrap validator in front of it.
	//
	// Producing a message here would be a divergence: the port would report where
	// upstream crashes. So this returns nothing and the CLI's unhandled-failure
	// handling owns it, which is where iteration 4 sent its two crashes
	// (spec 004 §7.8).
	theme, present := mappingValue(node, "theme")
	if !present {
		// TODO(iteration-12): upstream raises KeyError here; match whatever the
		// CLI prints for an unhandled exception.
		return nil
	}

	if errs := ValidateTheme(theme, location, source); len(errs) > 0 {
		return errs
	}
	if !isBuiltIn(theme) {
		// A custom theme passed the name-shape check, so the two folder checks
		// run next (`design.py:72-86`).
		//
		// **They are validation records, not a user error.** Upstream raises a
		// `PydanticCustomError` from inside the validator, so it arrives in the
		// same table as every other problem, located at `design` — the block, not
		// `design.theme`, because neither of these two passes a `loc` override the
		// way the name check does. The port used to run them from the CLI and
		// print a one-line `Error` panel instead, which `err_unknown_theme`
		// measures.
		if err := ValidateCustomThemeFolder(theme.Raw, relativeTo(ctx)); err != nil {
			return []schemaerr.ValidationError{
				blockError(node, CodeTheme, err.Error(), location, source),
			}
		}
		// **The theme's script is loaded here**, which is where upstream loads
		// it: `validate_design` imports `<theme>/__init__.py` immediately after
		// the two folder checks and instantiates the class it finds with the
		// whole `design` block (`design.py:90-135`). The port read
		// `<theme>/init.lua` only at render time until now, so a custom theme's
		// declared shape did not exist at validation time at all and this
		// function could do nothing but return.
		script := LoadThemeScript(relativeTo(ctx), theme.Raw)
		return validateScriptedTheme(node, script, theme.Raw, location, source)
	}
	return validateModel(node, baseTree(), baseTree().Root, theme.Raw, location, source,
		binder.ForbidExtra)
}

// validateScriptedTheme judges a custom theme's `design` block the way
// upstream's `theme_data_model_class(**design)` (`design.py:135`) does.
//
// **A theme folder with no script judges nothing at all.** Upstream's fallback
// builds `ThemeOptionsAreNotProvided(theme=theme_name)` (`design.py:139-142`)
// *without* the document's block, so nothing in it is ever seen — measured
// against the vendored binary as exit 0 for an unknown key that is exit 1 the
// moment an `__init__.py` appears in the same folder. A script that exists but
// could not be loaded is the same case: there is no declared shape to judge
// against, and reporting the broken script is its own unit.
//
// Which is why `ThemeScript` reports `Exists` separately from its options:
// gating on "are there options" would start judging a script-less theme the day
// someone made the loader return an empty map instead of nil.
func validateScriptedTheme(
	node *yamldoc.Node,
	script ThemeScript,
	theme string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	// **A failing script is reported, and stops the document being judged.**
	// Upstream raises out of `validate_design` before
	// `theme_data_model_class(**design)` is ever constructed
	// (`design.py:107-135`), so the block's own mistakes are never reached: one
	// record, the script's, not two.
	if len(script.Failures) > 0 {
		return script.Failures
	}
	if !script.Exists || script.Options == nil {
		return nil
	}
	tree := baseTree()
	// **The script's class declares the tree's own fields with the tree's own
	// types.** `create-theme` generates it by copying `classic_theme.py` and
	// renaming the class (`create_init_file_for_theme.py:31-41`), so a value
	// the built-in tree rejects is rejected here too — `page.size: bogus`
	// against a scripted theme is upstream exit 1, and the port used to render
	// `page-size: "bogus"` into the artifact at exit 0.
	//
	// `AllowExtra`, because what a script *adds* has no tree field to be
	// judged by; `unknownKeyErrors` is what forbids the keys neither declares.
	// Field errors precede extra-key errors, which is pydantic's own order.
	errs := validateModel(node, tree, tree.Root, theme, location, source, binder.AllowExtra)
	// **What the script *adds* is judged by what the script declared.** The
	// tree has no field for `custom_note`, so `AllowExtra` waves its value
	// through above; upstream's generated class holds the user's fields
	// alongside the copied tree ones and judges both.
	errs = append(errs,
		scriptOptionErrors(node, script.Options, tree, tree.Root, location, source)...)
	errs = append(errs,
		unknownKeyErrors(node, tree, tree.Root, script.Options, theme, location, source)...)
	return scriptedThemeRecords(errs, node, location, source)
}

// unknownKeyErrors rejects a key that neither the design tree nor the theme's
// script declares — upstream's `extra="forbid"` (`base.py:5`), which the
// generated theme class inherits from the same base the built-in tree uses.
//
// **The permitted set is the union of the two**, and that is the whole
// difference between this and the built-in path. A script exists to *add*
// options: `create-theme` writes the class by copying `classic_theme.py` and
// renaming it (`create_init_file_for_theme.py:31-41`), and a user then adds
// fields to it. Measured against the vendored binary — a declared
// `custom_note` is accepted, an undeclared key beside it is exit 1. Running a
// custom theme through the tree alone would reject exactly the options a
// scripted theme exists to declare.
//
// It reuses `binder.Bind` rather than comparing key sets by hand so the record
// — pydantic's code, its message, the key's own coordinates and the rejected
// value — is built by the one function that already builds it everywhere else.
// Only the extra-key errors are taken; a *value* on a known key is a different
// behavior and not this function's.
func unknownKeyErrors(
	node *yamldoc.Node,
	tree Tree,
	model string,
	script map[string]any,
	theme string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil
	}

	fields := make([]binder.Field, 0, len(tree.Models[model].Fields)+len(script))
	declared := make(map[string]bool, len(tree.Models[model].Fields))
	for _, field := range tree.Models[model].Fields {
		fields = append(fields, binder.Field{Name: field.Name, Value: binder.ValueAny})
		declared[field.Name] = true
	}
	for name := range script {
		if !declared[name] {
			fields = append(fields, binder.Field{Name: name, Value: binder.ValueAny})
		}
	}

	result, _ := binder.Bind(
		node,
		binder.Spec{Fields: fields, Policy: binder.ForbidExtra, Model: modelTitle(model, theme)},
		location,
		source,
	)

	// A nested model's own keys are forbidden by the same rule one level down.
	// Pydantic reaches them while validating the field, which is before the
	// enclosing model's extra keys are reported — so the recursion comes first
	// here too.
	var errs []schemaerr.ValidationError
	for _, field := range tree.Models[model].Fields {
		nested := nestedModelOf(field, result)
		if nested == "" {
			continue
		}
		value, _ := result.Value(field.Name)
		errs = append(errs, unknownKeyErrors(value, tree, nested, nestedScript(script, field.Name),
			theme, append(append([]string(nil), location...), field.Name), source)...)
	}
	return append(errs, result.ExtraErrors...)
}

// nestedModelOf names the model a field's mapping value belongs to, or ""
// when the field holds no model the document can put keys in.
//
// **A script-invented key is not descended into *here*.** Whatever a script
// declares beyond the tree has the type the script's own declaration gives it,
// so a mapping written under it is that type's business — and the tree has no
// model whose fields could call a key of it unknown.
//
// That reasoning held for *values* and did not hold for *unknown keys*:
// upstream's `extra="forbid"` reaches inside a script-declared group too, and a
// key the group does not declare is exit 1 there at any depth. `scriptOptions.go`
// answers that case against the script's own table, which is the only thing that
// knows what the group permits.
func nestedModelOf(field Field, result *binder.Result) string {
	value, present := result.Value(field.Name)
	if !present || value == nil || value.Kind != yamldoc.KindMapping {
		return ""
	}
	switch field.Kind {
	case KindNested:
		return field.Nested
	case KindFontFamily:
		// The mapping form is the five-element `FontFamily`; the bare-name form
		// is a scalar and never reaches here (spec 006 §3.1 behavior 12).
		return "FontFamily"
	case KindString, KindOptionalString, KindStringList, KindTypstDimension,
		KindThemeTag, KindBool, KindColor, KindLiteral:
	}
	return ""
}

// nestedScript is the script's declaration for one field, when it declared a
// group there. A script that declared a scalar — or nothing — at that path adds
// no permitted keys below it.
func nestedScript(script map[string]any, name string) map[string]any {
	nested, _ := script[name].(map[string]any)
	return nested
}

// scriptedThemeRecords is upstream's error *reporting* for a scripted theme,
// which is not the reporting a built-in theme gets.
//
// `pydantic_error_handling.py:53-55` drops the second element of every `design`
// error's location, to skip the discriminated union's tag:
//
//	if plain_error["loc"][0] in ["design", "locale"]:
//	    # Skip the second key because it's the discriminator value.
//	    plain_error["loc"] = plain_error["loc"][:1] + plain_error["loc"][2:]
//
// A built-in theme's error carries that tag — `('design', 'classic',
// 'undeclared_key')` — and the strip is right. A scripted theme's error is
// raised inside the wrap validator by the script's own class, carries no tag,
// and the strip eats a real segment instead. So:
//
//   - a failure on a top-level key, `('design', 'undeclared_key')`, becomes
//     `('design',)` and prints there. Wrong-looking, measured, and reproduced:
//     it is what a user of upstream sees.
//   - anything deeper becomes a path that usually does not resolve in the YAML,
//     and upstream's formatter dies on it — `RenderCVInternalError: Key
//     'unknown' not found in the YAML file.`, exit 1 with a traceback and no
//     table. The port cannot print a Python traceback and will not invent a
//     record upstream never shows, so it keeps the true location and prints its
//     own clean row: same exit code, same refusal to render.
//
// **Deduplication is by location alone** (`:167-176`), so once several depth-1
// failures collapse onto `design` only the first survives. Reproducing the
// collapse without the deduplication would print rows upstream does not.
func scriptedThemeRecords(
	errs []schemaerr.ValidationError,
	block *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	span := block.Span
	seen := make(map[string]bool, len(errs))
	out := make([]schemaerr.ValidationError, 0, len(errs))
	for _, err := range errs {
		if len(err.SchemaLocation) == len(location)+1 {
			err.SchemaLocation = append([]string(nil), location...)
			// The coordinates follow the location: upstream looks up
			// `('design',)` and highlights the block, not the key that failed.
			err.YamlLocation = &span
			err.YamlSource = source
		}
		key := strings.Join(err.SchemaLocation, ".")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, err)
	}
	return out
}

// isBuiltIn answers the question pydantic's discriminated union answers, which
// is about the *value* and not only its text.
//
// **A tagged scalar spelling a built-in theme's name is not that theme.**
// `design.theme: !!str classic` is a `TaggedScalar` upstream, which equals no
// string, so the union fails to discriminate and `validate_design` falls
// through to the custom-theme path — where `str(design["theme"])`
// (`design.py:57`) turns it back into `classic`, passes the name-shape check,
// and reports the *folder* message. Comparing text alone matched `classic` here
// and validated the built-in option tree instead.
func isBuiltIn(theme *yamldoc.Node) bool {
	if theme == nil || theme.Kind != yamldoc.KindString {
		return false
	}
	for _, known := range BuiltInThemes {
		if known == theme.Raw {
			return true
		}
	}
	return false
}

// validateModel walks one model of the tree.
//
// One recursive function rather than twenty-two hand-written validators, because
// the tree is data: the alternative is twenty-two chances to forget
// `ForbidExtra` on a level nobody tests.
// The policy travels down the tree because it is a property of the *theme*,
// not of a level: a scripted theme may add an option at any depth, and the
// built-in tree forbids one at every depth.
func validateModel(
	node *yamldoc.Node,
	tree Tree,
	model, theme string,
	location []string,
	source schemaerr.YamlSource,
	policy binder.Policy,
) []schemaerr.ValidationError {
	fields := make([]binder.Field, 0, len(tree.Models[model].Fields))
	for _, field := range tree.Models[model].Fields {
		fields = append(fields, binder.Field{
			Name: field.Name, Value: valueKind(field),
			// **Every design field has a default, so none is `Required`** — a
			// document naming only `theme` inherits the rest. But a default is
			// not the same as `X | None`: only `KindOptionalString`
			// (`degree_column`) is actually typed nullable upstream, so an
			// explicit null anywhere else is a type failure, not an absence.
			// `!Required` alone made `checkValue` treat the two the same,
			// which is exactly backwards for this tree — silently defaulting
			// `colors.name: null` to the *base* tree's colour rather than
			// rejecting it, and to a different colour than the theme's own.
			// Found by a fresh-context verifier (iteration 14's eleventh
			// re-verification).
			TypeRejectsNull: field.Kind != KindOptionalString,
		})
	}

	result, shapeErrs := binder.Bind(
		node,
		binder.Spec{Fields: fields, Policy: policy, Model: modelTitle(model, theme)},
		location,
		source,
	)

	// Group shape errors by field name so they can be interleaved with value
	// errors in declaration order. Errors not tied to a specific field (e.g.
	// a non-mapping input, an invalid key) are emitted first.
	var preErrs []schemaerr.ValidationError
	shapeErrsByField := make(map[string][]schemaerr.ValidationError)
	for _, err := range shapeErrs {
		loc := err.SchemaLocation
		if len(loc) <= len(location) {
			preErrs = append(preErrs, err)
			continue
		}
		fieldName := loc[len(location)]
		shapeErrsByField[fieldName] = append(shapeErrsByField[fieldName], err)
	}

	var errs []schemaerr.ValidationError
	errs = append(errs, preErrs...)
	for _, field := range tree.Models[model].Fields {
		if fieldErrs, hasShapeErr := shapeErrsByField[field.Name]; hasShapeErr {
			errs = append(errs, fieldErrs...)
			continue
		}
		value, present := result.Value(field.Name)
		if !present || value == nil {
			continue
		}
		errs = append(errs, validateField(field, value, tree, theme,
			append(append([]string(nil), location...), field.Name), source, policy)...)
	}

	return append(errs, result.ExtraErrors...)
}

// modelTitle is what the binder names in a shape message. The root reports as
// the theme's class — `Sb2novTheme` — and every nested model as its own, which
// is what the schema's titles say too.
func modelTitle(model, theme string) string {
	if model == "ClassicTheme" {
		return ThemeDefName(theme)
	}
	return model
}

// valueKind tells the binder what shape to expect, so a wrong *type* is its
// error and a wrong *value* is this file's.
func valueKind(field Field) binder.ValueType {
	switch field.Kind {
	case KindString, KindOptionalString, KindTypstDimension, KindThemeTag:
		return binder.ValueString
	case KindStringList:
		return binder.ValueStringList
	case KindNested, KindBool, KindFontFamily, KindColor, KindLiteral:
		// ValueAny, because none of these reports `string_type` for a non-string.
		// Measured: `size: 5` gives `literal_error`, `colors.body: 5` gives
		// `color_error`, `page: x` gives `model_type`. Declaring them
		// ValueString would report the binder's message for all three.
	}
	return binder.ValueAny
}

func validateField(
	field Field,
	node *yamldoc.Node,
	tree Tree,
	theme string,
	location []string,
	source schemaerr.YamlSource,
	policy binder.Policy,
) []schemaerr.ValidationError {
	switch field.Kind {
	case KindNested:
		if node.Kind != yamldoc.KindMapping {
			return one(node, binder.CodeModelType, modelTypeMessage(field.Nested), location, source)
		}
		return validateModel(node, tree, field.Nested, theme, location, source, policy)

	case KindTypstDimension:
		// The binder reported a non-string as `string_type`, which is what
		// upstream gives for `top_margin: 5`.
		if node.Kind == yamldoc.KindString && !ValidTypstDimension(node.Raw) {
			return one(node, CodeTypstDimension, MessageBadTypstDimension, location, source)
		}

	case KindColor:
		if err := validColorNode(node); err != nil {
			return one(node, CodeColor, err.Error(), location, source)
		}

	case KindLiteral:
		// **A font family is not a closed set.** Upstream declares it
		// `SkipJsonSchema[str] | Literal[...]` (`font_family.py:30`), so the
		// literals are a *documentation* arm for the JSON schema and the bare
		// `str` accepts anything — a user's system font renders fine there. This
		// port rejected it, so `theme: opal` with `font_family.body: Charter`
		// exited 1 where upstream exits 0. `fontfamily.go` had the rule written
		// down correctly the whole time; the tree generator emitted the fields as
		// a plain literal and nothing reconciled the two.
		if isFontFamilyField(field) {
			if node.Kind != yamldoc.KindString {
				return one(node, binder.CodeStringType, "Input should be a valid string",
					location, source)
			}
			return nil
		}

		// **Any** non-member is `literal_error`, whatever its shape: a mapping,
		// a sequence and a bool all give the same message as a wrong string.
		// Measured on `page.size`.
		if node.Kind != yamldoc.KindString || !contains(field.Members, node.Raw) {
			return one(node, binder.CodeLiteralError,
				binder.LiteralMessage(field.Members, "Input should be a valid value"),
				location, source)
		}

	case KindBool:
		if err := validBoolNode(node); err != nil {
			return one(node, boolCode(err), err.Error(), location, source)
		}

	case KindFontFamily:
		// Any *name* validates (spec 006 §3.1 behavior 12), and a mapping is the
		// five-element form. Anything else is the model's shape failure.
		switch node.Kind {
		case yamldoc.KindMapping:
			return validateModel(node, tree, "FontFamily", theme, location, source, policy)
		case yamldoc.KindString:
		default:
			return one(node, binder.CodeModelType, modelTypeMessage("FontFamily"), location, source)
		}

	case KindString, KindOptionalString, KindStringList, KindThemeTag:
		// Shape only, which the binder already reported.
	}
	return nil
}

// modelTypeMessage is pydantic's `model_type` text, which **names the model**:
// `page: x` gives `Input should be a valid dictionary or instance of Page`. The
// interpolation is the same one spec 003 §6 found missing in the entry binder.
func modelTypeMessage(model string) string {
	return "Input should be a valid dictionary or instance of " + model
}

// boolTruthy and boolFalsy are the strings pydantic accepts for a bool, matched
// case-insensitively (pydantic-core's `str_as_bool`).
var boolWords = map[string]bool{
	"0": true, "off": true, "f": true, "false": true, "n": true, "no": true,
	"1": true, "on": true, "t": true, "true": true, "y": true, "yes": true,
}

// boolFalsy is boolWords' falsy half — the strings pydantic coerces to `false`.
// Everything else in boolWords is truthy.
var boolFalsy = map[string]bool{
	"0": true, "off": true, "f": true, "false": true, "n": true, "no": true,
}

// validBoolNode is pydantic's bool coercion, which is lax in one direction and
// not the other. Measured:
//
//	show_footer: yes         accepted        — a truthy word
//	show_footer: 1           accepted        — an int, but only 0 or 1
//	show_footer: 1.0         accepted        — a float, but only 0.0 or 1.0
//	show_footer: 0x1         accepted        — any int spelling worth 0 or 1
//	show_footer: "yes please" bool_parsing   — a string it cannot interpret
//	show_footer: 2            bool_parsing   — a whole number that is not 0 or 1
//	show_footer: 1.5          bool_type      — a float that is not whole
//	show_footer: [1]          bool_type      — nor is a collection
//
// **A number is judged by its value, not by its spelling.** pydantic's lax bool
// mode takes any `int` worth 0 or 1 and any `float` that is exactly 0.0 or 1.0,
// so `1.0`, `0.0`, `-0.0`, `1e0`, `0x1`, `0o1`, `0b1` and `+1` all render — the
// coercion is real, `links.underline: 0o0` emitting `links-underline: false,`
// byte for byte. This arm used to reject every float outright and compare an
// int's *text* against `"0"`/`"1"`, which failed all eight. Found by a
// fresh-context verifier (iteration 14's twenty-second re-verification).
//
// `parseNumericText` is the colour tuple's `float()`, reused here for the same
// reason: only a value YAML itself resolved to an int is eligible for the
// hex/octal/binary coercion, so a quoted `"0x1"` — a Python `str` upstream —
// stays a string and fails, as `float("0x1")` does.
func validBoolNode(node *yamldoc.Node) error {
	switch node.Kind {
	case yamldoc.KindBool:
		return nil
	case yamldoc.KindString:
		if boolWords[strings.ToLower(node.Raw)] {
			return nil
		}
		return errBoolParsing
	case yamldoc.KindInt, yamldoc.KindFloat:
		// A `.nan`/`.inf` token does not parse here at all, which is the same
		// answer its value would earn: neither is 0 or 1, and neither is whole.
		value, ok := parseNumericText(node.Raw, node.Kind == yamldoc.KindInt)
		switch {
		case ok && (value == 0 || value == 1):
			return nil
		case ok && isWholeNumber(value):
			// **A number that is whole but not 0 or 1 is `bool_parsing`, not
			// `bool_type`.** pydantic's lax mode narrows an integral float to an
			// int and then fails *reading* it as a bool, which is a different
			// pydantic error from the one a shape it will not even try earns —
			// measured differing on `2`, `-1`, `2.0` and `1_0`. Only a
			// non-integral float (`1.5`, `1e-7`, `.nan`, `.inf`) stays
			// `bool_type`. Found by a fresh-context verifier (iteration 14's
			// twenty-second re-verification).
			return errBoolParsing
		}
		return errBoolType
	case yamldoc.KindNull, yamldoc.KindMapping, yamldoc.KindSequence, yamldoc.KindTagged:
		// A collection is never coerced, and neither is a TaggedScalar: it is
		// an object, so pydantic will not even try to read a bool out of it.
	}
	return errBoolType
}

// isWholeNumber reports whether pydantic would treat a number as an integer
// when deciding *which* boolean error to raise.
//
// It is `float.is_integer()` — false for a NaN, an infinity and any fractional
// value — **and additionally bounded by the range of an `i64`**, because
// pydantic-core converts through one: a float too large to be an integer there
// is `bool_type` rather than `bool_parsing`. Measured against pydantic on both
// sides of the boundary — `9.2e18` is `bool_parsing`, `9.3e18` is `bool_type` —
// which is also why `1e308` and `inf` take the type error.
func isWholeNumber(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return false
	}
	return value >= math.MinInt64 && value <= math.MaxInt64
}

// boolCode reads the code off the error `validBoolNode` returned rather than off
// the node's shape: a number carries either code depending on its *value*, so
// the shape alone can no longer tell them apart.
func boolCode(err error) schemaerr.Code {
	if errors.Is(err, errBoolParsing) {
		return CodeBoolParsing
	}
	return CodeBoolType
}

// validColorNode covers the shapes `Color` accepts beyond a string: a sequence
// of three or four numbers is a tuple, which is why `[1, 2, 3]` validates and
// `[1, 2]` does not.
func validColorNode(node *yamldoc.Node) error {
	switch node.Kind {
	case yamldoc.KindString:
		_, err := ParseColor(node.Raw)
		return err
	case yamldoc.KindSequence:
		// **Only the tuple's length was ever checked.** `[300, 0, 0]`,
		// `[-1, 0, 0]` and `[a, b, c]` all validated as long as they had
		// three or four elements, where upstream's `parse_color_value`
		// range- and type-checks each one — measured against `theme:
		// sb2nov` with each of those three shapes. Found by a fresh-context
		// verifier (iteration 14's twelfth re-verification).
		elements := make([]colorElement, 0, len(node.Elems))
		for i, elem := range node.Elems {
			if elem == nil {
				return errColorNotAValue
			}
			// **A null alpha is not an error, it is no alpha at all.**
			// `parse_float_alpha(None)` returns `None` rather than raising
			// (`color.py:394-407`'s Python original), so `[1, 2, 3, null]`
			// is exactly `[1, 2, 3]` — the same case `stringsOf` already
			// produces by dropping a null element outright once the tuple
			// reaches the merge layer. Only the fourth (alpha) position
			// gets this exemption; a null red, green or blue is still
			// `parse_color_value(None)`, a real failure. Found by a
			// fresh-context verifier (iteration 14's thirteenth
			// re-verification).
			if i == 3 && len(node.Elems) == 4 && elem.Kind == yamldoc.KindNull {
				continue
			}
			// **Only a `KindBool` or `KindInt` element is eligible for the
			// bool-word/hex/octal/binary coercion.** A `KindString` element
			// — `colors.name: ["0x10", 0, 0]` — hands upstream's
			// `float()` a Python `str`, which raises on that spelling
			// exactly as it would on any other non-numeric text; only an
			// *unquoted* `0x10` YAML itself resolves to an `int` gets the
			// coercion. Found by a fresh-context verifier (iteration 14's
			// sixteenth re-verification).
			coerce := elem.Kind == yamldoc.KindBool || elem.Kind == yamldoc.KindInt
			// **A sequence or a mapping element carries no `Raw` at all.**
			// `yamldoc.Node.Raw` is scalar-only, so `[1, 2, 3, [1]]`
			// reached `parseAlpha("")` — which is "no alpha", not "a bad
			// alpha" — and the tuple validated as three channels and
			// rendered a *different colour* than the document wrote, at
			// exit 0. The flag travels with the element rather than
			// failing here so the tuple's length is still checked first,
			// the order `parse_tuple` uses. Found by a fresh-context
			// verifier (iteration 15's colour-tuple sweep).
			nonScalar := elem.Kind == yamldoc.KindSequence || elem.Kind == yamldoc.KindMapping
			elements = append(elements, colorElement{
				Raw:         elem.Raw,
				Coerce:      coerce,
				IsPythonInt: elem.Kind == yamldoc.KindInt,
				NonScalar:   nonScalar,
			})
		}
		_, err := parseColorElements(elements)
		return err
	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt, yamldoc.KindFloat,
		yamldoc.KindMapping, yamldoc.KindTagged:
		// Fall through to the shape message below. A TaggedScalar belongs here
		// and not with the string arm: `parse_color_value` runs on the object,
		// not on its `str()`.
	}
	// An int, a float, a bool or a mapping. Not `string_type`: the colour type
	// owns the message, and dictionary row 13 rewrites it like any other.
	return errColorNotAValue
}

// SnakeCaseSectionTitles is `convert_section_titles_to_snake_case`
// (classic_theme.py:493-500): each entry of `sections.show_time_spans_in`
// lowercased with spaces replaced by underscores.
//
// **A coercion, not a validation** (spec 006 §3.2 behavior 15). It raises
// nothing, and what it produces is what the renderer matches section titles
// against — so `["Work Experience"]` has to become `["work_experience"]` or the
// time spans silently do not appear.
func SnakeCaseSectionTitles(titles []string) []string {
	out := make([]string, 0, len(titles))
	for _, title := range titles {
		out = append(out, strings.ReplaceAll(strings.ToLower(title), " ", "_"))
	}
	return out
}

// isFontFamilyField reports whether a literal field is one of the five font
// slots, which are the only literals upstream leaves open.
func isFontFamilyField(field Field) bool {
	return strings.HasSuffix(field.Ref, "font_family__FontFamily")
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func one(
	node *yamldoc.Node,
	code schemaerr.Code,
	message string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	return []schemaerr.ValidationError{blockError(node, code, message, location, source)}
}

func blockError(
	node *yamldoc.Node,
	code schemaerr.Code,
	message string,
	location []string,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	span := node.Span
	return schemaerr.ValidationError{
		Code:           code,
		SchemaLocation: append([]string(nil), location...),
		YamlLocation:   &span,
		YamlSource:     source,
		Message:        message,
		Input:          schemaerr.RenderInput(node),
	}
}

func mappingValue(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}
