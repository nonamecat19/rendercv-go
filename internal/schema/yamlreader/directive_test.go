package yamlreader_test

import (
	"errors"
	"testing"

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
