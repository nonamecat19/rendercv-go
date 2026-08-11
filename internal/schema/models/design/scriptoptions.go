package design

import (
	"strings"
	"unicode"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// The pydantic error codes for the four scalar types a theme script can
// declare that the design tree never does. `int_type` and `string_type` come
// from the binder; these are the ones no built-in field produces.
const (
	CodeIntParsing   schemaerr.Code = "int_parsing"
	CodeIntFromFloat schemaerr.Code = "int_from_float"
	CodeFloatType    schemaerr.Code = "float_type"
	CodeFloatParsing schemaerr.Code = "float_parsing"
)

// scriptOptionErrors validates the document's values for the options a theme
// **script** declares and the design tree does not.
//
// Upstream's generated theme class holds both sets of fields — the tree's,
// copied from `classic_theme.py`, and whatever the user added — and
// `theme_data_model_class(**design)` (`design.py:135`) judges every one of
// them. `validateModel` covers the tree's half; this is the other half, and
// without it a script-invented option accepted any value at all: measured, all
// fifteen rejecting vectors rendered at exit 0.
//
// **The declared default is the type.** A Lua declaration carries no
// annotation but always carries a value, so `custom_note = "hello"` says
// "string" the way `custom_note: str` does — D-002's substitution, and the same
// rule `luatheme.Validate` already uses for shapes.
//
// **Coercion is pydantic's lax mode and is reproduced deliberately.** A string
// of digits is a valid `int`, a bool word is a valid `bool`, an `int` is a
// valid `float`, and a `bool` is a valid `int` because Python's `bool` is a
// subclass of `int` — each measured against the vendored binary at exit 0. A
// check that merely compared Go types would reject all four and break parity in
// the opposite direction from the bug it fixes.
func scriptOptionErrors(
	node *yamldoc.Node,
	script map[string]any,
	tree Tree,
	model string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind != yamldoc.KindMapping || len(script) == 0 {
		return nil
	}

	var errs []schemaerr.ValidationError
	for _, item := range node.Items {
		// A key the tree declares is `validateModel`'s, whatever the script
		// says about it: upstream's class has one field per name, and the
		// generated copy of the tree's field is the one that wins.
		if _, declaredByTree := treeField(tree, model, item.Key); declaredByTree {
			continue
		}
		declared, declaredByScript := script[item.Key]
		if !declaredByScript || declared == nil || item.Value == nil {
			continue
		}
		errs = append(errs,
			scriptOptionValueErrors(declared, item.Value, item.Key,
				append(append([]string(nil), location...), item.Key), source)...)
	}
	return errs
}

// scriptOptionValueErrors is the per-value check, dispatched on the shape of
// the script's declared default.
//
// **A nested group recurses**, which reaches a depth upstream cannot report: a
// failure at `('design', 'custom_group', 'x')` survives the location strip as
// `('design', 'x')`, which does not resolve in the document, and upstream's
// formatter dies with `RenderCVInternalError: Key 'x' not found in the YAML
// file` — a traceback, exit 1, measured. The port matches the exit code and the
// refusal to render and prints a clean record instead, which is D-011's
// substitution and not a new divergence.
func scriptOptionValueErrors(
	declared any,
	node *yamldoc.Node,
	key string,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	switch typed := declared.(type) {
	case string:
		if node.Kind != yamldoc.KindString {
			return one(node, binder.CodeStringType, "Input should be a valid string", location, source)
		}

	case bool:
		// The tree's own bool rule, reused rather than restated: bool words,
		// `0`/`1`, `bool_parsing` for a whole number that is neither, and
		// `bool_type` for a fractional one. Measured identical for a
		// script-declared bool — `7` and `1.5` give the two different messages.
		if err := validBoolNode(node); err != nil {
			return one(node, boolCode(err), err.Error(), location, source)
		}

	case int:
		return integerErrors(node, location, source)

	case float64:
		return numberErrors(node, location, source)

	case []string:
		if node.Kind != yamldoc.KindSequence {
			// `error_dictionary.yaml:12` rewrites this to "This field should
			// contain a list of items but it doesn't."
			return one(node, binder.CodeListType, "Input should be a valid list", location, source)
		}

	case map[string]any:
		if node.Kind != yamldoc.KindMapping {
			return one(node, binder.CodeModelType, modelTypeMessage(scriptModelName(key)), location, source)
		}
		var errs []schemaerr.ValidationError
		for _, item := range node.Items {
			nested, ok := typed[item.Key]
			if !ok || nested == nil || item.Value == nil {
				continue
			}
			errs = append(errs, scriptOptionValueErrors(nested, item.Value, item.Key,
				append(append([]string(nil), location...), item.Key), source)...)
		}
		return errs
	}
	return nil
}

// integerErrors is pydantic's lax `int`.
//
// A bool is an int in Python, and a string of digits parses, so both are
// accepted — measured at exit 0. A float is accepted only when it is whole;
// `5.5` is `int_from_float`, whose message names the fractional part, while a
// mapping or a sequence never reaches parsing at all and is `int_type`.
func integerErrors(
	node *yamldoc.Node, location []string, source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	switch node.Kind {
	case yamldoc.KindBool, yamldoc.KindInt:
		return nil
	case yamldoc.KindFloat:
		if value, ok := parseNumericText(node.Raw, false); ok && isWholeNumber(value) {
			return nil
		}
		return one(node, CodeIntFromFloat,
			"Input should be a valid integer, got a number with a fractional part", location, source)
	case yamldoc.KindString:
		value, ok := parseNumericText(node.Raw, false)
		if ok && isWholeNumber(value) {
			return nil
		}
		// `error_dictionary.yaml:2` rewrites this one to the *date* message.
		// The dictionary is keyed on message text with no notion of the field,
		// so a custom theme's integer option tells the user to write
		// `YYYY-MM-DD`. Upstream's nonsense, reproduced because axis 4 is the
		// text and not the sense.
		return one(node, CodeIntParsing,
			"Input should be a valid integer, unable to parse string as an integer", location, source)
	case yamldoc.KindNull, yamldoc.KindMapping, yamldoc.KindSequence, yamldoc.KindTagged:
		// A shape pydantic will not try to read as a number at all. A tagged
		// scalar joins them rather than being resolved through its tag, which
		// is D-012's territory and not this unit's to widen.
	}
	return one(node, binder.CodeIntType, "Input should be a valid integer", location, source)
}

// numberErrors is pydantic's lax `float`, which differs from `int` only in
// accepting a fractional value and in its two message texts.
func numberErrors(
	node *yamldoc.Node, location []string, source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	switch node.Kind {
	case yamldoc.KindBool, yamldoc.KindInt, yamldoc.KindFloat:
		return nil
	case yamldoc.KindString:
		if _, ok := parseNumericText(node.Raw, false); ok {
			return nil
		}
		return one(node, CodeFloatParsing,
			"Input should be a valid number, unable to parse string as a number", location, source)
	case yamldoc.KindNull, yamldoc.KindMapping, yamldoc.KindSequence, yamldoc.KindTagged:
		// As in `integerErrors`: a shape, not a number.
	}
	return one(node, CodeFloatType, "Input should be a valid number", location, source)
}

// scriptModelName is the name a script-declared group carries in pydantic's
// `model_type` message.
//
// **Upstream names the user's Python class and a Lua table has no name**, so
// this derives one from the key the way upstream's own `create-theme` names the
// classes it generates — `page` → `Page`, `custom_group` → `CustomGroup`. A
// user who hand-writes `__init__.py` and names the class something else gets
// that name upstream and this one here; the message is the only part that
// differs, and only for a group.
func scriptModelName(key string) string {
	var out strings.Builder
	for _, word := range strings.Split(key, "_") {
		if word == "" {
			continue
		}
		runes := []rune(word)
		out.WriteRune(unicode.ToUpper(runes[0]))
		out.WriteString(string(runes[1:]))
	}
	return out.String()
}

// treeField finds a field by name in a tree model, which is how a key the tree
// already owns is left to `validateModel`.
func treeField(tree Tree, model, name string) (Field, bool) {
	for _, field := range tree.Models[model].Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}
