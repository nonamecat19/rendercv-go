package cv

import (
	"strconv"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ElementValidator validates one element of a scalar-or-list field. The node is
// the element as written; location is its schema location.
//
// TODO(iteration-4): the three real validators — email address, HTTP URL and
// phone number semantics (`cv.py:14-28`, upstream's pydantic type adapters) —
// are iteration 4's (spec §7). Iteration 2 registers pass-through
// implementations so the routing rule of spec §3.47 can be tested on its own.
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
	"website": passThroughValidator,
	"email":   passThroughValidator,
	"phone":   passThroughValidator,
}

func passThroughValidator(
	*yamldoc.Node,
	[]string,
	schemaerr.YamlSource,
) []schemaerr.ValidationError {
	return nil
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
//
// TODO(iteration-4): phone normalization itself — `+905419999999` rendering as
// `+90-541-999-99-99` (spec §3.49) — is upstream's phone library's, and is
// iteration 4's (spec §7).
func SerializePhone(phone string) string {
	return strings.ReplaceAll(phone, "tel:", "")
}

func indexLocation(location []string, index int) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, strconv.Itoa(index))
}
