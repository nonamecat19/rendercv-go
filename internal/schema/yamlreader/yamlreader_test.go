package yamlreader_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// Spec §4.1 — a nonexistent path, with the path interpolated exactly as
// supplied rather than resolved.
func TestNonexistentFile(t *testing.T) {
	for _, path := range []string{"does_not_exist.yaml", "./nested/does_not_exist.yaml"} {
		t.Run(path, func(t *testing.T) {
			_, err := yamlreader.ReadFile(path)

			var userErr *schemaerr.UserError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserError", err, err)
			}
			want := "The input file `" + path + "` doesn't exist!"
			if userErr.Message != want {
				t.Errorf("message = %q, want %q", userErr.Message, want)
			}
		})
	}
}

// Spec §4.2 — the extension check, on the file's final component.
func TestExtensionCheck(t *testing.T) {
	dir := t.TempDir()
	rejected := []string{"cv.txt", "cv.YAML", "cv.yamls", "cv"}
	for _, name := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			write(t, path, "cv:\n  name: John\n")

			_, err := yamlreader.ReadFile(path)
			var userErr *schemaerr.UserError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserError", err, err)
			}
			want := "The input file should have one of the following extensions:" +
				" .yaml, .yml, .json, .json5. The input file is " + name + "."
			if userErr.Message != want {
				t.Errorf("message = %q, want %q", userErr.Message, want)
			}
		})
	}
	for _, name := range []string{"cv.yaml", "cv.yml", "cv.json", "cv.json5"} {
		t.Run("accepts "+name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			write(t, path, "cv:\n  name: John\n")

			if _, err := yamlreader.ReadFile(path); err != nil {
				t.Errorf("ReadFile(%s) = %v, want nil", name, err)
			}
		})
	}
}

// Spec §5.1 — the extension check runs before the empty check, so a zero-byte
// `x.txt` reports the extension error, not the empty-file one.
func TestExtensionBeatsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	write(t, path, "")

	_, err := yamlreader.ReadFile(path)
	var userErr *schemaerr.UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserError", err, err)
	}
	want := "The input file should have one of the following extensions:" +
		" .yaml, .yml, .json, .json5. The input file is x.txt."
	if userErr.Message != want {
		t.Errorf("message = %q, want the extension error %q", userErr.Message, want)
	}
}

// Spec §4.3 — a zero-byte file with an accepted extension.
func TestEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.yaml")
	write(t, path, "")

	_, err := yamlreader.ReadFile(path)
	var userErr *schemaerr.UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserError", err, err)
	}
	if userErr.Message != "The input file is empty!" {
		t.Errorf("message = %q, want %q", userErr.Message, "The input file is empty!")
	}
}

// Spec §4.4 — a document whose root is a scalar string is the internal error,
// with the string interpolated.
func TestScalarStringRoot(t *testing.T) {
	const input = "path/to/cv.yaml"
	_, err := yamlreader.ReadString(input)

	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err = %v (%T), want *schemaerr.InternalError", err, err)
	}
	want := "You probably meant to pass a path to the YAML file, but you passed as a" +
		" string and RenderCV interpreted it as the contents of the YAML file. Pass" +
		" the path using `pathlib.Path(" + input + ")`."
	if internal.Message != want {
		t.Errorf("message = %q, want %q", internal.Message, want)
	}
}

