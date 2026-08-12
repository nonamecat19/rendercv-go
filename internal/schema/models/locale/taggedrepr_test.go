package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// TestATaggedScalarReprsAsItsConstructor pins spec 015 delta §3, findings 1
// and 2: a `TaggedScalar` inside a container is `repr`'d, and its `repr` is a
// constructor call rather than its text.
//
//	TaggedScalar(value=<repr>, style=<repr>, tag=Tag(<repr>))
//
// one f-string, `ruamel/yaml/comments.py:1186-1187`, with
// `Tag.__repr__` at `ruamel/yaml/tag.py:31-32`.
//
// **A top-level `TaggedScalar` is not affected and must not move.** `__str__`
// is the bare value (`comments.py:1177-1178`), so `language: !!str english`
// stays `Input tag 'english'`. The whole finding is the difference between
// `str()` and `repr()`, and the port already splits those in the right place:
// `PythonText` is `str()`, `pythonRepr` is what a container uses for its
// members.
//
// Every `want` was measured through the vendored
// `build_rendercv_dictionary_and_model`, reading `message` off the raised
// `RenderCVValidationError` — structurally, not by regex over `str(exc)`,
// because the quoted-style rows put a `'` or a `"` inside the message and the
// naive extraction lost them.
func TestATaggedScalarReprsAsItsConstructor(t *testing.T) {
	const strTag = "tag:yaml.org,2002:str"

	tests := []struct {
		name string
		yaml string
		want string
	}{
		// §3.4 — the shapes the finding names.
		{
			name: "in a sequence", yaml: "[!!str x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "in a mapping", yaml: "{a: !!str x}",
			want: `{'a': TaggedScalar(value='x', style=None, tag=Tag('` + strTag + `'))}`,
		},
		{
			name: "local tag", yaml: "[!unknown x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('!unknown'))]`,
		},
		{
			// The row that shows the port's old output was not merely a
			// different rendering: `[31]` is indistinguishable from a plain
			// `[31]`, which upstream renders differently on purpose.
			name: "digits stay a string", yaml: "[!!str 31]",
			want: `[TaggedScalar(value='31', style=None, tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "two in a sequence", yaml: "[!!str x, !unknown y]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('` + strTag + `')), ` +
				`TaggedScalar(value='y', style=None, tag=Tag('!unknown'))]`,
		},
		{
			name: "nested in a mapping", yaml: "{a: [!!str x]}",
			want: `{'a': [TaggedScalar(value='x', style=None, tag=Tag('` + strTag + `'))]}`,
		},
		{
			name: "nested in a sequence", yaml: "[[!!str x]]",
			want: `[[TaggedScalar(value='x', style=None, tag=Tag('` + strTag + `'))]]`,
		},

		// §3.3 — the tag table. `!!X` expands through
		// `DEFAULT_TAGS = {'!': '!', '!!': 'tag:yaml.org,2002:'}`
		// (`ruamel/yaml/parser.py:106`); anything else is its own text,
		// URI-decoded (`tag.py:55-88`).
		{
			name: "merge tag", yaml: "[!!merge x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:merge'))]`,
		},
		{
			name: "local without bangs", yaml: "[!foo bar]",
			want: `[TaggedScalar(value='bar', style=None, tag=Tag('!foo'))]`,
		},
		{
			name: "suffix with a slash", yaml: "[!!foo/bar x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:foo/bar'))]`,
		},
		{
			name: "suffix case is kept", yaml: "[!!STR x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('tag:yaml.org,2002:STR'))]`,
		},
		{
			// `!!` is **not** the `!!` handle: a handle needs a suffix, so the
			// scanner reads a local tag whose text is `!`, and the `!` handle
			// maps to `!` — giving `!!` back.
			name: "double bang alone", yaml: "[!! x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('!!'))]`,
		},
		{
			// The suffix is URI-decoded, so `%21` is a `!`.
			name: "percent-escaped bang", yaml: "[!%21 x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('!!'))]`,
		},
		{
			name: "verbatim local", yaml: "[!<!local> x]",
			want: `[TaggedScalar(value='x', style=None, tag=Tag('!local'))]`,
		},

		// §3.2 — the style table, all five.
		{
			name: "single quoted", yaml: "\n    - !!str 'q'",
			want: `[TaggedScalar(value='q', style="'", tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "double quoted", yaml: "\n    - !!str \"d\"",
			want: `[TaggedScalar(value='d', style='"', tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "literal", yaml: "\n    - !!str |\n        x",
			want: `[TaggedScalar(value='x\n', style='|', tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "folded", yaml: "\n    - !!str >\n        x",
			want: `[TaggedScalar(value='x\n', style='>', tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "no value at all", yaml: "\n    - !!str",
			want: `[TaggedScalar(value='', style=None, tag=Tag('` + strTag + `'))]`,
		},

		// The value goes through Python's `repr`, quote selection and all.
		{
			name: "apostrophe in the value", yaml: "[!!str \"it's\"]",
			want: `[TaggedScalar(value="it's", style='"', tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "backslash in the value", yaml: `[!!str 'a\b']`,
			want: `[TaggedScalar(value='a\\b', style="'", tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "empty quoted value", yaml: "[!!str '']",
			want: `[TaggedScalar(value='', style="'", tag=Tag('` + strTag + `'))]`,
		},
		{
			name: "non-ascii value", yaml: "[!!str ürk]",
			want: `[TaggedScalar(value='ürk', style=None, tag=Tag('` + strTag + `'))]`,
		},

		// **A tagged collection is not a TaggedScalar.** `construct_unknown`
		// branches on the node's shape (`constructor.py:1598-1640`), so the tag
		// is dropped and the collection is an ordinary one. These already
		// passed and are here so the fix cannot reach them.
		{name: "tagged sequence", yaml: "[!!seq [1]]", want: "[[1]]"},
		{name: "tagged mapping", yaml: "[!!map {a: 1}]", want: "[{'a': 1}]"},
		{name: "unknown tag on a sequence", yaml: "[!unknown [1]]", want: "[[1]]"},
		{name: "unknown tag on a mapping", yaml: "[!unknown {a: 1}]", want: "[{'a': 1}]"},

		// A **forcing** tag constructs a value, which is not a TaggedScalar
		// either (`yamlreader.ResolveTag`). Also already passing.
		{name: "forced int", yaml: "[!!int 1]", want: "[1]"},
		{name: "forced bool", yaml: "[!!bool yes]", want: "[True]"},
		{name: "forced null", yaml: "[!!null x]", want: "[None]"},
		{name: "forced float", yaml: "[!!float 1.50]", want: "[1.5]"},
		{name: "forced timestamp", yaml: "[!!timestamp 2024-01-01]", want: "['2024-01-01']"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block, language := localeBlock(t, test.yaml)
			errs := locale.ValidateLanguage(block, language, []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}

			want := "Input tag '" + test.want + "' found using 'language' does not match"
			if !strings.HasPrefix(errs[0].Message, want) {
				t.Errorf("message =\n  %q\nwant it to start\n  %q", errs[0].Message, want)
			}
		})
	}
}

// The top-level case, which is `str()` and is already right. It is asserted
// separately because it is the thing the fix must **not** change:
// `RenderInput` and `PythonText`'s own `KindTagged` arm both stay as they are.
func TestATopLevelTaggedScalarStaysItsText(t *testing.T) {
	for _, yaml := range []string{"!!str english", "!unknown english", "!! english"} {
		t.Run(yaml, func(t *testing.T) {
			block, language := localeBlock(t, yaml)
			errs := locale.ValidateLanguage(block, language, []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			const want = "Input tag 'english' found using 'language' does not match"
			if !strings.HasPrefix(errs[0].Message, want) {
				t.Errorf("message =\n  %q\nwant it to start\n  %q", errs[0].Message, want)
			}
		})
	}
}
