package design_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// TestThemeNameIsThePythonText pins `str(design["theme"])` (`design.py:57`) for
// the shapes `TestValidateThemeNonStringValues` does not reach.
//
// **A number inside a container is its value, not its token.** `design.theme:
// [0x1f]` reads `[31]` upstream and read `[0x1f]` here, `[1.50]` reads `[1.5]`
// and read `[1.50]`, because `design.go`'s own container renderer returned the
// raw text for `KindInt` and `KindFloat` — the half `schemaerr.RenderInput`
// had right for a scalar and a hand-rolled copy did not.
//
// **The quote and the escapes are Python's**, which the same copy did not have
// either: `{a: "it's"}` reprs as `{'a': "it's"}`, not `{'a': 'it's'}`.
//
// The message and the Input Value column carry the *same* rendering here —
// upstream re-pins both to the theme name (`design.py:64-68`), unlike
// `locale.language`, whose input stays the block's `...`. Both are measured
// through the vendored `build_rendercv_dictionary_and_model`.
func TestThemeNameIsThePythonText(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		// The two the token/value confusion is visible on.
		{name: "hex inside a sequence", value: "[0x1f]", want: "[31]"},
		{name: "trailing zero float inside", value: "[1.50]", want: "[1.5]"},
		{name: "underscores inside", value: "{a: 1_000}", want: "{'a': 1000}"},

		// Python's own quoting.
		{name: "apostrophe in a value", value: `{a: "it's"}`, want: `{'a': "it's"}`},
		{name: "backslash in a value", value: `{a: 'a\b'}`, want: `{'a': 'a\\b'}`},
		{name: "non-ascii in a value", value: "{a: 'Ольга'}", want: "{'a': 'Ольга'}"},
		{name: "empty string inside", value: "{a: ''}", want: "{'a': ''}"},

		// Shapes the previous renderer already had right, kept so the
		// replacement cannot regress them.
		{name: "mapping", value: "{a: 1}", want: "{'a': 1}"},
		{name: "mapping of two", value: "{a: 1, b: two}", want: "{'a': 1, 'b': 'two'}"},
		{name: "nested", value: "{a: [1, {b: c}]}", want: "{'a': [1, {'b': 'c'}]}"},
		{name: "sequence of strings", value: "[english, french]", want: "['english', 'french']"},
		{name: "empty mapping", value: "{}", want: "{}"},
		{name: "empty sequence", value: "[]", want: "[]"},
		{name: "null and bools inside", value: "[null, true, false]", want: "[None, True, False]"},
		{name: "null", value: "null", want: "None"},
		{name: "true", value: "true", want: "True"},
		{name: "false", value: "false", want: "False"},
		{name: "float", value: "1.50", want: "1.5"},
		{name: "uppercase name", value: "Classic", want: "Classic"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := design.ValidateTheme(
				themeNode(t, c.value), []string{"design"}, schemaerr.SourceMain,
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}

			want := "The custom theme name should only contain lowercase letters and" +
				" digits. The provided value is `" + c.want + "`."
			if errs[0].Message != want {
				t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, want)
			}
			// Upstream re-pins the input to the name as well as the message.
			if errs[0].Input != c.want {
				t.Errorf("input = %q, want %q", errs[0].Input, c.want)
			}
		})
	}
}

// TestTheThemeFolderIsTheNamesValue is the other half of the same `str()`:
// upstream computes `theme_name` once (`design.py:57`) and looks the **folder**
// up under it, so a numeric theme opens the folder its value spells, not the
// one its token does.
//
// Measured against the vendored pipeline: `theme: 007` reports the folder `7`
// and `theme: 0x1f` reports `31`, where the port reported `007` and `0x1f`.
// Both names pass the lowercase-alphanumeric check, so this is the branch
// beyond the message the rest of this file pins.
func TestTheThemeFolderIsTheNamesValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "octal", value: "007", want: "7"},
		{name: "hex", value: "0x1f", want: "31"},
		{name: "decimal", value: "31", want: "31"},
		// A string theme is its own text, which is every real document.
		{name: "a name", value: "nosuchtheme", want: "nosuchtheme"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "CV.yaml")
			doc, err := yamlreader.ReadString("theme: " + c.value + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}

			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain,
				&valctx.ValidationContext{InputFilePath: input})
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}

			want := "The custom theme folder `" + filepath.Join(filepath.Dir(input), c.want) +
				"` does not exist. It should be in the same directory as the input file."
			if errs[0].Message != want {
				t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, want)
			}
			if strings.Contains(errs[0].Message, "`"+filepath.Join(filepath.Dir(input), c.value)+"`") &&
				c.value != c.want {
				t.Errorf("the folder is the token %q, not the value %q", c.value, c.want)
			}
		})
	}
}
