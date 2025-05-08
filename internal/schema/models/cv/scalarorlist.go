package cv

import (
	"errors"
	"strconv"

	"github.com/nonamecat19/rendercv-go/internal/schema/emailaddr"
	"github.com/nonamecat19/rendercv-go/internal/schema/httpurl"
	"github.com/nonamecat19/rendercv-go/internal/schema/phonenum"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ElementValidator validates one element of a scalar-or-list field. The node is
// the element as written; location is its schema location.
type ElementValidator func(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError

// elementValidators maps a scalar-or-list field to its element validator,
// mirroring upstream's `{website, email, phone}` adapter table
// (cv.py:210-224). A field absent from this table is not a scalar-or-list
// field.
var elementValidators = map[string]ElementValidator{
	"website": validateWebsiteElement,
	"email":   validateEmailElement,
	"phone":   validatePhoneElement,
}

// emailTemplatePrefix is pydantic's own message template,
// `'value is not a valid email address: {reason}'`, used at two sites in
// `validate_email`.
//
// **This is the one place in the port that builds a prefixed message**, and it
// is deliberate. Spec 004 §3.2 behavior 4a: the prefix is source text upstream
// can be cited for, so the port reproduces it and the pipeline's strip runs on
// production data rather than only on synthetic records. `emailaddr` returns the
// bare reason precisely so that the knowledge of this contract lives here, at
// the one call site that feeds the pipeline.
const emailTemplatePrefix = "value is not a valid email address: "

// The codes the three element validators emit, all measured (spec 004 §3.9c).
// Email and phone are `value_error` because upstream raises a
// PydanticCustomError with that type; neither message carries the
// `Value error, ` prefix, which is behavior 4b's point.
const codeValueError schemaerr.Code = "value_error"

func validateEmailElement(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}
	if _, err := emailaddr.Validate(node.Raw); err != nil {
		return []schemaerr.ValidationError{elementError(
			codeValueError, emailTemplatePrefix+err.Error(), location, node, source,
		)}
	}
	return nil
}

func validatePhoneElement(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}
	if _, err := phonenum.Validate(node.Raw); err != nil {
		// The dictionary key, not its replacement — see phonenum.MessageInvalid.
		return []schemaerr.ValidationError{elementError(
			codeValueError, err.Error(), location, node, source,
		)}
	}
	return nil
}

func validateWebsiteElement(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil
	}
	_, err := httpurl.Validate(node.Raw)
	if err == nil {
		return nil
	}

	var urlErr *httpurl.Error
	if !errors.As(err, &urlErr) {
		return nil
	}
	return []schemaerr.ValidationError{elementError(
		urlErr.Code, urlErr.Message, location, node, source,
	)}
}

func elementError(
	code schemaerr.Code,
	message string,
	location []string,
	node *yamldoc.Node,
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

// ScalarOrListFields returns the field names governed by the shared rule, in
// upstream's declaration order (cv.py:177).
func ScalarOrListFields() []string {
	return []string{"website", "email", "phone"}
}

// ValidateScalarOrList mirrors validate_list_or_scalar_fields
// (cv.py:177-229). It inspects the value before any coercion, which is what
// makes the resulting error name the element type rather than reporting a union
// failure (spec §3.47):
//
//   - absent (or null) → absent, with no element validation (cv.py:205-206);
//   - a list → each element validated as the field's element type
//     (cv.py:226-227);
//   - anything else → validated as one value of that type (cv.py:229).
//
// Invoking it without a field name is the internal error of spec §4.7
// (cv.py:208-209).
func ValidateScalarOrList(
	fieldName string,
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) ([]schemaerr.ValidationError, error) {
	// Optional fields: a null value is accepted and validates nothing.
	if node == nil || node.Kind == yamldoc.KindNull {
		return nil, nil
	}

	if fieldName == "" {
		return nil, &schemaerr.InternalError{Message: "field_name is None in validator"}
	}

	validate, ok := elementValidators[fieldName]
	if !ok {
		return nil, &schemaerr.InternalError{Message: "field_name is None in validator"}
	}

	if node.Kind == yamldoc.KindSequence {
		var errs []schemaerr.ValidationError
		for i, elem := range node.Elems {
			errs = append(errs, validate(elem, indexLocation(location, i), source)...)
		}
		return errs, nil
	}

	return validate(node, location, source), nil
}

// SerializePhone mirrors serialize_phone (cv.py:231-250): the phone library's
// `tel:` URI scheme is stripped for rendering.
func SerializePhone(phone string) string {
	return phonenum.Serialize(phone)
}

func indexLocation(location []string, index int) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, strconv.Itoa(index))
}
