package jsonschema

import (
	"strings"
	"testing"
)

// A replacement keeps the key where it is. Everything about the top-level key
// order depends on it.
func TestSetOverwritesInPlace(t *testing.T) {
	object := NewObject().Set("a", 1).Set("b", 2).Set("c", 3)
	object.Set("a", 99)

	if got := strings.Join(object.Keys(), ","); got != "a,b,c" {
		t.Errorf("keys = %q, want a,b,c — a replacement must not move the key", got)
	}
	if value, _ := object.Get("a"); value != 99 {
		t.Errorf("a = %v, want the replacement", value)
	}
}

// Spec 005 §3.2 behavior 8, reproduced by the two operations that produce it:
// pydantic sorts, then the generator overwrites `title` and inserts three keys.
//
// This is the case that distinguishes in-place replacement from
// append-on-overwrite. Under the wrong semantics `title` moves to the end and
// the sequence becomes `$defs, additionalProperties, properties, required,
// type, title, description, $id, $schema` — every key upstream has, in the
// wrong order.
func TestTopLevelKeyOrderIsReproducible(t *testing.T) {
	// What pydantic emits, sorted.
	object := NewObject().
		Set("type", "object").
		Set("properties", nil).
		Set("$defs", nil).
		Set("required", nil).
		Set("title", "RenderCVModel").
		Set("additionalProperties", false).
		Sort()

	// What the generator then does.
	object.Set("title", "RenderCV")
	object.Set("description", "Resume builder for academics and engineers")
	object.Set("$id", "https://raw.githubusercontent.com/rendercv/rendercv/main/schema.json")
	object.Set("$schema", "http://json-schema.org/draft-07/schema#")

	const want = "$defs,additionalProperties,properties,required,title,type," +
		"description,$id,$schema"
	if got := strings.Join(object.Keys(), ","); got != want {
		t.Errorf("keys =\n  %q\nwant\n  %q", got, want)
	}
}

// Sort is ASCII, so `$` sorts before every letter — which is why `$defs` leads
// the top-level object.
func TestSortIsASCII(t *testing.T) {
	object := NewObject().
		Set("type", nil).Set("$schema", nil).Set("Title", nil).Set("anyOf", nil).Sort()

	if got := strings.Join(object.Keys(), ","); got != "$schema,Title,anyOf,type" {
		t.Errorf("keys = %q", got)
	}
}

// A present key with a nil value is not an absent key. One is
// `"description": null` and the other is nothing at all, and both are reachable.
func TestNilValueIsPresent(t *testing.T) {
	object := NewObject().Set("description", nil)

	if value, ok := object.Get("description"); !ok || value != nil {
		t.Errorf("description = %v (present %v), want a present nil", value, ok)
	}
	if _, ok := object.Get("absent"); ok {
		t.Error("an unset key reported present")
	}
}
