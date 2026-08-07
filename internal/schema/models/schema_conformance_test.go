//go:build conformance

package models_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
)

const upstreamSchemaPath = "../../../third_party/rendercv/schema.json"

// The top-level object, everything except `$defs`.
//
// `$defs` is excluded because 209 of its 227 entries belong to models
// iterations 6 and 7 own (spec 005 §1); the per-entry differential in
// `jsonschema` covers the eighteen that exist. What is checked here is the
// envelope: the four added keys, the four blocks, and the key order that is
// deliberately not sorted.
func TestTopLevelSchemaMatchesUpstream(t *testing.T) {
	raw, err := os.ReadFile(upstreamSchemaPath)
	if err != nil {
		t.Fatalf("reading %s: %v — is the submodule initialized? `just setup`",
			upstreamSchemaPath, err)
	}

	var upstream map[string]json.RawMessage
	if err := json.Unmarshal(raw, &upstream); err != nil {
		t.Fatalf("parsing %s: %v", upstreamSchemaPath, err)
	}

	got := models.Schema()

	for _, key := range []string{"title", "description", "$id", "$schema", "type"} {
		t.Run(key, func(t *testing.T) {
			var want string
			if err := json.Unmarshal(upstream[key], &want); err != nil {
				t.Fatalf("upstream %s: %v", key, err)
			}
			value, ok := got.Get(key)
			if !ok {
				t.Fatalf("the port has no %s", key)
			}
			if value != want {
				t.Errorf("%s = %v, want %q", key, value, want)
			}
		})
	}

	t.Run("additionalProperties is false", func(t *testing.T) {
		if value, _ := got.Get("additionalProperties"); value != false {
			t.Errorf("additionalProperties = %v, want false", value)
		}
	})

	// Empty rather than absent: `Cv` omits the key and the top level does not,
	// so the two rules are different and both are checked.
	t.Run("required is an empty list", func(t *testing.T) {
		value, ok := got.Get("required")
		if !ok {
			t.Fatal("the port omits `required`; upstream emits []")
		}
		list, ok := value.([]any)
		if !ok || len(list) != 0 {
			t.Errorf("required = %v, want an empty list", value)
		}
	})
}

// Spec 005 §3.2 behavior 8, on the real object: the top-level key order is not
// sorted, and it is what upstream's file has.
func TestTopLevelKeyOrderMatchesUpstream(t *testing.T) {
	raw, err := os.ReadFile(upstreamSchemaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", upstreamSchemaPath, err)
	}

	want := topLevelKeyOrder(t, raw)
	got := strings.Join(models.Schema().Keys(), ",")
	if got != want {
		t.Errorf("key order =\n  %q\nwant\n  %q", got, want)
	}
}

// topLevelKeyOrder reads the file's own key order, which `encoding/json` into a
// map would discard — and the order is the thing being checked.
func topLevelKeyOrder(t *testing.T, raw []byte) string {
	t.Helper()

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if _, err := decoder.Token(); err != nil { // the opening brace
		t.Fatalf("reading the opening brace: %v", err)
	}

	var keys []string
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatalf("reading a key: %v", err)
		}
		key, ok := token.(string)
		if !ok {
			t.Fatalf("key token is %T", token)
		}
		keys = append(keys, key)

		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			t.Fatalf("skipping %s: %v", key, err)
		}
	}
	return strings.Join(keys, ",")
}

// The serialized form ends at the brace, as upstream's file does.
func TestSchemaJSONHasNoTrailingNewline(t *testing.T) {
	text, err := models.SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if !strings.HasSuffix(text, "\n}") {
		t.Errorf("the schema does not end at the closing brace")
	}
	if strings.HasSuffix(text, "\n") {
		t.Error("the schema ends with a newline; upstream's file does not")
	}
}
