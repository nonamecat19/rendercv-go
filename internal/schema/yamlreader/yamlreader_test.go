package yamlreader_test

import (
	"errors"
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
