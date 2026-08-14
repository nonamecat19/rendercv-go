// Package settings is a deliberately minimal slice of upstream's `settings`
// model.
//
// **It holds only what spec 004 §7.9 pulls forward**: a `current_date` shape
// check, so the pipeline's §3.5 override has something to fire on and the
// 25-record differential can reach 25. Every other settings field is
// iteration 7's.
package settings

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// isoDatePattern is the `datetime.date` arm of `current_date`'s union, which
// pydantic accepts in `YYYY-MM-DD` form.
var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// CodeCurrentDate is the code a bad `current_date` carries. It is pydantic's
// union failure, and the message is irrelevant: the pipeline's step 6 override
// replaces whatever arrives (spec 004 §3.5 behavior 18).
const CodeCurrentDate schemaerr.Code = "value_error"

// ValidateCurrentDate checks `settings.current_date`, declared
// `datetime.date | Literal["today"]`.
//
// The message it emits is deliberately **not** spec 004 §4.13. Upstream's two
// union branches produce two different messages and the pipeline overrides both
// on the strength of the location alone, so a validator that pre-substituted
// §4.13 would take the message out of the override's reach and hide whether the
// override runs at all. What matters here is the location.
func ValidateCurrentDate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}

	value := node.Raw
	if value == "today" || isoDatePattern.MatchString(value) {
		return nil
	}

	span := node.Span
	return []schemaerr.ValidationError{{
		Code:           CodeCurrentDate,
		SchemaLocation: append(append([]string(nil), location...), "current_date"),
		YamlLocation:   &span,
		YamlSource:     source,
		Message:        "Input should be a valid date or 'today'",
		Input:          schemaerr.RenderInput(node),
	}}
}

// The two pydantic codes `bold_keywords` can raise, repeated here for the same
// reason `CodeExtraForbidden` is: the binder owns the canonical spellings and
// this package stays out of its import graph.
const (
	// CodeListType is pydantic's `list_type`, raised when the field's value is
	// not a sequence at all.
	CodeListType schemaerr.Code = "list_type"
	// CodeStringType is pydantic's `string_type`, raised per element that is
	// not a `str`.
	CodeStringType schemaerr.Code = "string_type"
)

const (
	// messageListType is rewritten by `error_dictionary.yaml:13` to "This field
	// should contain a list of items but it doesn't."
	messageListType = "Input should be a valid list"
	// messageStringType is rewritten by no dictionary row; the pipeline only
	// adds its trailing period, giving "Input should be a valid string."
	messageStringType = "Input should be a valid string"
)

// ValidateBoldKeywords checks `settings.bold_keywords`, declared `list[str]`
// (settings.py:27).
//
// **A Python `str` is iterable, so the interesting question is whether pydantic
// spreads `bold_keywords: 'A'` into `['A']`. It does not.** Pydantic v2's lax
// mode excludes `str` and `bytes` from the sequence coercions it allows
// precisely because splitting a string into characters is never what a user
// meant, so a bare string raises `list_type` like an int or a mapping does —
// measured against the vendored binary, which refuses the document at exit 1
// while the port rendered it at exit 0.
//
// Element failures carry the element's index as a decimal location component,
// which is the shape the binder's `ValueStringList` arm already produces and
// which upstream prints as `settings.bold_keywords.0`. A null is not a list and
// not a string, so it fails at whichever of the two levels it appears on.
func ValidateBoldKeywords(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}

	field := append(append([]string(nil), location...), "bold_keywords")
	if node.Kind != yamldoc.KindSequence {
		return []schemaerr.ValidationError{
			valueError(CodeListType, messageListType, field, node, source),
		}
	}

	var errs []schemaerr.ValidationError
	for index, element := range node.Elems {
		if element != nil && element.Kind == yamldoc.KindString {
			continue
		}
		at := append(append([]string(nil), field...), strconv.Itoa(index))
		errs = append(errs, valueError(CodeStringType, messageStringType, at, element, source))
	}
	return errs
}

// CodeStringTypePDFTitle is pydantic's `string_type`, raised when
// `settings.pdf_title` is not a string (spec 006 delta §3.2, mechanism B).
//
// It is a separate constant from CodeStringType even though the two share a
// value: `bold_keywords` reports it per element, `pdf_title` reports it at the
// field itself, and giving each mechanism its own name keeps a future reader
// from assuming one call site's fixture also covers the other.
const CodeStringTypePDFTitle = CodeStringType

