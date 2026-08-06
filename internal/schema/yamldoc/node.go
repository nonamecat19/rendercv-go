package yamldoc

type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindMapping
	KindSequence
)

type ScalarStyle uint8

const (
	StylePlain ScalarStyle = iota
	StyleSingleQuoted
	StyleDoubleQuoted
	StyleLiteral
	StyleFolded
)

type Node struct {
	Kind  Kind
	Span  Span
	Raw   string
	Style ScalarStyle
	Items []Item
	Elems []*Node
}

type Item struct {
	Key     string
	KeySpan Span
	Value   *Node
}