// Spec §5.3 — the seven alias and anchor cases parse to their recorded values.
// The token-level assertions live in noalias_test.go; these constrain the tree.
func TestAliasAndAnchorCasesParseToValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, doc *yamldoc.Node)
	}{
		{
			name:  "bare star is a string",
			input: "key: *not_an_alias\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				assertScalar(t, doc, "key", "*not_an_alias")
			},
		},
		{
			name:  "star inside text",
			input: "mixed: *a and more\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				assertScalar(t, doc, "mixed", "*a and more")
			},
		},
		{
			name:  "stars in a sequence",
			input: "multi:\n  - *one\n  - *two\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				seq := value(t, doc, "multi")
				if len(seq.Elems) != 2 {
					t.Fatalf("multi = %+v, want two elements", seq.Elems)
				}
				if seq.Elems[0].Raw != "*one" || seq.Elems[1].Raw != "*two" {
					t.Errorf("multi = [%q %q], want [*one *two]", seq.Elems[0].Raw, seq.Elems[1].Raw)
				}
			},
		},
		{
			name:  "star nested in a mapping",
			input: "nested:\n  inner: *deep_value\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				assertScalar(t, value(t, doc, "nested"), "inner", "*deep_value")
			},
		},
		{
			name:  "emphasis in a highlight",
			input: "highlights:\n  - normal *star* here\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				seq := value(t, doc, "highlights")
				if len(seq.Elems) != 1 || seq.Elems[0].Raw != "normal *star* here" {
					t.Errorf("highlights = %+v, want [normal *star* here]", seq.Elems)
				}
			},
		},
		{
			name:  "quoted star",
			input: "b: '*quoted'\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				assertScalar(t, doc, "b", "*quoted")
			},
		},
		{
			name:  "star in a literal block",
			input: "block: |\n  a *literal* block\n",
			check: func(t *testing.T, doc *yamldoc.Node) {
				got := value(t, doc, "block")
				if got.Kind != yamldoc.KindString {
					t.Errorf("block kind = %v, want string", got.Kind)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString(tc.input)
			if err != nil {
				t.Fatalf("ReadString(%q) = %v", tc.input, err)
			}
			tc.check(t, doc)
		})
	}
}

// Spec §3.10a, §5.3 — the anchor half: an anchor node is unwrapped to its
// value, while the alias that references it stays a literal string. This has no
// upstream test and is its own acceptance criterion.
func TestAnchorIsUnwrappedAndAliasStaysAString(t *testing.T) {
	doc, err := yamlreader.ReadString("real_anchor: &anchor value\nuse: *anchor\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	assertScalar(t, doc, "real_anchor", "value")
	assertScalar(t, doc, "use", "*anchor")
}

// Spec §5.4 — timestamps stay strings; they are deliberately unresolved.
func TestTimestampsStayStrings(t *testing.T) {
	tests := []struct {
		key   string
		input string
	}{
		{key: "d", input: "d: 2020-09-24\n"},
		{key: "t", input: "t: 2020-09-24T10:00:00Z\n"},
		{key: "m", input: "m: 2020-09\n"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			doc, err := yamlreader.ReadString(tc.input)
			if err != nil {
				t.Fatalf("ReadString = %v", err)
			}
			got := value(t, doc, tc.key)
			if got.Kind != yamldoc.KindString {
				t.Errorf("%s kind = %v, want string", tc.key, got.Kind)
			}
		})
	}
}

// A document's key order round-trips, whatever order it was written in.
func TestKeyOrderRoundTrips(t *testing.T) {
	doc, err := yamlreader.ReadString("zebra: 1\nalpha: 2\nmiddle: 3\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	want := []string{"zebra", "alpha", "middle"}
	if len(doc.Items) != len(want) {
		t.Fatalf("keys = %+v, want %v", doc.Items, want)
	}
	for i, key := range want {
		if doc.Items[i].Key != key {
			t.Fatalf("key[%d] = %q, want %q", i, doc.Items[i].Key, key)
		}
	}
}

// A quoted key is the same key. The reader took mapping keys from the node's
// source form, which kept the quotes, so `"name": John` bound a field called
// `"name"` — which no model declares, so it was reported as an unknown key
// instead of binding. Scalar *values* never had the problem; they already read
// the token.
//
// Upstream's ruamel unquotes both styles: `{"name": …, 'email': …}` reads back
// as `['name', 'email']` (measured against the vendored Python).
func TestQuotedKeysAreUnquoted(t *testing.T) {
	doc, err := yamlreader.ReadString("\"name\": John\n'email': a@b.com\nplain: 1\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	want := []string{"name", "email", "plain"}
	if len(doc.Items) != len(want) {
		t.Fatalf("keys = %+v, want %v", doc.Items, want)
	}
	for i, key := range want {
		if doc.Items[i].Key != key {
			t.Errorf("key[%d] = %q, want %q", i, doc.Items[i].Key, key)
		}
	}
}

func value(t *testing.T, node *yamldoc.Node, key string) *yamldoc.Node {
	t.Helper()
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value
		}
	}
	t.Fatalf("key %q not found in %+v", key, node.Items)
	return nil
}

