package entries

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// DateSchema is `ArbitraryDate` and ExactDateSchema is `ExactDate`
// (entry_with_date.py:34-36, entry_with_complex_fields.py:40).
//
// **The arm order differs between them and it is upstream's**: `int | str`
// against `str | int`. It is the same asymmetry spec 004 §3.9b made observable
// in error messages, showing up here a second time — so a port that "tidied"
// either one would break two contracts at once.
func DateSchema() *jsonschema.Object {
	return jsonschema.NewObject().Set("anyOf", []any{
		jsonschema.NewObject().Set("type", "integer"),
		jsonschema.NewObject().Set("type", "string"),
	})
}

// ExactDateSchema is `ExactDate`.
func ExactDateSchema() *jsonschema.Object {
	return jsonschema.NewObject().Set("anyOf", []any{
		jsonschema.NewObject().Set("type", "string"),
		jsonschema.NewObject().Set("type", "integer"),
	})
}
