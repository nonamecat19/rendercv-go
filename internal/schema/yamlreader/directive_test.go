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

// A `%YAML` directive whose version is not `<digits>.<digits>` never reaches
// ruamel's *parser* at all: its scanner refuses the line while reading it
// (`ruamel/yaml/scanner.py:892-937`, `:978-995`), so upstream reports
// `while scanning a directive` and exits 1.
//
// **goccy's taxonomy does not cover this.** It accepts a bare `%YAML` with no
// version silently — the port rendered `%YAML\n---\n<cv>` at exit 0 where
// upstream exits 1 — and it lumps every other malformed spelling in with an
// out-of-range one under `unknown YAML version`, which the port mapped to
// ruamel's *parser* message `found incompatible YAML document`. Both are the
// wrong verdict, so the syntax is checked against ruamel's own rule on the
// source before goccy sees it.
//
// The rule, transcribed from the scanner: optional spaces, one or more digits,
// a literal `.`, one or more digits, then either the end of the line or spaces
// followed by the end of the line or a `#` comment. Nothing else about the
// numbers matters here — `01.2` and `1.02` are well formed and `10.2` is too
// (it is the *parser* that later rejects major 10). Measured against the
// vendored CLI on every row below.
func TestAMalformedYamlDirectiveVersionIsRefused(t *testing.T) {
	malformed := []struct{ name, yaml string }{
		{name: "no version at all", yaml: "%YAML\n---\nk: v\n"},
		{name: "major only", yaml: "%YAML 1\n---\nk: v\n"},
		{name: "not numeric", yaml: "%YAML x.y\n---\nk: v\n"},
		{name: "trailing dot", yaml: "%YAML 1.\n---\nk: v\n"},
		{name: "no major", yaml: "%YAML .2\n---\nk: v\n"},
		{name: "three parts", yaml: "%YAML 1.2.3\n---\nk: v\n"},
		{name: "trailing junk", yaml: "%YAML 1.2 x\n---\nk: v\n"},
	}

	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)

			var directive *yamlreader.MalformedDirectiveError
			if !errors.As(err, &directive) {
				t.Fatalf("ReadString err = %v, want MalformedDirectiveError", err)
			}
			if directive.Line != 1 {
				t.Errorf("Line = %d, want 1", directive.Line)
			}
			if directive.Error() != "while scanning a directive" {
				t.Errorf("Error() = %q, want %q", directive.Error(), "while scanning a directive")
			}
		})
	}

	// The spellings the scanner accepts. `1.1` is refused further along by
	// D-018, so it cannot be asserted to load; what is asserted is that the
	// *scanner* let it past.
	wellFormed := []struct{ name, yaml string }{
		{name: "the ordinary spelling", yaml: "%YAML 1.2\n---\nk: v\n"},
		{name: "extra leading spaces", yaml: "%YAML  1.2\n---\nk: v\n"},
		{name: "a trailing comment", yaml: "%YAML 1.2 # hi\n---\nk: v\n"},
		{name: "trailing spaces", yaml: "%YAML 1.2  \n---\nk: v\n"},
		{name: "leading zero in major", yaml: "%YAML 01.2\n---\nk: v\n"},
		{name: "leading zero in minor", yaml: "%YAML 1.02\n---\nk: v\n"},
		{name: "a two-digit major", yaml: "%YAML 10.2\n---\nk: v\n"},
		{name: "a version this port refuses later", yaml: "%YAML 1.1\n---\nk: v\n"},
	}

	for _, test := range wellFormed {
		t.Run(test.name, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)

			var directive *yamlreader.MalformedDirectiveError
			if errors.As(err, &directive) {
				t.Fatalf("ReadString err = %v, want no MalformedDirectiveError", err)
			}
		})
	}

	// A directive that is not `%YAML` carries no version to scan, and ruamel
	// drops an unrecognised one whole.
	for _, yaml := range []string{"%TAG !e! a:\n---\nk: v\n", "%FOO bar\n---\nk: v\n"} {
		if _, err := yamlreader.ReadString(yaml); err != nil {
			t.Errorf("ReadString(%q) = %v, want nil", yaml, err)
		}
	}
}

// D-014 — a well-formed `%YAML 1.<n>` whose minor part is neither 1 nor 2.
//
// ruamel's parser lets it past: `process_directives` checks only the major part
// (`ruamel/yaml/parser.py:296-304`). The failure comes one line later, at
// `self.loader.version = yaml_version` (`:321`), where the setter asserts
// (`ruamel/yaml/main.py:849-851`). Nothing catches an `AssertionError`, so
// upstream exits 1 with a Rich traceback on stderr and nothing on stdout —
// measured at 0 bytes stdout / 12958 bytes stderr for `%YAML 1.3`.
//
// The traceback is D-014's class and is not reproducible. The exit code and the
// refusal are, so the port reports the nearest clean error: upstream's own
// assertion text, which is the one deterministic line of that traceback, as a
// validation record. The port rendered these at exit 0 before.
//
// **The split is by minor part only, once the major part is 1.** Measured on
// every row: 1.1 and 1.2 load, 1.0, 1.3, 1.9, 1.10 and 1.20 assert.
func TestAnOutOfRangeYamlMinorVersionIsRefused(t *testing.T) {
	tests := []struct{ yaml, directive, tuple string }{
		{yaml: "%YAML 1.0\n---\nk: v\n", directive: "%YAML 1.0", tuple: "(1, 0)"},
		{yaml: "%YAML 1.3\n---\nk: v\n", directive: "%YAML 1.3", tuple: "(1, 3)"},
		{yaml: "%YAML 1.9\n---\nk: v\n", directive: "%YAML 1.9", tuple: "(1, 9)"},
		{yaml: "%YAML 1.10\n---\nk: v\n", directive: "%YAML 1.10", tuple: "(1, 10)"},
		{yaml: "%YAML 1.20\n---\nk: v\n", directive: "%YAML 1.20", tuple: "(1, 20)"},
	}

	for _, test := range tests {
		t.Run(test.directive, func(t *testing.T) {
			_, err := yamlreader.ReadString(test.yaml)

			var version *yamlreader.UnsupportedVersionError
			if !errors.As(err, &version) {
				t.Fatalf("ReadString err = %v, want UnsupportedVersionError", err)
			}
			if version.Line != 1 {
				t.Errorf("Line = %d, want 1", version.Line)
			}
			want := "The '" + test.directive + "' directive is not supported: version minor" +
				" part can only be 2 or 1, got " + test.tuple + "."
			if version.Error() != want {
				t.Errorf("Error() =\n  %q\nwant\n  %q", version.Error(), want)
			}
		})
	}

	// The minor parts ruamel's setter accepts, and every major part it rejects
	// before reaching the setter at all — a wrong major is its parser's
	// `found incompatible YAML document`, not this assertion.
	for _, yaml := range []string{
		"%YAML 1.1\n---\nk: v\n", "%YAML 1.2\n---\nk: v\n",
		"%YAML 0.1\n---\nk: v\n", "%YAML 2.0\n---\nk: v\n",
		"%YAML 3.0\n---\nk: v\n", "%YAML 10.2\n---\nk: v\n",
	} {
		var version *yamlreader.UnsupportedVersionError
		if _, err := yamlreader.ReadString(yaml); errors.As(err, &version) {
			t.Errorf("ReadString(%q) = %v, want no UnsupportedVersionError", yaml, err)
		}
	}
}
