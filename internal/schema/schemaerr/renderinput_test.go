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
		// **An integer's spelling is not its value.** The column carries `str()`
		// of the parsed object, so the base prefix, the underscores, the leading
		// zeros and a `+` sign are all gone. Measured on fourteen spellings
		// through upstream's own loader; `+905419999999` is the one that matters,
		// an unquoted WhatsApp username.
		{
			name: "a leading plus is dropped",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "+905419999999"},
			want: "905419999999",
		},
		{
			name: "hexadecimal renders in decimal",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "0x1f"},
			want: "31",
		},
		{
			name: "octal renders in decimal",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "0o17"},
			want: "15",
		},
		{
			name: "binary renders in decimal",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "0b101"},
			want: "5",
		},
		{
			name: "underscores are dropped",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "1_000"},
			want: "1000",
		},
		{
			name: "leading zeros are dropped",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "007"},
			want: "7",
		},
		{
			name: "an int has no negative zero",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "-0"},
			want: "0",
		},
		{
			name: "a negative integer keeps its sign",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "-42"},
			want: "-42",
		},
		{
			// Unreadable as an int64 — the raw text is better than a wrong
			// number. The same value's Python `str()` would be exact, which is
			// the float/bignum half of the gap and still open.
			name: "a token too large to parse stays as written",
			node: &yamldoc.Node{Kind: yamldoc.KindInt, Raw: "99999999999999999999999"},
			want: "99999999999999999999999",
		},
		// An opaque tagged scalar renders as its text, not as `None`: upstream's
		// TaggedScalar keeps the value it could not type, and the table echoes
		// it while the field still fails.
		{
			name: "an opaque tagged scalar renders as its text",
			node: &yamldoc.Node{Kind: yamldoc.KindTagged, Raw: "Bob"},
			want: "Bob",
		},
		{
			name: "an empty opaque tagged scalar renders as nothing",
			node: &yamldoc.Node{Kind: yamldoc.KindTagged, Raw: ""},
			want: "",
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
