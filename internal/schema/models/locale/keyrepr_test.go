package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// TestMappingKeyRepr pins the rendering of a mapping **key** in the tag
// pydantic quotes in `union_tag_invalid` — spec 015 delta §4.
//
// A key is `repr`'d exactly like a value, so its *type* decides the spelling:
// `{1: a}` is `{1: 'a'}` and `{'1': a}` is `{'1': 'a'}`. **Every shape appears
// twice, bare and quoted**, because the whole finding is that the port could
// not tell the two apart: `yamldoc.Item` kept the key's text and dropped its
// kind, so a rule guessed from the text would have moved the defect to the
// other spelling rather than fixed it.
//
// Every `want` was measured through the vendored
// `build_rendercv_dictionary_and_model`, reading the tag out of the raised
// `RenderCVValidationError.message`. The flow spelling used here and the block
// spelling `\n  1: a` were measured separately and agree.
func TestMappingKeyRepr(t *testing.T) {
	cases := []keyReprCase{
		{name: "integer", yaml: "{1: a}", want: "{1: 'a'}"},
		{name: "integer single quoted", yaml: "{'1': a}", want: "{'1': 'a'}"},
		{name: "integer double quoted", yaml: `{"1": a}`, want: "{'1': 'a'}"},

		{name: "negative integer", yaml: "{-1: a}", want: "{-1: 'a'}"},
		{name: "negative integer quoted", yaml: "{'-1': a}", want: "{'-1': 'a'}"},
		{name: "signed integer", yaml: "{+1: a}", want: "{1: 'a'}"},
		{name: "signed integer quoted", yaml: "{'+1': a}", want: "{'+1': 'a'}"},
		{name: "negative zero", yaml: "{-0: a}", want: "{0: 'a'}"},
		{name: "negative zero quoted", yaml: "{'-0': a}", want: "{'-0': 'a'}"},

		{name: "hex", yaml: "{0x1f: a}", want: "{31: 'a'}"},
		{name: "hex quoted", yaml: "{'0x1f': a}", want: "{'0x1f': 'a'}"},
		{name: "octal", yaml: "{0o17: a}", want: "{15: 'a'}"},
		{name: "octal quoted", yaml: "{'0o17': a}", want: "{'0o17': 'a'}"},
		{name: "binary", yaml: "{0b101: a}", want: "{5: 'a'}"},
		{name: "binary quoted", yaml: "{'0b101': a}", want: "{'0b101': 'a'}"},
		{name: "underscores", yaml: "{1_000: a}", want: "{1000: 'a'}"},
		{name: "underscores quoted", yaml: "{'1_000': a}", want: "{'1_000': 'a'}"},

		{name: "float", yaml: "{1.50: a}", want: "{1.5: 'a'}"},
		{name: "float quoted", yaml: "{'1.50': a}", want: "{'1.50': 'a'}"},
		{name: "exponent", yaml: "{1e3: a}", want: "{1000.0: 'a'}"},
		{name: "exponent quoted", yaml: "{'1e3': a}", want: "{'1e3': 'a'}"},
		{name: "infinity", yaml: "{.inf: a}", want: "{inf: 'a'}"},
		{name: "infinity quoted", yaml: "{'.inf': a}", want: "{'.inf': 'a'}"},
		{name: "not a number", yaml: "{.nan: a}", want: "{nan: 'a'}"},
		{name: "not a number quoted", yaml: "{'.nan': a}", want: "{'.nan': 'a'}"},

		{name: "true", yaml: "{true: a}", want: "{True: 'a'}"},
		{name: "true single quoted", yaml: "{'true': a}", want: "{'true': 'a'}"},
		{name: "true double quoted", yaml: `{"true": a}`, want: "{'true': 'a'}"},
		{name: "capitalized true", yaml: "{True: a}", want: "{True: 'a'}"},
		{name: "capitalized true quoted", yaml: "{'True': a}", want: "{'True': 'a'}"},
		{name: "false", yaml: "{False: a}", want: "{False: 'a'}"},
		{name: "false quoted", yaml: "{'False': a}", want: "{'False': 'a'}"},

		{name: "null", yaml: "{null: a}", want: "{None: 'a'}"},
		{name: "null quoted", yaml: "{'null': a}", want: "{'null': 'a'}"},
		{name: "tilde", yaml: "{~: a}", want: "{None: 'a'}"},
		{name: "tilde quoted", yaml: "{'~': a}", want: "{'~': 'a'}"},

		// YAML 1.2 words that are *not* bools, and a date that is not a date:
		// the two spellings agree, and they agree because the bare one is
		// already a string.
		{name: "yes", yaml: "{yes: a}", want: "{'yes': 'a'}"},
		{name: "yes quoted", yaml: "{'yes': a}", want: "{'yes': 'a'}"},
		{name: "on", yaml: "{ON: a}", want: "{'ON': 'a'}"},
		{name: "on quoted", yaml: "{'ON': a}", want: "{'ON': 'a'}"},
		{name: "date", yaml: "{2024-01-01: a}", want: "{'2024-01-01': 'a'}"},
		{name: "date quoted", yaml: "{'2024-01-01': a}", want: "{'2024-01-01': 'a'}"},

		{name: "word", yaml: "{english: a}", want: "{'english': 'a'}"},
		{name: "word quoted", yaml: "{'english': a}", want: "{'english': 'a'}"},
		{name: "empty single quoted", yaml: "{'': a}", want: "{'': 'a'}"},
		{name: "empty double quoted", yaml: `{"": a}`, want: "{'': 'a'}"},

		{name: "mixed keys", yaml: "{a: 1, 2: b}", want: "{'a': 1, 2: 'b'}"},
		{name: "mixed keys quoted", yaml: "{'a': 1, '2': b}", want: "{'a': 1, '2': 'b'}"},

		// Block spelling, the shape the document a user writes actually has.
		{name: "integer key in block style", yaml: "\n  1: a", want: "{1: 'a'}"},
		{name: "two integer keys in block style", yaml: "\n  1: a\n  2: b", want: "{1: 'a', 2: 'b'}"},
		{name: "quoted key in block style", yaml: "\n  \"1\": a", want: "{'1': 'a'}"},

		// A container key is a **tuple** upstream and never reaches the
		// renderer here: goccy refuses the document with `found an invalid key
		// for this map`, so no node exists to render. Spec 015 delta §4.2 — a
		// parser-level divergence, recorded rather than
		// worked around.
		{
			name: "sequence key", yaml: "{[1]: a}", want: "{(1,): 'a'}",
			unrepresentable: "goccy refuses a collection as a mapping key",
		},
		{
			name: "sequence key of two", yaml: "{[1, 2]: a}", want: "{(1, 2): 'a'}",
			unrepresentable: "goccy refuses a collection as a mapping key",
		},
		{
			name: "empty sequence key", yaml: "{[]: a}", want: "{(): 'a'}",
			unrepresentable: "goccy refuses a collection as a mapping key",
		},
	}

	runKeyReprCases(t, cases)
}

