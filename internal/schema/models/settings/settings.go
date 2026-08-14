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
