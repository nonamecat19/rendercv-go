package locale

import (
	"fmt"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// The two codes a wrong-length list carries. **Two, not one**: the bound that
// was violated decides both the code and the wording (spec 007 §3 behavior 10).
//
// Implementing the constraint as `!= 12` would produce one message where
// upstream produces two, which is right for neither case.
const (
	CodeTooShort schemaerr.Code = "too_short"
	CodeTooLong  schemaerr.Code = "too_long"
)

// lengthError is a wrong-length list, carrying the code its bound implies.
type lengthError struct {
	code    schemaerr.Code
	message string
}

func (e *lengthError) Error() string { return e.message }

// ErrorCode implements schemaerr.Coded, so one validator can report either of
// the two codes — the mechanism iteration 4 added for URLs.
func (e *lengthError) ErrorCode() schemaerr.Code { return e.code }

// validateMonthList enforces the exactly-twelve constraint.
//
// Both messages interpolate the **actual** count, so neither is a constant, and
// neither matches a dictionary row — the pipeline only appends a period.
func validateMonthList(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node.Kind != yamldoc.KindSequence {
		// The shape failure is the binder's to report, not this rule's.
		return nil
	}

	got := len(node.Elems)
	if got == MonthCount {
		return nil
	}

	failure := &lengthError{
		code: CodeTooShort,
		message: fmt.Sprintf(
			"List should have at least %d items after validation, not %d", MonthCount, got),
	}
	if got > MonthCount {
		failure = &lengthError{
			code: CodeTooLong,
			message: fmt.Sprintf(
				"List should have at most %d items after validation, not %d", MonthCount, got),
		}
	}

	span := node.Span
	return []schemaerr.ValidationError{{
		Code:           failure.ErrorCode(),
		SchemaLocation: append([]string(nil), location...),
		YamlLocation:   &span,
		YamlSource:     source,
		Message:        failure.Error(),
		Input:          schemaerr.RenderInput(node),
	}}
}