func assertScalar(t *testing.T, node *yamldoc.Node, key, want string) {
	t.Helper()
	if got := value(t, node, key).Raw; got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// An empty quoted scalar is the empty string, not its own quote characters.
//
// goccy leaves a token's `Value` empty for two different reasons — the value is
// empty, or the token carries no value at all — and the fallback to `Origin`
// conflated them. `degree: ""` read back as the two-character string `""`,
// which compares unequal to `""` everywhere downstream: it is a non-empty value
// to `render_entry_templates`' empty-value filter, so the surrounding formatting
// it would have cleaned up survives, and the quotes themselves reach the
// artifact. Found while building the entry dump of spec 009 T1.
func TestEmptyQuotedScalarIsEmpty(t *testing.T) {
	doc, err := yamlreader.ReadString("double: \"\"\nsingle: ''\nplain: x\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	assertScalar(t, doc, "double", "")
	assertScalar(t, doc, "single", "")
	assertScalar(t, doc, "plain", "x")
}

// A block scalar carries its **body**, not its indicator.
//
// `buildLiteral` read `n.Start` — the `|` token — so every block scalar in every
// CV became the literal string `" |\n"`, and that reached the `.typ`, the `.md`
// and the `.html`. A test here fed a literal block and asserted only its Kind,
// so it passed on the garbage; the scalar corpus has no block entry and no
// golden CV uses one, which is why a green parity run said nothing about it.
func TestBlockScalarsCarryTheirBody(t *testing.T) {
	document := "literal: |\n  line one\n  line two\n" +
		"folded: >\n  line one\n  line two\n" +
		"stripped: |-\n  line one\n  line two\n" +
		"foldedstripped: >-\n  line one\n  line two\n"

	doc, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}

	// The four forms differ only in folding and chomping, which goccy has
	// already applied — so these are ruamel's values, measured.
	assertScalar(t, doc, "literal", "line one\nline two\n")
	assertScalar(t, doc, "folded", "line one line two\n")
	assertScalar(t, doc, "stripped", "line one\nline two")
	assertScalar(t, doc, "foldedstripped", "line one line two")
}

// The same in a sequence, which is how a `TextEntry` writes one.
func TestBlockScalarsInASequence(t *testing.T) {
	doc, err := yamlreader.ReadString("notes:\n  - |\n    first\n    second\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	entry := value(t, doc, "notes").Elems[0]
	if entry.Raw != "first\nsecond\n" {
		t.Errorf("= %q, want the block's body", entry.Raw)
	}
}

// TestExplicitNullDocumentIsEmpty pins upstream's actual predicate. Its check
// is `yaml.load(file_content) is None` (`yaml_reader.py:55-57`), so a document
// whose whole value is an explicit null loads to `None` exactly as a zero-byte
// file does and reports the same `RenderCVUserError`.
//
// The port keyed on the *absence* of a document instead, so `null` fell
// through to the model builder and reached the user as a validation table
// (1504 bytes) where upstream prints the 553-byte `Error` panel. All six
// spellings below are byte-identical against the vendored CLI after the fix.
func TestExplicitNullDocumentIsEmpty(t *testing.T) {
	for _, src := range []string{"null\n", "~\n", "Null\n", "NULL\n", "# comment\n", ""} {
		t.Run(strings.TrimSpace(src), func(t *testing.T) {
			_, err := yamlreader.ReadString(src)

			var userErr *schemaerr.UserError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserError", err, err)
			}
			if userErr.Message != "The input file is empty!" {
				t.Errorf("message = %q, want %q", userErr.Message, "The input file is empty!")
			}
		})
	}
}

// TestTabIndentedQuotedContinuation pins a valid document the port used to
// reject.
//
// A tab is an error nearly everywhere in YAML, but **not inside a quoted
// scalar**: ruamel loads `name: "a\n\tb"` and upstream renders a CV from it,
// while goccy's scanner refuses the tab and the port exited 1 on a valid input
// file.
//
// The leading whitespace of a continuation line is folded away, so a tab and a
// space yield the same value — which is what makes the substitution safe. Only
// the lines goccy itself names are touched, so a document that already parses
// is never rewritten.
func TestTabIndentedQuotedContinuation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "double-quoted", src: "cv:\n  name: \"a\n\tb\"\n", want: "a b"},
		{name: "single-quoted", src: "cv:\n  name: 'a\n\tb'\n", want: "a b"},
		{name: "a tab then a space", src: "cv:\n  name: \"a\n\t b\"\n", want: "a b"},
		{name: "several continuations", src: "cv:\n  name: \"a\n\tb\n\tc\"\n", want: "a b c"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node, err := yamlreader.ReadString(test.src)
			if err != nil {
				t.Fatalf("err = %v, want the document to parse", err)
			}

			cv := node.Items[0].Value
			if got := cv.Items[0].Value.Raw; got != test.want {
				t.Errorf("name = %q, want %q — folding must give the same value a space would", got, test.want)
			}
		})
	}
}