// ValidatePDFTitle checks `settings.pdf_title`, declared `str`
// (settings.py:32). No coercion in either direction: `"42"` passes, `42` does
// not (spec 006 delta §3.2).
func ValidatePDFTitle(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindString {
		return nil
	}
	field := append(append([]string(nil), location...), "pdf_title")
	return []schemaerr.ValidationError{
		valueError(CodeStringTypePDFTitle, messageStringType, field, node, source),
	}
}

// CodeModelType and messageModelType are mechanism F's — `settings` and
// `settings.render_command` must each be a mapping (spec 006 delta §3.6, §7).
// Upstream's CLI crashes on the way to these records (`render_command.py:205`);
// the port reaches them because it validates before it resolves overlay files
// (behavior 7.5).
const CodeModelType schemaerr.Code = "model_type"

const messageModelType = "Input should be a valid dictionary"

func modelTypeMessage(model string) string {
	return messageModelType + " or instance of " + model
}

// Validate is the whole `settings` block: mechanism F1 (the block itself must
// be a mapping), then every nested field in declaration order (spec 006 delta
// §3, §6.1). An absent `settings` is legal — every field defaults
// (settings.py:11-51) — so a nil node produces no errors.
func Validate(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}
	if node.Kind != yamldoc.KindMapping {
		// Mechanism F1: upstream's CLI crashes on a non-mapping `settings`
		// before validation runs (including `null` — spec 006 delta §3.6);
		// the port validates before it resolves overlay files (behavior 7.5)
		// and so reaches the record upstream's own model would have produced.
		return []schemaerr.ValidationError{
			valueError(CodeModelType, modelTypeMessage("Settings"), location, node, source),
		}
	}

	var errs []schemaerr.ValidationError
	// Declaration order (settings.py:11-51): current_date, render_command,
	// bold_keywords, pdf_title.
	if current, ok := mappingValue(node, "current_date"); ok {
		errs = append(errs, ValidateCurrentDate(current, location, source)...)
	}
	if renderCommand, ok := mappingValue(node, "render_command"); ok {
		errs = append(errs, ValidateRenderCommand(
			renderCommand, append(append([]string(nil), location...), "render_command"), source)...)
	}
	// `bold_keywords` is `list[str]`, and a wrong-typed value for it used to
	// render at exit 0 where upstream refuses the document. It sits after
	// `current_date` because pydantic reports in declaration order
	// (settings.py:11-31).
	if keywords, ok := mappingValue(node, "bold_keywords"); ok {
		errs = append(errs, ValidateBoldKeywords(keywords, location, source)...)
	}
	if pdfTitle, ok := mappingValue(node, "pdf_title"); ok {
		errs = append(errs, ValidatePDFTitle(pdfTitle, location, source)...)
	}
	// **Unknown keys under `settings` were accepted by nobody's decision**:
	// two specs each recorded the settings model as the other's work. An
	// audit measured `settings: {bogus: 1}` rendering at exit 0 where
	// upstream reports an unknown key.
	errs = append(errs, ValidateUnknownKeys(node, location, source)...)
	return errs
}

// nullablePathFields: design/locale admit an explicit null (§2.2, §5.2). The
// other six path fields (output_folder, typst_path, pdf_path, markdown_path,
// html_path, png_path) reject it.
var nullablePathFields = []string{"design", "locale"}

const messagePathType = "Input is not a valid path for <class 'pathlib.Path'>"

// CodePathType is pydantic-core's `path_type`.
const CodePathType schemaerr.Code = "path_type"

// ValidateRenderCommand checks `settings.render_command`'s thirteen fields:
// mechanism F2 (the block itself must be a mapping), mechanism C (the eight
// path fields) and mechanism D (the five `dont_generate_*` booleans).
//
// Errors are appended in `RenderCommandFieldNames`' declaration order
// (spec 006 delta §6.1), which is why this walks the field list rather than
// the document's `Items`.
func ValidateRenderCommand(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil {
		return nil
	}
	if node.Kind != yamldoc.KindMapping {
		// Mechanism F2 (spec 006 delta §3.6): `null` refuses here too, same as
		// every other non-mapping kind — it is not exempted the way an *absent*
		// key is.
		return []schemaerr.ValidationError{
			valueError(CodeModelType, modelTypeMessage("RenderCommand"), location, node, source),
		}
	}

	var errs []schemaerr.ValidationError
	for _, name := range RenderCommandFieldNames {
		value, present := mappingValue(node, name)
		if !present {
			continue
		}
		field := append(append([]string(nil), location...), name)
		if isDontGenerateField(name) {
			if err, bad := validateBoolField(value, field, source); bad {
				errs = append(errs, err)
			}
			continue
		}
		if err, bad := validatePathField(name, value, field, source); bad {
			errs = append(errs, err)
		}
	}
	return errs
}