// TestTaggedMappingKeyRepr pins spec 015 delta §4.1: a **tagged** key, where
// the two findings meet.
//
// The tag decides which one applies. An unforced tag leaves ruamel a
// `TaggedScalar`, which reprs as its constructor call; a forced one constructs
// the value, and the value's own kind decides — so `{!!int 1: a}` is `{1: 'a'}`
// exactly as a bare `{1: a}` is.
//
// **No code was written for this.** `Item.KeyNode` is built from the key with
// its tag still on it, by the path a value takes, so both halves fall out of
// the tag fix and the key fix together. This test exists to say so, and to
// fail if either one is later narrowed.
func TestTaggedMappingKeyRepr(t *testing.T) {
	cases := []keyReprCase{
		// Unforced: a TaggedScalar, whose repr is a constructor call.
		{
			name: "string tag", yaml: "{!!str k: a}",
			want: "{TaggedScalar(value='k', style=None, tag=Tag('tag:yaml.org,2002:str')): 'a'}",
		},
		{
			name: "local tag", yaml: "{!unknown k: a}",
			want: "{TaggedScalar(value='k', style=None, tag=Tag('!unknown')): 'a'}",
		},
		{
			name: "quoted under a tag", yaml: `{!!str "k": a}`,
			want: `{TaggedScalar(value='k', style='"', tag=Tag('tag:yaml.org,2002:str')): 'a'}`,
		},

		// Forced: the constructed value, tag gone from the repr.
		{name: "integer tag", yaml: "{!!int 1: a}", want: "{1: 'a'}"},
		{name: "boolean tag", yaml: "{!!bool yes: a}", want: "{True: 'a'}"},
		{name: "float tag", yaml: "{!!float 1.50: a}", want: "{1.5: 'a'}"},
		{name: "null tag", yaml: "{!!null x: a}", want: "{None: 'a'}"},
	}

	runKeyReprCases(t, cases)
}

// keyReprCase is one measured shape of `locale.language`.
type keyReprCase struct {
	name string
	yaml string
	want string
	// unrepresentable names the reason the port cannot produce want. The
	// measured value stays in the table rather than being deleted, so the
	// evidence survives the limitation.
	unrepresentable string
}

func runKeyReprCases(t *testing.T, cases []keyReprCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.unrepresentable != "" {
				t.Skipf("upstream prints %s; %s", c.want, c.unrepresentable)
			}
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
