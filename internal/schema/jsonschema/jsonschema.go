// Package jsonschema builds the JSON Schema of `rendercv-go schema`, mirroring
// schema/json_schema_generator.py.
//
// Upstream's generator is 45 lines because pydantic reflects the schema out of
// the model classes. The Go port has no such reflection — its validators are
// functions over document nodes — so the schema is hand-built data, diffed
// against upstream per `$defs`. This package is the machinery; the data lives
// beside each model.
package jsonschema

import "sort"

// Object is a JSON object that remembers its key order.
//
// Every schema node is one, because spec 005 §6 makes order contractual in
// three different ways at once: sorted almost everywhere, field-declaration
// order inside `properties`, and one fixed non-sorted sequence at the top level.
type Object struct {
	keys   []string
	values map[string]any
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{values: map[string]any{}}
}

// Set adds a key or replaces an existing one's value.
//
// **A replacement keeps the key where it is.** That is not a convenience: it is
// how the top-level `title` stays inside the sorted run while `description`,
// `$id` and `$schema` land after it (spec 005 §3.1 behavior 6). An
// implementation that moved a replaced key to the end would produce every key
// upstream has, in the wrong order, and the difference is three keys at the end
// of a 405 KB file.
func (o *Object) Set(key string, value any) *Object {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
	return o
}

// Sort orders the keys by ASCII, which is what pydantic does for every object
// except `properties` (spec 005 §6 rules 2 and 3).
func (o *Object) Sort() *Object {
	sort.Strings(o.keys)
	return o
}

// Keys returns the key order.
func (o *Object) Keys() []string {
	return o.keys
}

// Get reports a key's value and whether it is present. A present key whose value
// is nil is the `"description": null` of spec 005 §5 behavior 18, and is
// distinguishable from an absent one.
func (o *Object) Get(key string) (any, bool) {
	value, ok := o.values[key]
	return value, ok
}

// Len returns the number of keys.
func (o *Object) Len() int {
	return len(o.keys)
}
