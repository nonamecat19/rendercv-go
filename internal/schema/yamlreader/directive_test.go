package yamlreader_test

import (
	"errors"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// A directive-headed document loads exactly as the same document without the
// directive. Upstream inherits that from ruamel, which processes `%YAML` and
// `%TAG` and silently drops anything else
// (`ruamel/yaml/parser.py:288-330`); measured through the vendored
// `read_yaml`, `%YAML 1.2`, `%TAG !e! tag:example.com,2000:` and `%FOO bar`
// all load their document unchanged.
//
// The port rejected every one of them with `expected a single document in the
// stream`, because goccy makes the directive line a document of its own —
// spec-delta-directives §4.4.2.
func TestADirectiveHeadedDocumentLoadsNormally(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "no directive", yaml: "---\nk: v\n"},
		{name: "yaml version", yaml: "%YAML 1.2\n---\nk: v\n"},
		{name: "tag named handle", yaml: "%TAG !e! tag:example.com,2000:\n---\nk: v\n"},
		{name: "tag primary handle", yaml: "%TAG !! tag:example.com,2000:\n---\nk: v\n"},
		{name: "tag default handle", yaml: "%TAG ! tag:example.com,2000:\n---\nk: v\n"},
		{name: "unrecognised directive", yaml: "%FOO bar\n---\nk: v\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString(test.yaml)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			if len(doc.Items) != 1 || doc.Items[0].Key != "k" {
				t.Fatalf("document = %+v, want the single key k", doc.Items)
			}
			if got := doc.Items[0].Value.Raw; got != "v" {
				t.Errorf("k = %q, want %q", got, "v")
			}
		})
	}
}

// The predicate is "the document is only a directive", not "the document has
// no body": a genuine two-document stream must keep failing, including one
// whose second document is empty. Measured — `---\nk: v\n---\n` is ruamel's
// `ComposerError: but found another document`.
func TestAGenuineMultiDocumentStreamStillFails(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "two documents", yaml: "---\nk: v\n---\nj: w\n"},
		{name: "empty second document", yaml: "---\nk: v\n---\n"},
		{name: "directive then two documents", yaml: "%YAML 1.2\n---\nk: v\n---\nj: w\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)
			var multiDoc *yamlreader.MultiDocumentError
			if !errors.As(err, &multiDoc) {
				t.Fatalf("ReadString err = %v, want MultiDocumentError", err)
			}
		})
	}
}

// `%TAG` rebinds a handle for the document that carries it, and the rebinding
// reaches the tag a `TaggedScalar`'s repr names — ruamel's `trval` is
// `handles[handle] + uri_decode(suffix)` over a per-document table
// (`ruamel/yaml/tag.py:55-88`, `ruamel/yaml/parser.py:106`, `:327-329`).
//
// **goccy hands the handle back unexpanded**, so the expansion is the port's.
// Every row was measured through the vendored `read_yaml`.
func TestATagDirectiveRebindsAHandle(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		kind yamldoc.Kind
		tag  string
		raw  string
	}{
		{
			name: "primary handle rebound",
			yaml: "%TAG !! tag:example.com,2000:\n---\nk: !!str x\n",
			kind: yamldoc.KindTagged, tag: "tag:example.com,2000:str", raw: "x",
		},
		{
			name: "named handle",
			yaml: "%TAG !e! tag:example.com,2000:\n---\nk: !e!x v\n",
			kind: yamldoc.KindTagged, tag: "tag:example.com,2000:x", raw: "v",
		},
		{
			name: "default handle rebound",
			yaml: "%TAG ! tag:example.com,2000:\n---\nk: !foo v\n",
			kind: yamldoc.KindTagged, tag: "tag:example.com,2000:foo", raw: "v",
		},
		{
			name: "suffix is URI-decoded after expansion",
			yaml: "%TAG !e! tag:example.com,2000:\n---\nk: !e!a%21b v\n",
			kind: yamldoc.KindTagged, tag: "tag:example.com,2000:a!b", raw: "v",
		},

		// **The constructor is looked up by the resolved tag, not the written
		// one.** A rebound `!!` takes `!!int` away from ruamel's int
		// constructor, and a named handle bound to `tag:yaml.org,2002:` gives
		// it back.
		{
			name: "rebound primary handle loses the int constructor",
			yaml: "%TAG !! tag:example.com,2000:\n---\nk: !!int 1\n",
			kind: yamldoc.KindTagged, tag: "tag:example.com,2000:int", raw: "1",
		},
		{
			name: "named handle reaches the int constructor",
			yaml: "%TAG !e! tag:yaml.org,2002:\n---\nk: !e!int 5\n",
			kind: yamldoc.KindInt, raw: "5",
		},
		{
			name: "a verbatim tag ignores the handles",
			yaml: "%TAG ! tag:example.com,2000:\n---\nk: !<tag:yaml.org,2002:int> 7\n",
			kind: yamldoc.KindInt, raw: "7",
		},

		// A bare `!` is the non-specific tag, not the `!` handle, so rebinding
		// `!` does not touch it: `! x` is the plain string `x`.
		{
			name: "the non-specific tag is unaffected",
			yaml: "%TAG ! tag:example.com,2000:\n---\nk: ! x\n",
			kind: yamldoc.KindString, raw: "x",
		},

		// The defaults still apply where no directive rebinds them.
		{
			name: "no directive keeps DEFAULT_TAGS",
			yaml: "k: !!str x\n",
			kind: yamldoc.KindTagged, tag: "tag:yaml.org,2002:str", raw: "x",
		},
		{
			name: "an unrelated handle leaves the defaults alone",
			yaml: "%TAG !e! tag:example.com,2000:\n---\nk: !!str x\n",
			kind: yamldoc.KindTagged, tag: "tag:yaml.org,2002:str", raw: "x",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString(test.yaml)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := doc.Items[0].Value
			if node.Kind != test.kind {
				t.Errorf("Kind = %v, want %v", node.Kind, test.kind)
			}
			if node.Tag != test.tag {
				t.Errorf("Tag = %q, want %q", node.Tag, test.tag)
			}
			if node.Raw != test.raw {
				t.Errorf("Raw = %q, want %q", node.Raw, test.raw)
			}
		})
	}
}

// D-018 — the port has no YAML 1.1 scalar resolver, so it refuses the
// directive by name instead of resolving `yes` as a bool the way upstream
// would. Every other `%YAML` version the parser accepts is left alone.
func TestYamlOnePointOneIsRefusedByName(t *testing.T) {
	_, err := yamlreader.ReadString("%YAML 1.1\n---\nk: yes\n")

	var directive *yamlreader.UnsupportedDirectiveError
	if !errors.As(err, &directive) {
		t.Fatalf("ReadString err = %v, want UnsupportedDirectiveError", err)
	}
	if directive.Directive != "%YAML 1.1" {
		t.Errorf("Directive = %q, want %q", directive.Directive, "%YAML 1.1")
	}
	if directive.Line != 1 {
		t.Errorf("Line = %d, want 1", directive.Line)
	}

	if _, err := yamlreader.ReadString("%YAML 1.2\n---\nk: yes\n"); err != nil {
		t.Errorf("%%YAML 1.2 must still load: %v", err)
	}
}
