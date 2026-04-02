package modelbuilder

import (
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// blockFailure is what ruamel reports for a badly indented line in block
// context: the phrasing it opens with, its context mark when it has one, and
// its problem mark.
type blockFailure struct {
	phrasing string
	context  yamldoc.Position // the zero value when ruamel reports no context
	problem  yamldoc.Position
}

// blockKind distinguishes the two block collections, because ruamel names them
// differently: a mapping is `while parsing a block mapping` and a sequence is
// `while parsing a block collection`.
type blockKind int

const (
	blockMapping blockKind = iota
	blockSequence
)

// openBlock is one block collection the scan found still open: what kind it is,
// the 0-based column its entries start at, and where it began.
type openBlock struct {
	kind  blockKind
	col   int
	start yamldoc.Position
}

// blockScan reconstructs ruamel's verdict for the failure goccy spells
// `value is not allowed in this context`, from the source and the token goccy
// blamed. ok is false when the scan cannot name a block construct, which is
// where ruamel stops talking about block constructs too.
//
// **One goccy failure, four ruamel ones.** goccy reports a single "this token
// cannot go here"; ruamel splits the same inputs between its parser and its
// scanner, and the split is not visible in goccy's text at all:
//
//   - its *parser* raises `while parsing a block mapping` or `while parsing a
//     block collection` when a token cannot follow the construct it is inside.
//     Which construct that is, is the innermost block level still open at the
//     offending line's own indentation — ruamel's scanner unwinds its indent
//     stack to that column first, so a key one step left of a sequence item's
//     keys lands on the *sequence* and two steps left lands on the mapping
//     above it. Measured on both, and on three levels of nesting.
//   - its *scanner* raises `mapping values are not allowed here` when a plain
//     scalar folds across lines and the line it folds into carries a colon,
//     and `while scanning a simple key` when a key that has to fit on one line
//     does not. A key is required — and so worth an error rather than a silent
//     re-read — exactly when it sits at the open construct's own column.
//
// The scanner runs ahead of the parser, so a scanner failure at or before
// goccy's token wins; past it, the parser's token is the answer.
//
// Measured over 101,535 shapes across four sweeps: three structured (indent
// width, key length, nesting depth, mapping versus sequence enclosure, position
// of the successor, quoted versus plain keys, comments indented and trailing,
// blank lines, document markers) and one pseudo-random. All 74,985 structured
// shapes agree with ruamel on the phrasing and both marks. Of the 7,433
// random-junk shapes this answers, 19 disagree — documents with several
// unrelated mistakes at once, where ruamel's own scanner and parser race
// differently. 19,117 more are declined; see the ok result.
func blockScan(content string, tok yamldoc.Position) (blockFailure, bool) {
	lines := strings.Split(content, "\n")

	var stack []openBlock
	pending := -1  // column of a key still waiting for its value, or -1
	plainAt := 0   // line an open plain scalar started on, or 0
	strayLine := 0 // a scalar the parser will reject, unless the scanner beats it
	strayCol := 0
	blockAt := -1     // column that bounds an open block scalar's content, or -1
	blockOpenAt := -1 // column of a key whose block scalar has no content yet

	// parserVerdict is ruamel's parser failing at (line, column): it names the
	// innermost level still open there and marks where that level began.
	parserVerdict := func(line, column int) (blockFailure, bool) {
		open := stack
		for len(open) > 0 && open[len(open)-1].col > tok.Column-1 {
			open = open[:len(open)-1]
		}
		if len(open) == 0 {
			return blockFailure{}, false
		}
		inner := open[len(open)-1]
		phrasing := "while parsing a block mapping"
		if inner.kind == blockSequence {
			phrasing = "while parsing a block collection"
		}
		return blockFailure{
			phrasing: phrasing,
			context:  inner.start,
			problem:  yamldoc.Position{Line: line, Column: column},
		}, true
	}

	for n := 1; n <= len(lines); n++ {
		text := lines[n-1]
		if strings.TrimSpace(text) == "" {
			continue // a blank line is a line break inside a scalar, nothing more
		}
		pos := lineIndent(text)

		if blockAt >= 0 {
			if pos >= blockAt {
				continue // a block scalar's own content
			}
			blockAt = -1
		}

		if strings.HasPrefix(strings.TrimLeft(text, " "), "#") {
			// A comment ends an open plain scalar, and ends a block scalar that
			// has not established its indentation yet.
			if blockOpenAt >= 0 && pos <= blockOpenAt {
				blockOpenAt = -1
			}
			if strayLine > 0 {
				return parserVerdict(strayLine, strayCol)
			}
			plainAt = 0
			continue
		}

		if isDocumentMarker(text) {
			stack, pending, plainAt = nil, -1, 0
			strayLine, strayCol = 0, 0
			blockAt, blockOpenAt = -1, -1
			continue
		}

		// A line deeper than a construct that already has its value is not a
		// new entry: it is scalar content, or the token the parser rejects.
		if len(stack) > 0 && pending < 0 && pos > stack[len(stack)-1].col {
			body := strings.TrimSpace(text)
			switch {
			case blockOpenAt >= 0:
				blockAt, blockOpenAt = pos, -1
			case plainAt > 0:
				if plainAt <= tok.Line && hasKeyIndicator(text) {
					return blockFailure{
						phrasing: "mapping values are not allowed here",
						problem: yamldoc.Position{
							Line: n, Column: strings.IndexByte(text, ':') + 1,
						},
					}, true
				}
			case strings.ContainsRune("\"'[{", rune(body[0])),
				hasKeyIndicator(text), isBlockEntry(body):
				return parserVerdict(n, pos+1)
			case body[0] == '|' || body[0] == '>':
			default:
				plainAt, strayLine, strayCol = n, n, pos+1
			}
			continue
		}

		// Anything at or left of the open level ends a stray scalar, and the
		// parser reports the scalar rather than this line.
		if strayLine > 0 {
			return parserVerdict(strayLine, strayCol)
		}

		before := stack
		for len(stack) > 0 && stack[len(stack)-1].col > pos {
			stack = stack[:len(stack)-1]
		}

		rest := text[pos:]
		entry := isBlockEntry(rest)
		if entry {
			if n >= tok.Line {
				stack = before
				return parserVerdict(tok.Line, tok.Column)
			}
			for isBlockEntry(rest) {
				// A sequence whose dash sits at its parent's own column adds no
				// indentation level, so ruamel keeps naming the parent.
				if len(stack) == 0 || stack[len(stack)-1].col != pos {
					stack = append(stack, openBlock{
						kind:  blockSequence,
						col:   pos,
						start: yamldoc.Position{Line: n, Column: pos + 1},
					})
				}
				pos++
				for pos < len(text) && text[pos] == ' ' {
					pos++
				}
				rest = text[pos:]
				if rest == "" {
					break
				}
			}
		}

		plainAt, blockOpenAt = 0, -1
		if hasKeyIndicator(rest) {
			if n >= tok.Line {
				stack = before
				return parserVerdict(tok.Line, tok.Column)
			}
			if len(stack) == 0 || stack[len(stack)-1].kind != blockMapping ||
				stack[len(stack)-1].col != pos {
				stack = append(stack, openBlock{
					kind:  blockMapping,
					col:   pos,
					start: yamldoc.Position{Line: n, Column: pos + 1},
				})
			}
			raw := rest[strings.IndexByte(rest, ':')+1:]
			value := strings.TrimSpace(stripComment(raw))
			if value == "" {
				pending = pos
				continue
			}
			pending = -1
			// A trailing comment, a quoted scalar and a flow collection all end
			// where they end; only a bare plain scalar can reach the next line.
			if value == strings.TrimSpace(raw) && !strings.ContainsRune("\"'[{", rune(value[0])) {
				if value[0] == '|' || value[0] == '>' {
					blockOpenAt = pos
				} else {
					plainAt = n
				}
			}
			continue
		}

		if rest == "" {
			pending = -1
			continue
		}

		col := -1
		if len(stack) > 0 {
			col = stack[len(stack)-1].col
		}
		// A scalar is a legal value when a key or a dash above it asked for one.
		legal := entry || (pending >= 0 && pos > pending)
		pending = -1

		if pos == col {
			if n > tok.Line {
				return parserVerdict(tok.Line, tok.Column)
			}
			quoted := rest[0] == '"' || rest[0] == '\''
			return blockFailure{
				phrasing: "while scanning a simple key",
				context:  yamldoc.Position{Line: n, Column: pos + 1},
				problem:  simpleKeyProblem(lines, n, col, quoted),
			}, true
		}
		if strings.ContainsRune("\"'[{", rune(rest[0])) ||
			strings.TrimSpace(stripComment(rest)) != strings.TrimSpace(rest) {
			if !legal {
				return parserVerdict(n, pos+1)
			}
			continue
		}
		plainAt = n
		if !legal {
			strayLine, strayCol = n, pos+1
		}
	}

	if strayLine > 0 {
		return parserVerdict(strayLine, strayCol)
	}
	return parserVerdict(tok.Line, tok.Column)
}

// simpleKeyProblem is where ruamel's scanner stands when it gives up on a
// required key: the next token it reached, which is not the same place for the
// two kinds of scalar.
//
// A plain scalar is still being scanned, so the scan stops on whatever ends it
// — the colon of the line it folded into when the successor is deeper, and
// otherwise that successor's first character. A quoted scalar is already a
// finished token, so the scanner has moved on, and a run of comments directly
// behind it is consumed along with it; a comment separated by a blank line is
// not. Measured on 1,486 shapes.
func simpleKeyProblem(lines []string, n, col int, quoted bool) yamldoc.Position {
	i := n + 1
	blank := false
	for i <= len(lines) && strings.TrimSpace(lines[i-1]) == "" {
		i++
		blank = true
	}
	if i > len(lines) {
		return yamldoc.Position{Line: len(lines), Column: 1}
	}

	text := lines[i-1]
	if strings.HasPrefix(strings.TrimLeft(text, " "), "#") {
		if quoted && !blank {
			j := i + 1
			for j <= len(lines) && (strings.TrimSpace(lines[j-1]) == "" ||
				strings.HasPrefix(strings.TrimLeft(lines[j-1], " "), "#")) {
				j++
			}
			return yamldoc.Position{Line: min(j, len(lines)), Column: 1}
		}
		return yamldoc.Position{Line: i, Column: lineIndent(text) + 1}
	}

	if !quoted && lineIndent(text) > col && strings.ContainsRune(text, ':') {
		return yamldoc.Position{Line: i, Column: strings.IndexByte(text, ':') + 1}
	}
	return yamldoc.Position{Line: i, Column: lineIndent(text) + 1}
}

// lineIndent is the 0-based column a line's first non-space character sits at.
func lineIndent(text string) int {
	return len(text) - len(strings.TrimLeft(text, " "))
}

// isBlockEntry reports whether a line's content starts with a sequence
// indicator.
func isBlockEntry(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// isDocumentMarker reports whether a line is a `---` or `...` at column 1,
// which closes every block level the document had opened.
func isDocumentMarker(text string) bool {
	return text == "---" || text == "..." ||
		strings.HasPrefix(text, "--- ") || strings.HasPrefix(text, "... ")
}

// stripComment drops a trailing `#` comment, which ends any plain scalar before
// it. Quotes are honoured so a `#` inside a scalar does not open one.
func stripComment(text string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '#' && (i == 0 || text[i-1] == ' ' || text[i-1] == '\t'):
			return text[:i]
		}
	}
	return text
}
