package yamldoc

type Position struct {
	Line   int
	Column int
}

type Span struct {
	Start Position
	End   Position
}
