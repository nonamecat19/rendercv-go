package design

import (
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// CodeModelAttributesType is a `design` that is not a mapping. It is the only
// block-level failure `design` has — and **not** the pair `locale` has, which is
// the finding below.
const CodeModelAttributesType schemaerr.Code = "model_attributes_type"

const messageNotAMapping = "Input should be a valid dictionary or object to" +
	" extract fields from"

// Validate is the whole `design` block: the discriminator, then the theme's
// option tree.
//
// **The two steps do not both run** (spec 006 §3 behavior 6). `validate_design`
// resolves the theme first and re-raises anything that is not a discriminator
// failure unchanged, so a built-in theme with a bad option reports **that
// option** and not "unknown theme".
//
// A theme name that is not built in is a custom theme, and `ValidateTheme` owns
// the name-shape check that iteration 4 ported. The folder checks that would
// follow it are Wave E's, so a well-named custom theme currently validates
// without its options being checked at all — the option tree describes
// `ClassicTheme`, and a custom theme's is its own.
func Validate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
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
	if !isBuiltIn(theme.Raw) {
		// A custom theme passed the name-shape check. Its options are its own
		// and Wave E owns loading them.
		return nil
	}
	return validateModel(node, baseTree(), baseTree().Root, theme.Raw, location, source)
}

func isBuiltIn(theme string) bool {
	for _, known := range BuiltInThemes {
		if known == theme {
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
func validateModel(
	node *yamldoc.Node,
	tree Tree,
	model, theme string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	fields := make([]binder.Field, 0, len(tree.Models[model].Fields))
	for _, field := range tree.Models[model].Fields {
		fields = append(fields, binder.Field{Name: field.Name, Value: valueKind(field)})
	}

	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fields, Policy: binder.ForbidExtra, Model: modelTitle(model, theme)},
		location,
		source,
	)

	for _, field := range tree.Models[model].Fields {
		value, present := result.Value(field.Name)
		if !present || value == nil || value.Kind == yamldoc.KindNull {
			continue
		}
		errs = append(errs, validateField(field, value, tree, theme,
			append(append([]string(nil), location...), field.Name), source)...)
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
	case KindString, KindOptionalString, KindTypstDimension, KindColor,
		KindLiteral, KindThemeTag:
		return binder.ValueString
	case KindStringList:
		return binder.ValueStringList
	case KindNested, KindBool, KindFontFamily:
		// ValueAny: the binder has no bool shape, and a nested model's or a font
		// family's shape failure belongs to the walk below rather than to the
		// binder's single-shape check.
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
) []schemaerr.ValidationError {
	switch field.Kind {
	case KindNested:
		return validateModel(node, tree, field.Nested, theme, location, source)

	case KindTypstDimension:
		if node.Kind == yamldoc.KindString && !ValidTypstDimension(node.Raw) {
			return one(node, CodeTypstDimension, MessageBadTypstDimension, location, source)
		}

	case KindColor:
		if node.Kind == yamldoc.KindString {
			if _, err := ParseColor(node.Raw); err != nil {
				return one(node, CodeColor, err.Error(), location, source)
			}
		}

	case KindLiteral:
		if node.Kind == yamldoc.KindString && !contains(field.Members, node.Raw) {
			return one(node, binder.CodeLiteralError,
				binder.LiteralMessage(field.Members, "Input should be a valid value"),
				location, source)
		}

	case KindFontFamily:
		// Any name validates, and a mapping is the five-element form
		// (spec 006 §3.1 behavior 12). Only the mapping's *keys* can be wrong.
		if node.Kind == yamldoc.KindMapping {
			return validateModel(node, tree, "FontFamily", theme, location, source)
		}

	case KindString, KindOptionalString, KindStringList, KindBool, KindThemeTag:
		// Shape only, which the binder already reported.
	}
	return nil
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
