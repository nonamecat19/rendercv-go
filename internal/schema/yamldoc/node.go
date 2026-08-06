package yamldoc

// Kind is the shape of a document node: a scalar of a resolved type, a
// mapping, or a sequence.
type Kind uint8

// The node kinds, in resolution order (see yamlreader.ResolveScalar).
const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindMapping
	KindSequence
)

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
}