// A tab that is *not* quoted-scalar indentation must still be the error it
// always was, so the retry cannot quietly accept what upstream rejects.
func TestTabOutsideAQuotedScalarStillFails(t *testing.T) {
	for _, src := range []string{"cv:\n\tname: a\n", "\ta: 1\n"} {
		t.Run(src, func(t *testing.T) {
			if _, err := yamlreader.ReadString(src); err == nil {
				t.Error("err = nil, want a parse failure")
			}
		})
	}
}

// TestTabsOutsideTheLegalRegions pins where YAML forbids a tab.
//
// **goccy accepts a tab in seven of these positions and ruamel rejects all of
// them**, so the port rendered documents upstream refuses at exit 1. A tab is
// legal only inside a quoted scalar, a block scalar's content, a comment's
// text, or a flow collection; anywhere else it is separation whitespace, which
// YAML does not allow to be a tab.
//
// Every row was measured against ruamel and then end-to-end against the
// vendored CLI, where all eighteen shapes here and in the companion test are
// now byte-identical.
func TestTabsOutsideTheLegalRegions(t *testing.T) {
	rejected := map[string]string{
		"indenting a key":          "cv:\n\tname: a\n",
		"at the root":              "\ta: 1\n",
		"after a sequence dash":    "cv:\n  -\ta\n",
		"after a colon":            "cv:\tvalue\n",
		"after a nested colon":     "cv:\n  name:\tJohn\n",
		"inside a plain scalar":    "cv:\n  name: a\tb\n",
		"a plain continuation":     "cv:\n  name: x\n\ty\n",
		"trailing":                 "cv:\n  name: a\t\n",
		"on a blank line":          "cv:\n  name: a\n\t\nx: 1\n",
		"under a block scalar":     "cv:\n  name: |\n    x\n\ty\n",
		"before a comment":         "cv:\n  name: a\t# c\n",
		"before a flow collection": "cv:\n  x:\t[a]\n",
	}

	for name, src := range rejected {
		t.Run(name, func(t *testing.T) {
			var tabErr *yamlreader.TabError
			if _, err := yamlreader.ReadString(src); !errors.As(err, &tabErr) {
				t.Fatalf("err = %v (%T), want *yamlreader.TabError", err, err)
			}
		})
	}
}

