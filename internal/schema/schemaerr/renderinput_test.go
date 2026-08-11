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
			// **The column carries Python's `str(True)`, not the YAML
			// token.** Found by a fresh-context verifier (iteration 14's
			// fourteenth re-verification).
			name: "a lowercase true renders as Python's True",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "true"},
			want: "True",
		},
		{
			name: "an uppercase TRUE renders as Python's True",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "TRUE"},
			want: "True",
		},
		{
			name: "false renders as Python's False",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "false"},
			want: "False",
		},
		{
			name: "a string renders as written",
			node: &yamldoc.Node{Kind: yamldoc.KindString, Raw: "not_a_valid_url"},
			want: "not_a_valid_url",
		},
		// YAML 1.1's bool spellings, which only an explicit `!!bool` tag can
		// bring to a KindBool node: `!!bool yes` is `True` upstream, and this
		// arm read anything but `true` as `False`.
		{
			name: "a tagged yes renders as Python's True",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "yes"},
			want: "True",
		},
		{
			name: "a tagged on renders as Python's True",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "On"},
			want: "True",
		},
		{
			name: "a tagged off renders as Python's False",
			node: &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "off"},
			want: "False",
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