func isDontGenerateField(name string) bool {
	return strings.HasPrefix(name, "dont_generate_")
}

// validatePathField is mechanism C. A string of any content is legal
// (`""` included, §5.1); every other kind is `path_type`, except that
// `design`/`locale` additionally admit `null` (§2.2).
func validatePathField(
	name string, node *yamldoc.Node, field []string, source schemaerr.YamlSource,
) (schemaerr.ValidationError, bool) {
	if node == nil || node.Kind == yamldoc.KindString {
		return schemaerr.ValidationError{}, false
	}
	if node.Kind == yamldoc.KindNull && isNullablePathField(name) {
		return schemaerr.ValidationError{}, false
	}
	return valueError(CodePathType, messagePathType, field, node, source), true
}

func isNullablePathField(name string) bool {
	for _, candidate := range nullablePathFields {
		if candidate == name {
			return true
		}
	}
	return false
}

// The pydantic codes and messages mechanism D can raise, split by *kind* of
// bad input (spec 006 delta §3.4) — not a single "invalid boolean" rule.
const (
	CodeBoolParsing schemaerr.Code = "bool_parsing"
	CodeBoolType    schemaerr.Code = "bool_type"

	messageBoolParsing = "Input should be a valid boolean, unable to interpret input"
	messageBoolType    = "Input should be a valid boolean"
)

// validateBoolField is mechanism D's refuse side. The accept side —
// `ParseLaxBool` — is shared with `resolveRenderCommand` so a value that
// validates at exit 0 is the same value that takes effect.
func validateBoolField(
	node *yamldoc.Node, field []string, source schemaerr.YamlSource,
) (schemaerr.ValidationError, bool) {
	if node == nil {
		return schemaerr.ValidationError{}, false
	}
	if _, ok, parsing := ParseLaxBool(node); !ok {
		if parsing {
			return valueError(CodeBoolParsing, messageBoolParsing, field, node, source), true
		}
		return valueError(CodeBoolType, messageBoolType, field, node, source), true
	}
	return schemaerr.ValidationError{}, false
}

// ParseLaxBool is pydantic-core's lax boolean coercion (spec 006 delta §3.4,
// measured — not read off a type annotation, because the accepted set and the
// two refusal messages are not derivable from `bool` alone).
//
// `ok` reports whether the value coerces at all; when it does, `value` is the
// coerced bool. When it does not, `parsing` distinguishes the two refusal
// messages: true for a string or an int outside the accepted set
// (`bool_parsing`, "unable to interpret input"), false for everything else — a
// float that is not exactly 0 or 1, null, a list, a mapping
// (`bool_type`).
func ParseLaxBool(node *yamldoc.Node) (value, ok, parsing bool) {
	if node == nil {
		return false, false, false
	}
	switch node.Kind {
	case yamldoc.KindBool:
		return yamldoc.BoolIsTrue(node.Raw), true, false
	case yamldoc.KindInt:
		n, err := strconv.ParseInt(node.Raw, 10, 64)
		if err != nil {
			return false, false, true
		}
		switch n {
		case 0:
			return false, true, false
		case 1:
			return true, true, false
		default:
			return false, false, true
		}
	case yamldoc.KindFloat:
		f, err := strconv.ParseFloat(node.Raw, 64)
		if err != nil {
			return false, false, false
		}
		switch f {
		case 0:
			return false, true, false
		case 1:
			return true, true, false
		default:
			return false, false, false
		}
	case yamldoc.KindString:
		switch strings.ToLower(node.Raw) {
		case "true", "t", "y", "yes", "on", "1":
			return true, true, false
		case "false", "f", "n", "no", "off", "0":
			return false, true, false
		default:
			return false, false, true
		}
	case yamldoc.KindNull, yamldoc.KindMapping, yamldoc.KindSequence, yamldoc.KindTagged:
		return false, false, false
	}
	return false, false, false
}

