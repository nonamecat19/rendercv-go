package yamlreader_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// A bare `!` is the **non-specific tag**, and it is not a tag at all for the
// loader's purposes: it asks the resolver for the usual answer rather than
// naming a type, so `! x` is the string `x` and `! 31` is the integer 31 —
// neither is a `TaggedScalar`.
//
// Spec 015 delta §3.3 records this as the one row of the tag table that
// produces no tag. It matters more than its rarity suggests: `KindTagged` is
// the kind every typed field rejects (`yamldoc/node.go:22-36`), so reading `!`
// as a tag turns a document upstream accepts into a validation error —
// measured, `locale.language: ! english` raises nothing at all upstream.
//
// Measured through the vendored `build_rendercv_dictionary_and_model`, reading
// the `union_tag_invalid` message for a `locale.language` of each shape:
//
//	! english  ->  no error at all
//	! x        ->  Input tag 'x'
//	! 31       ->  Input tag '31'
//	[! x]      ->  Input tag '['x']'      — a quoted string, not a TaggedScalar
//	[! 31]     ->  Input tag '[31]'       — an integer
func TestABareTagIsNonSpecific(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want yamldoc.Kind
		raw  string
	}{
		{name: "string", yaml: "! x", want: yamldoc.KindString, raw: "x"},
		{name: "integer", yaml: "! 31", want: yamldoc.KindInt, raw: "31"},
		{name: "float", yaml: "! 1.5", want: yamldoc.KindFloat, raw: "1.5"},
		{name: "bool", yaml: "! true", want: yamldoc.KindBool, raw: "true"},
		{name: "null", yaml: "! null", want: yamldoc.KindNull, raw: "null"},

		// The neighbours, which **are** tags and must not be swept up with it.
		// `!!` is not the `!!` handle: a handle needs a suffix, so the scanner
		// reads a local tag whose text is `!`, and upstream reprs it `Tag('!!')`.
		{name: "double bang", yaml: "!! x", want: yamldoc.KindTagged, raw: "x"},
		{name: "local", yaml: "!unknown x", want: yamldoc.KindTagged, raw: "x"},
		{name: "escaped bang", yaml: "!%21 x", want: yamldoc.KindTagged, raw: "x"},
		{name: "explicit str", yaml: "!!str x", want: yamldoc.KindTagged, raw: "x"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("language: " + test.yaml + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := doc.Items[0].Value
			if node.Kind != test.want {
				t.Errorf("Kind = %v, want %v", node.Kind, test.want)
			}
			if node.Raw != test.raw {
				t.Errorf("Raw = %q, want %q", node.Raw, test.raw)
			}
		})
	}
}