// The four regions where a tab is ordinary content. These are the rows that
// make the check above safe to have: a rule that rejected any of them would
// refuse valid CVs, which is worse than the divergence it closes.
func TestTabsInsideTheLegalRegions(t *testing.T) {
	accepted := map[string]string{
		"a quoted continuation":   "cv:\n  name: \"a\n\tb\"\n",
		"inside a quoted scalar":  "cv:\n  name: \"a\tb\"\n",
		"in block scalar content": "cv:\n  name: |\n    a\tb\n",
		"in a comment":            "# a\tb\ncv:\n  name: John\n",
		"in a flow collection":    "cv:\n  x: [a,\tb]\n",
	}

	for name, src := range accepted {
		t.Run(name, func(t *testing.T) {
			if _, err := yamlreader.ReadString(src); err != nil {
				t.Errorf("err = %v, want the document to parse", err)
			}
		})
	}
}

// A tag on a collection is transparent — spec 015 §5.1. ruamel's
// `construct_unknown` branches on the *node's shape*, not on the tag
// (`ruamel/yaml/constructor.py:1598-1610`), so a mapping stays a
// `CommentedMap` and a sequence a `CommentedSeq` **even when the tag names a
// scalar type**: `!!str [1,2]` is a sequence upstream.
//
// The reader had no tag case at all, so every one of these built a null and
// the document lost its whole `cv` block — `cv: !!map` renders upstream and
// was refused here.
//
// `!!str` (and any other *known scalar* tag) over a collection is absent on
// purpose: goccy's parser refuses the document outright — `unexpected scalar
// value type`, in both the flow and the block spelling — so it is not
// something this case can reach, and it is recorded as a divergence instead.
func TestTagOnACollectionIsTransparent(t *testing.T) {
	tests := []struct {
		name     string
		tagged   string
		untagged string
	}{
		{
			name:     "mapping",
			tagged:   "cv: !!map\n  name: John\n",
			untagged: "cv:\n  name: John\n",
		},
		{
			name:     "sequence",
			tagged:   "cv: !!seq\n  - a\n  - b\n",
			untagged: "cv:\n  - a\n  - b\n",
		},
		{
			name:     "flow mapping",
			tagged:   "cv: !!map {name: John}\n",
			untagged: "cv: {name: John}\n",
		},
		{
			name:     "an unknown tag on a mapping",
			tagged:   "cv: !whatever\n  name: John\n",
			untagged: "cv:\n  name: John\n",
		},
		{
			name:     "a nested tagged mapping",
			tagged:   "cv:\n  sections:\n    experience:\n      - !!map\n        company: Acme\n",
			untagged: "cv:\n  sections:\n    experience:\n      - company: Acme\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tagged, err := yamlreader.ReadString(tc.tagged)
			if err != nil {
				t.Fatalf("ReadString(tagged) = %v", err)
			}
			untagged, err := yamlreader.ReadString(tc.untagged)
			if err != nil {
				t.Fatalf("ReadString(untagged) = %v", err)
			}
			if got := kindTree(value(t, tagged, "cv")); got != kindTree(value(t, untagged, "cv")) {
				t.Errorf("tagged tree = %s, want the untagged %s",
					got, kindTree(value(t, untagged, "cv")))
			}
		})
	}
}

// A tag on the document root is transparent too, which is why `!!map` at the
// top used to reach the `is None` predicate (`yaml_reader.py:55-57`) and print
// the empty-file panel.
func TestTagOnTheDocumentRootIsTransparent(t *testing.T) {
	doc, err := yamlreader.ReadString("!!map\ncv:\n  name: John\n")
	if err != nil {
		t.Fatalf("ReadString = %v", err)
	}
	if doc.Kind != yamldoc.KindMapping {
		t.Fatalf("root kind = %v, want a mapping", doc.Kind)
	}
	assertScalar(t, value(t, doc, "cv"), "name", "John")
}