// valueError builds one record, tolerating a nil node the way the binder's own
// does: a missing node still has a location, and its Input column is `None`.
func valueError(
	code schemaerr.Code,
	message string,
	location []string,
	node *yamldoc.Node,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	err := schemaerr.ValidationError{
		Code:           code,
		SchemaLocation: location,
		YamlSource:     source,
		Message:        message,
		Input:          schemaerr.RenderInput(node),
	}
	if node != nil {
		span := node.Span
		err.YamlLocation = &span
	}
	return err
}

// FieldNames is `Settings`' declared field set (settings.py:10-52). The
// underscore-prefixed `_resolved_current_date` is a private attribute, not a
// field, so it is not here — and a document writing it is an unknown key like
// any other.
var FieldNames = []string{"current_date", "render_command", "bold_keywords", "pdf_title"}

// RenderCommandFieldNames is `RenderCommand`'s (render_command.py).
var RenderCommandFieldNames = []string{
	"output_folder", "design", "locale",
	"typst_path", "pdf_path", "markdown_path", "html_path", "png_path",
	"dont_generate_markdown", "dont_generate_html", "dont_generate_typst",
	"dont_generate_pdf", "dont_generate_png",
}

// ValidateUnknownKeys rejects keys neither model declares
// (`BaseModelWithoutExtraKeys`).
//
// **Nobody owned this until an audit found it.** `STATE.md` deferred the settings
// model to iteration 12; `specs/012-cli/spec.md` recorded the validation text as
// iteration 4's and already done. Two specs each named the other, the ledger
// agreed with both, and `settings: {bogus: 1}` rendered happily where upstream
// reports an unknown key.
//
// The message is the binder's own, which the error pipeline already maps to
// dictionary row `extra_forbidden` — so this adds a location, not a new string.
func ValidateUnknownKeys(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil
	}

	var errs []schemaerr.ValidationError
	for _, item := range node.Items {
		if !known(FieldNames, item.Key) {
			errs = append(errs, unknownKey(item, location))
			continue
		}
		if item.Key != "render_command" || item.Value == nil ||
			item.Value.Kind != yamldoc.KindMapping {
			continue
		}
		nested := append(append([]string(nil), location...), "render_command")
		for _, inner := range item.Value.Items {
			if !known(RenderCommandFieldNames, inner.Key) {
				errs = append(errs, unknownKey(inner, nested))
			}
		}
	}
	return errs
}

func unknownKey(item yamldoc.Item, location []string) schemaerr.ValidationError {
	span := item.KeySpan
	return schemaerr.ValidationError{
		Code:           CodeExtraForbidden,
		SchemaLocation: append(append([]string(nil), location...), item.Key),
		YamlLocation:   &span,
		YamlSource:     source(item),
		Message:        messageExtraForbidden,
		// **The Input Value column was blank on every settings unknown key.**
		// The record left `Input` at its zero value, and a zero `Input` is an
		// empty cell, not an absent one — so the column rendered nothing where
		// upstream prints the offending value. Pydantic's `input` for
		// `extra_forbidden` is the *value* of the unknown key, not the model
		// that rejected it (verified against pydantic directly, on all nine
		// value shapes), and `pydantic_error_handling.py:122-126` renders it
		// `str(value)` unless it is a `dict` or a `list`, in which case `...`.
		// That is exactly `RenderInput`, which the binder's own
		// `extra_forbidden` on `cv` and `design` already used — which is why
		// those two blocks matched while `settings` did not.
		Input: schemaerr.RenderInput(item.Value),
	}
}

// source is a placeholder for the per-item source; every settings key comes from
// the document the caller passed, so the caller's own source is the answer.
func source(yamldoc.Item) schemaerr.YamlSource { return schemaerr.SourceMain }

// mappingValue reads one key of a mapping node, reporting whether it was
// there. Repeated from `models.mappingValue` rather than imported: that
// package imports this one, so the reverse edge would be a cycle.
func mappingValue(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil, false
	}
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}

func known(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// CodeExtraForbidden and messageExtraForbidden are the binder's, repeated rather
// than imported because `binder` imports this package's siblings and the cycle
// is not worth breaking for two constants.
const CodeExtraForbidden schemaerr.Code = "extra_forbidden"

const messageExtraForbidden = "Extra inputs are not permitted"
