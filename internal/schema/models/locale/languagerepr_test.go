package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// TestLanguageTagIsThePythonText pins the tag pydantic quotes in
// `union_tag_invalid` for every shape `locale.language` can take.
//
// **The tag is `str()` of the parsed value, not the Input Value column's
// rendering.** The port used `schemaerr.RenderInput`, whose container arms
// return `...` because that is what the *column* shows
// (`pydantic_error_handling.py:126`); the message quotes Python's own repr, so
// `{language: {a: 1}}` reads `Input tag '{'a': 1}'` upstream and
// `Input tag '...'` here.
//
// **A number is its value, not its token**: `0x1f` is `31` and `1.50` is `1.5`,
// at the top level and inside a container alike. That is the half `RenderInput`
// already had right for scalars and the half a container renderer copied from
// `design.go` would have got wrong.
//
// Every `want` was measured through the vendored
// `build_rendercv_dictionary_and_model`, reading the tag out of the raised
// `RenderCVValidationError.message`.
func TestLanguageTagIsThePythonText(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		// Containers: the shapes the port got wrong.
		{name: "mapping", yaml: "{a: 1}", want: "{'a': 1}"},
		{name: "mapping of two", yaml: "{a: 1, b: two}", want: "{'a': 1, 'b': 'two'}"},
		{name: "sequence", yaml: "[1, 2]", want: "[1, 2]"},
		{name: "sequence of strings", yaml: "[english, french]", want: "['english', 'french']"},
		{name: "nested", yaml: "{a: [1, {b: c}]}", want: "{'a': [1, {'b': 'c'}]}"},
		{name: "sequence of sequences", yaml: "[[a, b], [c]]", want: "[['a', 'b'], ['c']]"},
		{name: "mapping in a sequence", yaml: "[{a: 1}]", want: "[{'a': 1}]"},
		{name: "empty mapping", yaml: "{}", want: "{}"},
		{name: "empty sequence", yaml: "[]", want: "[]"},
		{name: "null and bools inside", yaml: "[null, true, false]", want: "[None, True, False]"},

		// A number inside a container is its **value**: these two are the
		// shapes that separate a real repr from a rendering of the token.
		{name: "hex inside", yaml: "[0x1f]", want: "[31]"},
		{name: "trailing zero float inside", yaml: "[1.50]", want: "[1.5]"},
		{name: "underscores inside", yaml: "{a: 1_000}", want: "{'a': 1000}"},
		{name: "exponent inside", yaml: "{a: 1e308}", want: "{'a': 1e+308}"},
		{name: "infinity inside", yaml: "{a: .inf}", want: "{'a': inf}"},
		{name: "not a number inside", yaml: "{a: .nan}", want: "{'a': nan}"},

		// Python's `repr` picks its own quote and escapes.
		{name: "apostrophe in a value", yaml: `{a: "it's"}`, want: `{'a': "it's"}`},
		{name: "backslash in a value", yaml: `{a: 'a\b'}`, want: `{'a': 'a\\b'}`},
		{name: "tab in a value", yaml: `{a: "x\ty"}`, want: `{'a': 'x\ty'}`},
		{name: "newline in a value", yaml: `{a: "x\ny"}`, want: `{'a': 'x\ny'}`},
		{name: "non-ascii in a value", yaml: "{a: 'Ольга'}", want: "{'a': 'Ольга'}"},
		{name: "empty string value", yaml: "{a: ''}", want: "{'a': ''}"},

		// A key is repr'd like a value, so a non-string key is unquoted.
		{name: "integer key", yaml: "\n  1: a", want: "{1: 'a'}"},
		{name: "boolean key", yaml: "\n  true: a", want: "{True: 'a'}"},
		{name: "null key", yaml: "\n  null: a", want: "{None: 'a'}"},
		{name: "float key", yaml: "\n  1.50: a", want: "{1.5: 'a'}"},
		{name: "nested mapping value", yaml: "\n  a:\n    b: 1", want: "{'a': {'b': 1}}"},

		// Scalars, which `RenderInput` already rendered correctly and which the
		// replacement must not regress.
		{name: "unknown string", yaml: "klingon", want: "klingon"},
		{name: "quoted string", yaml: "'klingon'", want: "klingon"},
		{name: "null", yaml: "null", want: "None"},
		{name: "true", yaml: "true", want: "True"},
		{name: "false", yaml: "false", want: "False"},
		{name: "integer", yaml: "31", want: "31"},
		{name: "hex", yaml: "0x1f", want: "31"},
		{name: "float", yaml: "1.50", want: "1.5"},
		{name: "tagged scalar", yaml: "!!str english", want: "english"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			block, language := localeBlock(t, c.yaml)
			errs := locale.ValidateLanguage(block, language, []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}

			want := "Input tag '" + c.want + "' found using 'language' does not match"
			if !strings.HasPrefix(errs[0].Message, want) {
				t.Errorf("message =\n  %q\nwant it to start\n  %q", errs[0].Message, want)
			}
		})
	}
}
