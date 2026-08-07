//go:build conformance

package jsonschema_test

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

const upstreamSchema = "../../../third_party/rendercv/schema.json"

// The per-`$defs` differential (spec 005 §8, plan §6).
//
// For each entry the port claims, its bytes are compared against upstream's
// **through the same encoder**. Re-serializing upstream's side is deliberate:
// comparing against a substring of the file would be sensitive to where the def
// sits, and comparing parsed values would lose the key order that is half the
// contract. Round-tripping both tests exactly the two things that matter.
func TestDefsMatchUpstream(t *testing.T) {
	upstream := loadUpstreamDefs(t)

	for name, object := range portDefs() {
		t.Run(name, func(t *testing.T) {
			want, ok := upstream[name]
			if !ok {
				t.Fatalf("upstream has no $defs entry %q — the port invented one", name)
			}

			got, err := jsonschema.Marshal(object)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if got != want {
				t.Errorf("=\n%s\n\nwant\n%s", got, want)
			}
		})
	}
}

// The absent set: every `$defs` key upstream has that the port does not.
//
// It is written out rather than counted, so it fails when the list shrinks
// without the models landing — which is what stops iterations 6 and 7 forgetting
// to close Axis 3 (spec 005 §7.1). A shorter list means a model arrived; a longer
// one means the port lost something.
func TestAbsentDefsAreAccountedFor(t *testing.T) {
	upstream := loadUpstreamDefs(t)
	port := portDefs()

	var absent []string
	for name := range upstream {
		if _, ok := port[name]; !ok {
			absent = append(absent, name)
		}
	}
	sort.Strings(absent)

	// 227 upstream, 224 in the port: iterations 2-4's 18, iteration 7's 45
	// locale entries, and iteration 6's 161 design entries. The three left are
	// `Settings`, `RenderCommand` and `PlannedPathRelativeToInput`, which
	// iteration 6 takes as its T15.
	const wantAbsent = 227 - 224
	if len(absent) != wantAbsent {
		t.Errorf("%d $defs are absent, want %d — if a model landed, update this"+
			" count and Axis 3's status in specs/STATE.md", len(absent), wantAbsent)
	}

	// None of the absent ones may be a `cv` or entry model: those are this
	// iteration's and the count above would hide one going missing.
	for _, name := range absent {
		if isOwnedNow(name) {
			t.Errorf("%q is owned by iterations 2-4 and has no schema", name)
		}
	}
}

// portDefs is every `$defs` entry the port can build today.
func portDefs() map[string]*jsonschema.Object {
	defs := entries.SchemaDefs()
	for name, object := range cv.SchemaDefs() {
		defs[name] = object
	}
	for name, object := range locale.SchemaDefs() {
		defs[name] = object
	}
	for name, object := range design.SchemaDefs() {
		defs[name] = object
	}
	return defs
}

// isOwnedNow reports whether a `$defs` key belongs to a model that already
// exists, and is the mirror of spec 005 §1's table.
func isOwnedNow(name string) bool {
	switch name {
	case "ArbitraryDate", "ExactDate", "BulletEntry", "NumberedEntry",
		"ReversedNumberedEntry", "Cv", "CustomConnection", "Section",
		"ListOfEntries", "SocialNetwork", "SocialNetworkName",
		"TypstDimension", "ExistingPathRelativeToInput":
		return true
	}
	return len(name) > 30 && name[:34] == "rendercv__schema__models__cv__entr"
}

// loadUpstreamDefs re-serializes each of upstream's `$defs` through the port's
// encoder, so the comparison is on keys and order rather than on whitespace.
func loadUpstreamDefs(t *testing.T) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(upstreamSchema)
	if err != nil {
		t.Fatalf("reading %s: %v — is the submodule initialized? `just setup`", upstreamSchema, err)
	}

	var document struct {
		Defs json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parsing %s: %v", upstreamSchema, err)
	}

	var defs map[string]json.RawMessage
	if err := json.Unmarshal(document.Defs, &defs); err != nil {
		t.Fatalf("parsing $defs: %v", err)
	}

	out := make(map[string]string, len(defs))
	for name, value := range defs {
		object, err := decodeObject(value)
		if err != nil {
			t.Fatalf("decoding $defs.%s: %v", name, err)
		}
		text, err := jsonschema.Marshal(object)
		if err != nil {
			t.Fatalf("re-serializing $defs.%s: %v", name, err)
		}
		out[name] = text
	}
	return out
}
