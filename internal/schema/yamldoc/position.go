package yamldoc

// Position is a 1-indexed line and column in a source document.
type Position struct {
	Line   int
	Column int
}

// Span is the region of a source document a node occupies.
type Span struct {
	Start Position
	End   Position
}
