package schemaerr_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Spec 004 §3.11 behavior 39 and §4.15. Every kind, because the rule is one
// switch and a missed arm falls through to the raw text silently.
func TestRenderInput(t *testing.T) {
	tests := []struct {
		name string
		node *yamldoc.Node
		want string
	}{
		{
			// A missing field's input is the whole enclosing mapping, which is
			// why this case is the common one rather than an edge
			// (expected_errors.yaml:59, :65, :77).
			name: "a mapping renders as three dots, not its contents",
			node: &yamldoc.Node{Kind: yamldoc.KindMapping, Items: []yamldoc.Item{{Key: "a"}}},
			want: "...",
		},
		{
			name: "so does a sequence",
			node: &yamldoc.Node{Kind: yamldoc.KindSequence, Elems: []*yamldoc.Node{{Raw: "x"}}},
			want: "...",
		},
		{
			// Python's spelling, whatever the YAML wrote.
			name: "a null renders as None",
			node: &yamldoc.Node{Kind: yamldoc.KindNull, Raw: "~"},
			want: "None",
		},
		{
			name: "an absent node renders as None too",
			node: nil,
			want: "None",
		},
		{
			name: "an integer renders as written",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "5"},
			want: "5",
		},
		{
			name: "a float renders as written",
			node: &yamldoc.Node{Kind: yamldoc.KindFloat, Raw: "3.14"},
			want: "3.14",
		},
		{
			name: "a bool renders as written",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "true"},
			want: "true",
		},
		{
			name: "a string renders as written",
			node: &yamldoc.Node{Kind: yamldoc.KindString, Raw: "not_a_valid_url"},
			want: "not_a_valid_url",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := schemaerr.RenderInput(test.node); got != test.want {
				t.Errorf("RenderInput = %q, want %q", got, test.want)
			}
		})
	}
}
