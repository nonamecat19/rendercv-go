package yamldoc

import "strings"

// Kind is the shape of a document node: a scalar of a resolved type, a
// mapping, or a sequence.
type Kind uint8

// The node kinds, in resolution order (see yamlreader.ResolveScalar).
//
// KindTagged is appended last, and deliberately: the constants' values are
// pinned by a test, and inserting one in the middle would move the rest.
const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindMapping
	KindSequence

	// KindTagged is a scalar carrying an explicit tag that names no type the
	// loader constructs — `!!str`, `!unknown`, `!!merge` — which upstream's
	// round-trip loader turns into a `TaggedScalar`
	// (`ruamel/yaml/constructor.py:1598-1621`, reached from
	// `construct_yaml_str` at `:1181-1184` whenever the node carries a tag
	// handle).
	//
	// A `TaggedScalar` is an ordinary object with no relationship to `str`,
	// `int` or `bool`, so **every** typed field rejects it while its `str()`
	// — its text, kept in Raw — still reaches the Input Value column and any
	// message that interpolates the value. That is why this is a kind rather
	// than a flag on an otherwise-resolved node: the consumers that must
	// reject it do so by not naming it, instead of by remembering to check
	// (spec 015 plan §1).
	KindTagged
)

// BoolIsTrue is the truth of a KindBool node, from its text.
//
// The set is wider than the one a *plain* scalar resolves with, because an
// explicit `!!bool` tag accepts YAML 1.1's spellings — `yes`, `no`, `y`, `n`,
// `on`, `off` — matched on `value.lower()`
// (`ruamel/yaml/constructor.py:432-445`). A plain `yes` is a string and never
// reaches here; a tagged one is a `bool` and does, so a caller testing only
// for `"true"` read `!!bool yes` as false.
//
// Callers must not spell this test themselves: the answer decides what a
// document renders, what the Input Value column shows, and what a settings
// flag does, and those three drifting apart is the failure this replaces.
func BoolIsTrue(raw string) bool {
	switch strings.ToLower(raw) {
	case "true", "yes", "y", "on":
		return true
	}
	return false
}

// ScalarStyle is how a scalar was written, which decides whether its text is
// resolved to a typed value or kept as a string.
type ScalarStyle uint8

// The scalar styles YAML distinguishes.
const (
	StylePlain ScalarStyle = iota
	StyleSingleQuoted
	StyleDoubleQuoted
	StyleLiteral
	StyleFolded
)

// Node is one value in the parsed document, carrying its source span so an
// error can be reported where the user wrote it.
type Node struct {
	Kind  Kind
	Span  Span
	Raw   string
	Style ScalarStyle
	Items []Item
	Elems []*Node
}

// Item is one key/value pair of a mapping, in input order.
type Item struct {
	Key     string
	KeySpan Span
	Value   *Node

	// KeyTagged marks a key written with an explicit tag. Upstream resolves it
	// to a `TaggedScalar` rather than a `str`, so it names no field at all and
	// pydantic reports `Keys should be strings.` against the enclosing mapping
	// (measured on `cv: {!!str name: John Doe}`). Key keeps the text, which is
	// what that record's Input Value column carries.
	KeyTagged bool
}
