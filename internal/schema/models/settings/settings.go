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
