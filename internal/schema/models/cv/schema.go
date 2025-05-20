package cv

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// SocialNetworkNameSchema is the seventeen-name literal union
// (social_network.py:13-31).
//
// The members come from SocialNetworkNames rather than being written out, so the
// schema and the validator cannot disagree — and the order is the literal type's
// declaration order, which spec 004 §4.23 already pinned in the error message.
// A reordering therefore fails two tests rather than none.
func SocialNetworkNameSchema() *jsonschema.Object {
	members := make([]any, 0, len(SocialNetworkNames))
	for _, name := range SocialNetworkNames {
		members = append(members, string(name))
	}
	return jsonschema.NewObject().
		Set("enum", members).
		Set("type", "string")
}

// TypstDimensionSchema is `TypstDimension` (design/typst_dimension.py).
//
// It is a design type and it is here because nothing in `design` exists yet and
// `Cv` does not reach it either — it is shared, already modelled as a string,
// and belongs to whichever package first needs it. Iteration 6 may move it.
func TypstDimensionSchema() *jsonschema.Object {
	return jsonschema.NewObject().Set("type", "string")
}

// ExistingPathRelativeToInputSchema is the required-path type (path.py:67-72).
//
// `format: path` is pydantic's annotation for a `pathlib.Path`, and it carries
// no validation weight in JSON Schema — it is metadata an IDE may use.
func ExistingPathRelativeToInputSchema() *jsonschema.Object {
	return jsonschema.NewObject().
		Set("format", "path").
		Set("type", "string")
}
