package design

import (
	"regexp"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// typstDimensionPattern is `validate_typst_dimension`'s
// (design/typst_dimension.py:23), anchored because upstream uses `re.fullmatch`.
//
// Three things it decides, each measured rather than read off the regex:
//
//   - `1` fails. A bare number is not a dimension; the unit is required.
//   - `1 cm` fails. No space is admitted between number and unit.
//   - `-0.5in` passes. A negative dimension is legal, and Typst uses them.
var typstDimensionPattern = regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:cm|in|pt|mm|em)$`)

// MessageBadTypstDimension is spec 006 §4A.1, verbatim
// (typst_dimension.py:26-27). No dictionary row matches it, so the pipeline only
// appends a period.
const MessageBadTypstDimension = "The value must be a number followed by a unit" +
	" (cm, in, pt, mm, em). For example, 0.1cm."

// CodeTypstDimension is the kind `PydanticCustomError(other)` raises
// (models/custom_error_types.py:6) — the same code the theme-name check uses,
// which is why it is a constant here rather than reused from `CodeTheme`: they
// are equal today by upstream's choice, not by construction.
const CodeTypstDimension schemaerr.Code = "rendercv_other_error"

// ValidTypstDimension reports whether a string is a Typst dimension.
//
// **The type check is not this rule's.** `top_margin: 1` — an integer, not a
// string — gives `string_type` from the binder and never reaches here; upstream
// runs `AfterValidator` on a `str`, so the same split holds there.
func ValidTypstDimension(value string) bool {
	return typstDimensionPattern.MatchString(value)
}