// kindTree spells a node's shape and values compactly, so two trees can be
// compared as strings — the point of the tag cases is that the tagged tree is
// indistinguishable from the untagged one.
func kindTree(node *yamldoc.Node) string {
	if node == nil {
		return "<nil>"
	}
	switch node.Kind {
	case yamldoc.KindMapping:
		parts := make([]string, 0, len(node.Items))
		for _, item := range node.Items {
			parts = append(parts, item.Key+":"+kindTree(item.Value))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case yamldoc.KindSequence:
		parts := make([]string, 0, len(node.Elems))
		for _, elem := range node.Elems {
			parts = append(parts, kindTree(elem))
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		return fmt.Sprintf("%d(%q)", node.Kind, node.Raw)
	}
}

// A tag that names a type the loader constructs forces that type on the scalar
// — spec 015 §3.2, measured through upstream's own configured loader
// (`from rendercv.schema.yaml_reader import read_yaml`), never a default
// `ruamel.yaml.YAML()`: the two disagree about `!!timestamp`, and the
// difference is upstream's own override (`yaml_reader.py:83-86`).
func TestATagCanForceAScalarsKind(t *testing.T) {
	tests := []struct {
		input string
		kind  yamldoc.Kind
		raw   string
	}{
		{input: "a: !!int 200\n", kind: yamldoc.KindInt, raw: "200"},
		{input: "a: !!int 0x10\n", kind: yamldoc.KindInt, raw: "0x10"},
		{input: "a: !!int 1_000\n", kind: yamldoc.KindInt, raw: "1_000"},
		{input: "a: !!float 0.5\n", kind: yamldoc.KindFloat, raw: "0.5"},
		// `!!float 1` is a Python float upstream, where the same token
		// untagged is an int.
		{input: "a: !!float 1\n", kind: yamldoc.KindFloat, raw: "1"},
		{input: "a: !!bool true\n", kind: yamldoc.KindBool, raw: "true"},
		// YAML 1.1's spellings, which ruamel still accepts for an explicit
		// `!!bool` (`ruamel/yaml/constructor.py:432-445`) and which a plain
		// scalar does not resolve to a bool at all.
		{input: "a: !!bool yes\n", kind: yamldoc.KindBool, raw: "yes"},
		{input: "a: !!bool off\n", kind: yamldoc.KindBool, raw: "off"},
		{input: "a: !!bool N\n", kind: yamldoc.KindBool, raw: "N"},
		// `!!null` discards the scalar's text: `!!null x` is None upstream.
		{input: "a: !!null ~\n", kind: yamldoc.KindNull, raw: ""},
		{input: "a: !!null x\n", kind: yamldoc.KindNull, raw: ""},
		// Upstream replaces the timestamp constructor with `construct_scalar`,
		// so a tagged ISO date is a plain string — the same answer the reader
		// already gives an untagged one.
		{input: "a: !!timestamp 2001-01-01\n", kind: yamldoc.KindString, raw: "2001-01-01"},
		// A quoted scalar carrying a forcing tag is forced too: the tag wins
		// over the style, which is what makes `!!int` a coercion rather than a
		// resolution hint.
		{input: "a: !!int \"200\"\n", kind: yamldoc.KindInt, raw: "200"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			doc, err := yamlreader.ReadString(tc.input)
			if err != nil {
				t.Fatalf("ReadString = %v", err)
			}
			got := value(t, doc, "a")
			if got.Kind != tc.kind {
				t.Errorf("kind = %d, want %d", got.Kind, tc.kind)
			}
			if got.Raw != tc.raw {
				t.Errorf("raw = %q, want %q", got.Raw, tc.raw)
			}
		})
	}
}
