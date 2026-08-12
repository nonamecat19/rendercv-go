// Package modelbuilder mirrors src/rendercv/schema/rendercv_model_builder.py.
package modelbuilder

import (
	"errors"
	"fmt"
	"strings"

	goyaml "github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// ReadYamlWithValidationErrors mirrors read_yaml_with_validation_errors
// (rendercv_model_builder.py:64-101): it parses YAML content and converts a
// parser failure into a single user validation error so syntax errors travel
// the same pipeline as schema validation errors.
//
// Non-parser failures from the reader (empty input, string root) pass through
// unchanged, as they do upstream.
func ReadYamlWithValidationErrors(
	content string,
	source schemaerr.YamlSource,
) (*yamldoc.Node, error) {
	node, err := yamlreader.ReadString(content)
	if err == nil {
		return node, nil
	}

	// A multi-document stream is ruamel's *composer* failure rather than its
	// parser's, so it carries marks of its own instead of a failing token —
	// see yamlreader.MultiDocumentError.
	var multiDoc *yamlreader.MultiDocumentError
	if errors.As(err, &multiDoc) {
		return nil, &schemaerr.UserValidationError{
			Errors: []schemaerr.ValidationError{{
				SchemaLocation: nil,
				YamlLocation:   &yamldoc.Span{Start: multiDoc.Start, End: multiDoc.End},
				YamlSource:     source,
				Message:        fmt.Sprintf("This is not a valid YAML file. %s.", multiDoc.Error()),
				Input:          schemaerr.InputEllipsis,
			}},
		}
	}

	// A tab used as separation whitespace is ruamel's scanner failure, which
	// goccy does not raise at all in most positions — see yamlreader.TabError.
	// ruamel reports no context mark, so the location is the one line.
	var tabErr *yamlreader.TabError
	if errors.As(err, &tabErr) {
		at := yamldoc.Position{Line: tabErr.Line, Column: 1}
		return nil, &schemaerr.UserValidationError{
			Errors: []schemaerr.ValidationError{{
				SchemaLocation: nil,
				YamlLocation:   &yamldoc.Span{Start: at, End: at},
				YamlSource:     source,
				Message:        fmt.Sprintf("This is not a valid YAML file. %s.", tabErr.Error()),
				Input:          schemaerr.InputEllipsis,
			}},
		}
	}

	var parserErr goyaml.Error
	if !errors.As(err, &parserErr) {
		return nil, err
	}

	return nil, &schemaerr.UserValidationError{
		Errors: []schemaerr.ValidationError{yamlSyntaxValidationError(parserErr, content, source)},
	}
}

// yamlSyntaxValidationError builds the single record described by spec §3.83:
// no schema location, a location derived from the parser's marks, the source of
// the document being parsed, the message of §4.17, and the literal input echo.
func yamlSyntaxValidationError(
	parserErr goyaml.Error,
	content string,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	return schemaerr.ValidationError{
		SchemaLocation: nil,
		YamlLocation:   yamlErrorLocation(parserErr, content),
		YamlSource:     source,
		Message:        fmt.Sprintf("This is not a valid YAML file. %s", parserMessage(parserErr.Error(), content)),
		Input:          schemaerr.InputEllipsis,
	}
}

// ruamelPhrasing maps goccy's error taxonomy onto ruamel's, for the syntax
// failures the corpus contains (spec 004 §7.5, plan §6 option B).
//
// The two libraries describe the same failures differently: goccy names the
// token it wanted, ruamel names the construct it was in. Upstream interpolates
// ruamel's phrasing into a user-visible message, so parity needs the mapping.
//
// **The set is deliberately small and grows only when a corpus case is added.**
// The corpus has exactly one syntax case today — `this: [is, not, a, cv`, whose
// golden reads `while parsing a flow sequence` — and the other four rows were
// measured alongside it because they are the shapes a user is next most likely
// to write. An unmapped failure falls through to goccy's own first line, which
// is option A for the remainder: wrong, but visibly wrong rather than silently
// misattributed.
//
// Each key is a substring of goccy's message; each value is ruamel's verbatim
// first line, measured against the vendored Python.
var ruamelPhrasing = []struct{ goccy, ruamel string }{
	{"sequence end token", "while parsing a flow sequence"},
	// **A block line that breaks an open flow collection is a third shape.**
	// goccy phrases it by the closer it wanted, and it is not the
	// unterminated-to-EOF case: `cv: [a` followed by `b: c` stops *at* the
	// block line rather than running to the end of the stream, so it needs its
	// own location rule too (see flowInterruptedLocation).
	{"']' must be specified", "while parsing a flow sequence"},
	{"'}' must be specified", "while parsing a flow mapping"},
	// **The same break phrased a fourth way.** When the breaking line's value
	// opens a flow mapping — `cv: [a\n  b: {c,` — goccy names neither closer
	// and says `unexpected map key` instead. Enumerating the shape space (every
	// opening delimiter, first element, indent 0-6 and nine inner values) found
	// 35 inputs it answers this way and all 35 have an open flow *sequence*, so
	// this row needs no condition. Measured on all of them: ruamel says
	// `while parsing a flow sequence` and locates it like the rows above.
	{"unexpected map key", "while parsing a flow sequence"},
	{"flow map", "while parsing a flow mapping"},
	{"quoted text", "while scanning a quoted scalar"},
	// goccy has two spellings for a tab and only one says "tab character".
	// **Neither is reached by a tab in an ordinary position** — `\ta: 1` and
	// `cv:\n\tname: a` are both caught by yamlreader.TabError before the parser
	// runs, and that branch phrases them itself. What is left for these two rows
	// is the tabs TabError deliberately allows: one indenting a block scalar's
	// content (`a: |\n\tx` — "found a tab character where an indentation space
	// is expected") and one at the start of a line inside what TabError read as
	// an open flow collection but goccy read as a plain scalar (`a: b[\n\tc` —
	// "found character '\t' that cannot start any token"). ruamel says
	// `while scanning for the next token` for all four, measured.
	{"tab character", "while scanning for the next token"},
	{"cannot start any token", "while scanning for the next token"},
	{"already defined", "while constructing a mapping"},
}

// parserMessage mirrors rendercv_model_builder.py:87-89: the first line of the
// parser's own error text, stripped, with a period appended when absent.
//
// The first line is mapped onto ruamel's phrasing first, so what upstream
// interpolates and what the port interpolates agree for the mapped set.
func parserMessage(text, content string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)

	// **An unterminated flow collection has two ruamel phrasings, not one**, and
	// goccy's message cannot tell them apart: `cv: [` and `cv: [a` produce the
	// identical `sequence end token ']' not found`. ruamel distinguishes them by
	// what its parser was doing when the stream ended — see
	// flowNodeExpectedAtEOF.
	if flowNodeExpectedAtEOF(text, content) {
		return "while parsing a flow node."
	}

	for _, row := range ruamelPhrasing {
		if strings.Contains(text, row.goccy) {
			text = row.ruamel
			break
		}
	}

	if text == "" {
		return "."
	}
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text
}

// unterminatedConstructMessages are the goccy failure substrings whose token
// is measurably ruamel's *context* mark (the opening delimiter) rather than
// its *problem* mark — an unclosed flow sequence or flow mapping, where the
// scanner reads all the way to EOF hunting for the missing `]`/`}`. Measured
// against the vendored CLI on both shapes, nested and top-level.
//
// A bad-indentation failure is a different shape — the token *is* the problem
// and ruamel reports no context mark at all, so it keeps a single-line span
// and is not in this set. **A duplicate key is a third shape**, handled
// separately by mappingStartLine: ruamel reports a context mark there too, but
// it is the enclosing mapping's first key rather than anything at EOF. This
// comment used to claim a duplicate key was single-line like bad indentation,
// which was measurably false.
var unterminatedConstructMessages = []string{
	"sequence end token", // `this: [is, not, a, cv` — the corpus case
	"flow map content",   // `name: {John` — measured separately, same shape
	"flow mapping end",   // `name: {` — the empty shape, see flowNodeExpectedAtEOF
}

// spanToEOFMessages are every failure whose span runs from the token to the
// end of the stream — the unterminated flow collections above, plus an
// unterminated quoted scalar, whose scanner likewise reads to EOF hunting for
// the closing quote. Measured: `cv: "a` reports `line 1 to line 2` and
// `cv: "a\nb\nc` reports `line 1 to line 4`.
//
// **The quoted case is deliberately not in unterminatedConstructMessages**,
// because that list also gates flowNodeExpectedAtEOF and a quoted scalar can
// legitimately end in a comma: `cv: ["a,` is ruamel's *quoted scalar* failure,
// not its flow-node one, so routing it through the node check would report the
// wrong construct at the wrong line.
var spanToEOFMessages = append(
	append([]string{}, unterminatedConstructMessages...),
	"quoted text",
)

// flowNodeExpectedAtEOF reports whether ruamel would have been *waiting for a
// node* when the stream ended, rather than waiting for a separator or a closing
// delimiter.
//
// **This is the difference between two ruamel phrasings that goccy reports
// identically.** `cv: [a` and `cv: [` both come out of goccy as `sequence end
// token ']' not found`, but ruamel says `while parsing a flow sequence` for the
// first and `while parsing a flow node` for the second — and it reports them at
// different places: the sequence form carries a context mark at the opening
// `[` and a problem mark at EOF, so it spans lines, while the node form puts
// both marks at EOF and so reports a single line.
//
// The discriminator, measured against ruamel on eleven shapes: the parser wants
// a node exactly when the last significant character of the document is an
// opening flow delimiter or a comma. Comments and blank lines are not
// significant; a trailing comma counts, so `cv: [a,` is the node form even
// though it has content, and `cv: {a: 1,` likewise.
//
// **The question is only meaningful if the scan reached the end at all.** When
// a block line broke the flow first, ruamel stopped there and never saw the
// rest of the file, so the last character of the *file* answers a question it
// was not asked: `cv: [a\n  b: c,` ends in a comma and is nonetheless
// `while parsing a flow sequence`, line 1 to line 2, because the scan stopped
// on line 2. Reading the comma made the port answer `while parsing a flow node`
// at line 3 — the wrong construct at a line the user cannot see. `cv: [a,\nb:
// c,` is the counterpart that really is the node form: after the first comma
// the flow wanted an element, `b: c` is a legal one, and only the *trailing*
// comma ends the stream.
func flowNodeExpectedAtEOF(message, content string) bool {
	unterminated := false
	for _, candidate := range unterminatedConstructMessages {
		if strings.Contains(message, candidate) {
			unterminated = true
			break
		}
	}
	if !unterminated {
		return false
	}

	if open := openFlowLine(content); open > 0 && breakingLine(content, open) > 0 {
		return false
	}

	switch lastSignificantByte(content) {
	case '[', '{', ',':
		return true
	default:
		return false
	}
}

// openFlowLine is the line of the outermost flow delimiter still open at the
// end of `content`, or 0 when every one of them was closed.
func openFlowLine(content string) int {
	return outermostFlowOpenLine(content, strings.Count(content, "\n")+2)
}

// flowInterruptedByBlockLine reports whether a goccy message describes an open
// flow collection that a block-mapping line disturbed, rather than one the
// scanner carried to the end of the stream.
//
// goccy spells the same break by whichever token it wanted next: the closer
// (`',' or ']' must be specified`) when the breaking line's value is a plain
// scalar or a flow sequence, and `unexpected map key` when that value opens a
// flow mapping. ruamel makes no such distinction — both are its context mark at
// the opening delimiter and its problem mark at the breaking line — so both
// take the same location rule.
func flowInterruptedByBlockLine(message string) bool {
	return strings.Contains(message, "must be specified") ||
		strings.Contains(message, "unexpected map key")
}

// isUnterminatedFlow reports whether a goccy message names a flow collection
// the scanner never found the end of, as opposed to the quoted scalar that
// shares its span shape but not its marks.
func isUnterminatedFlow(message string) bool {
	for _, candidate := range unterminatedConstructMessages {
		if strings.Contains(message, candidate) {
			return true
		}
	}
	return false
}

// breakingLine is the first line after `open` that carries a YAML key
// indicator — a colon followed by whitespace or the end of the line, outside
// quotes — or 0 when no line does.
//
// That indicator is what makes a line a block-mapping entry, which an open
// flow collection cannot contain, so it is where ruamel's scan stops. `b` and
// `b:c` carry none and are swallowed by the flow instead. Measured on all
// three, indented and not.
// **A key indicator only breaks the flow when the flow is not expecting an
// element.** After a comma or the opening delimiter, `c: d` is a legal
// single-pair flow mapping and the collection swallows it — `cv: [a,\n b,\nc: d`
// runs to EOF — whereas after a complete element the parser wants `,` or the
// closer, so `cv: [a\n  b: c` stops at the second line.
func breakingLine(content string, open int) int {
	lines := strings.Split(content, "\n")
	// The scan cannot run past the end of the stream, so a block line in a
	// *following* document is not the one that broke this flow.
	limit := min(streamEndLine(content, open), len(lines))

	for i := open + 1; i < limit; i++ {
		if !hasKeyIndicator(lines[i-1]) {
			continue
		}
		switch lastSignificantByte(strings.Join(lines[:i-1], "\n")) {
		case ',', '[', '{':
			continue // an element is expected here, so this line is one
		}
		return i
	}
	return 0
}

// hasKeyIndicator reports whether a line carries an unquoted `: ` (or a
// trailing `:`).
func hasKeyIndicator(text string) bool {
	inSingle, inDouble := false, false
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '\\' && i+1 < len(text) {
				i++
			} else if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == ':':
			if i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\t' {
				return true
			}
		}
	}
	return false
}

// streamEndLine is where ruamel's scanner stops when it runs off the end of a
// construct: the first document marker after `after`, or the physical end of
// the file when there is none.
//
// **A `---` or `...` ends the scan before the file does.** `cv: [a\n...` is
// `line 1 to line 2`, not to line 3, because the marker closes the stream the
// scanner was reading. A marker *before* the construct began does not count —
// a leading `---` leaves an unterminated flow spanning to the true EOF — and a
// marker must be unquoted at column 1, so a `"..."` scalar is not one.
func streamEndLine(content string, after int) int {
	lines := strings.Split(content, "\n")
	for i := after + 1; i <= len(lines); i++ {
		text := lines[i-1]
		if text == "---" || text == "..." ||
			strings.HasPrefix(text, "--- ") || strings.HasPrefix(text, "... ") {
			return i
		}
	}
	return strings.Count(content, "\n") + 1
}

// outermostFlowOpenLine returns the line of the outermost flow delimiter that
// is still open when line `before` is reached — ruamel's context mark for a
// flow collection a block line interrupted.
//
// It is the *outermost* one, not the nearest: `cv: [[a` broken on the next
// line reports line 1 for the outer `[`, and both open on the same line
// anyway. Quotes are honoured so a bracket inside a scalar does not count.
func outermostFlowOpenLine(content string, before int) int {
	depth, openedAt := 0, 0
	line := 1
	inSingle, inDouble, inComment := false, false, false
	var prev byte

	for i := 0; i < len(content) && line < before; i++ {
		c := content[i]
		if c == '\n' {
			line++
			inComment = false
			prev = c
			continue
		}

		switch {
		case inComment:
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '\\' && i+1 < len(content) {
				i++
				break
			}
			if c == '"' {
				inDouble = false
			}
		case c == '#' && (prev == 0 || prev == ' ' || prev == '\t' || prev == '\n'):
			inComment = true
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '[' || c == '{':
			if depth == 0 {
				openedAt = line
			}
			depth++
		case c == ']' || c == '}':
			if depth > 0 {
				depth--
			}
			if depth == 0 {
				openedAt = 0
			}
		}
		prev = c
	}

	if depth == 0 {
		return 0
	}
	return openedAt
}

// mappingStartLine finds the line where the block mapping containing a key at
// (line, column) began — ruamel's context mark for a duplicate-key failure.
//
// It walks upward from the key, skipping blank and comment lines, and keeps
// the highest line whose own key sits at the same column. A line indented
// further belongs to some nested value and is stepped over; a line indented
// less closes the mapping, as does a document marker. A `- ` sequence
// indicator counts as indentation, so the `a` of `  - a: 1` is at column 5 and
// matches the `a` of `    a: 2`.
//
// Measured against ruamel on eleven shapes: adjacent and distant duplicates, a
// duplicate that is not the mapping's first key (the case where goccy's own
// "already defined at" line is the wrong answer) both at the top level and
// nested, nesting one and two levels deep, a value block between the two keys,
// blank lines, comments, a sequence of mappings, and a document marker. They
// are `TestDuplicateKeySpansItsMapping`'s rows, one for one; the count and the
// list are part of what that test pins, so a new row belongs in both.
func mappingStartLine(content string, line, column int) int {
	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		return line
	}

	start := line
	for i := line - 1; i >= 1; i-- {
		text := lines[i-1]
		trimmed := strings.TrimLeft(text, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue // not part of any mapping
		}
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "...") {
			break // a document boundary closes everything
		}

		switch indent := keyColumn(text); {
		case indent == column:
			start = i
		case indent < column:
			return start // the mapping is closed by something less indented
		}
	}
	return start
}

// keyColumn is the 1-based column a line's own key starts at, counting any
// `- ` sequence indicators as part of the indentation.
func keyColumn(text string) int {
	column := 1
	for {
		rest := text[min(column-1, len(text)):]
		trimmed := strings.TrimLeft(rest, " \t")
		column += len(rest) - len(trimmed)
		if !strings.HasPrefix(trimmed, "- ") && trimmed != "-" {
			return column
		}
		column += 2 // step over the indicator and its space
	}
}

// lastSignificantByte returns the final byte of the document that YAML would
// treat as content, skipping trailing whitespace and comments. It is
// quote-aware, because a `#` inside a quoted scalar does not open a comment and
// a `[` inside one is not a delimiter — `cv: ["a#b"` and `cv: ["a["` are both
// the sequence form, measured.
func lastSignificantByte(content string) byte {
	// **A document marker ends the stream the scanner was reading.** Without
	// this, `cv: [\n...\n` ends in the `.` of the marker rather than the `[`,
	// so the flow-node form is missed and the location comes out as a span.
	if end := streamEndLine(content, 0); end <= strings.Count(content, "\n") {
		lines := strings.Split(content, "\n")
		content = strings.Join(lines[:end-1], "\n")
	}

	var last byte
	inSingle, inDouble, inComment := false, false, false
	var prev byte

	for i := 0; i < len(content); i++ {
		c := content[i]

		switch {
		case inComment:
			if c == '\n' {
				inComment = false
			}
			prev = c
			continue
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
			last, prev = c, c
			continue
		case inDouble:
			// A backslash escapes the next byte, so an escaped quote does not
			// close the scalar.
			if c == '\\' && i+1 < len(content) {
				i++
				last, prev = content[i], content[i]
				continue
			}
			if c == '"' {
				inDouble = false
			}
			last, prev = c, c
			continue
		}

		// A `#` opens a comment only at the start of a line or after
		// whitespace; `a#b` is a single plain scalar.
		if c == '#' && (prev == 0 || prev == ' ' || prev == '\t' || prev == '\n') {
			inComment = true
			prev = c
			continue
		}

		switch c {
		case '\'':
			inSingle = true
			last = c
		case '"':
			inDouble = true
			last = c
		case ' ', '\t', '\n', '\r':
			// not significant
		default:
			last = c
		}
		prev = c
	}

	return last
}

// yamlErrorLocation mirrors get_yaml_error_location
// (rendercv_model_builder.py:42-62). Upstream picks the start mark from the
// context mark, falling back to the problem mark, and the end mark the other
// way around. For the two shapes in `unterminatedConstructMessages`, goccy's
// token is the *context* mark — measured on `this: [is, not, a, cv`, where
// the token is the `[` that opened the flow sequence, at (1, 7) — but goccy
// exposes no second, *problem* mark of its own.
//
// **The problem mark is EOF for both measured shapes.** ruamel's scanner
// reads to the true end of the stream hunting for the closing delimiter, so
// its problem_mark's line is the total newline count of the document
// (0-indexed, so `+1` to display) regardless of where the scan started or
// how deeply the construct is nested. Widening the span to that line
// reproduces upstream's `line N to line M` exactly for both.
//
// **Scoped to the mapped shapes.** Widening every syntax error's span to EOF
// would be guessing at shapes never measured — a bad-indentation failure keeps
// its single-line span, which matches upstream there (measured: `mapping value
// is not allowed`'s span does not widen). A duplicate key also spans, but to
// the enclosing mapping's first key rather than to EOF; see mappingStartLine.
func yamlErrorLocation(parserErr goyaml.Error, content string) *yamldoc.Span {
	tok := parserErr.GetToken()
	if tok == nil || tok.Position == nil {
		return nil
	}
	start := yamldoc.Position{Line: tok.Position.Line, Column: tok.Position.Column}
	end := start

	message := parserErr.Error()

	// **The node form collapses to EOF instead of spanning to it.** ruamel puts
	// both marks at the end of the stream when it was waiting for a node, so
	// the location is a single line and not a range — `cv: [` reports `line 2`,
	// where `cv: [a` reports `line 1 to line 2`.
	if flowNodeExpectedAtEOF(message, content) {
		eof := yamldoc.Position{Line: streamEndLine(content, start.Line), Column: 1}
		return &yamldoc.Span{Start: eof, End: eof}
	}

	// **An open flow collection that a later line disturbed** spans from the
	// delimiter that opened it to wherever ruamel's scan stopped. **Neither
	// mark is goccy's token**, so both are recovered from the source: goccy
	// reports the same message at the same line whether the next line is
	// `b: c`, a bare `b`, or an indented `  b: c`, while ruamel stops at the
	// first two differently and at the third differently again.
	//
	// The scan stops at the first line carrying a **key indicator** — a colon
	// followed by whitespace or end of line — because a flow collection cannot
	// contain a block mapping. A plain scalar, a quoted scalar and `b:c` (no
	// space, so not a key) are all swallowed by the flow, and the scan runs on
	// to a document marker or the end of the file instead. Measured on all of
	// them.
	if flowInterruptedByBlockLine(message) {
		if open := outermostFlowOpenLine(content, start.Line+1); open > 0 {
			end := breakingLine(content, open)
			if end == 0 {
				end = streamEndLine(content, open)
			}
			return &yamldoc.Span{
				Start: yamldoc.Position{Line: open, Column: 1},
				End:   yamldoc.Position{Line: end, Column: 1},
			}
		}
	}

	// **A duplicate key spans the enclosing mapping, not the key.** ruamel's
	// context mark is where the *mapping* began and its problem mark is the
	// offending key, so `cv: 1\ncv: 2` is `line 1 to line 2`. The port reported
	// the key's line alone, and the comment below this one used to assert that
	// was correct.
	//
	// goccy names the key's *first occurrence* in its message, which is a
	// different line whenever the duplicated key is not the mapping's first —
	// `a: 1\nb: 2\nb: 3` is context line 1 to ruamel and first-occurrence line
	// 2 to goccy — so the mapping's start is computed from the source instead.
	if strings.Contains(message, "already defined") {
		if start.Line = mappingStartLine(content, start.Line, start.Column); start.Line < end.Line {
			return &yamldoc.Span{Start: start, End: end}
		}
	}

	for _, unterminated := range spanToEOFMessages {
		if strings.Contains(message, unterminated) {
			// **goccy's token is the delimiter it gave up on, which is not
			// always the one ruamel names.** ruamel's context mark is the
			// *outermost* flow collection still open, so `cv: [a\n  b: [c,`
			// is `line 1 to line 2` — goccy points at the inner `[` on line 2
			// and the port reported `line 2 to line 3`, a construct the user
			// did not open at a line the failure is not about. Where both
			// delimiters sit on one line the two agree, which is why every
			// earlier measured shape was unaffected.
			//
			// **Only for the flow collections.** An unterminated quoted scalar
			// shares this branch but not this rule: ruamel names the *quote*,
			// even when a flow opened on an earlier line, so `cv: [a\n  b: "c`
			// is `line 2 to line 3` and not line 1.
			if open := openFlowLine(content); isUnterminatedFlow(message) &&
				open > 0 && open < start.Line {
				start = yamldoc.Position{Line: open, Column: 1}
				if end.Line < start.Line {
					end = start
				}
			}

			// **A block line can stop this scan short of the end**, and goccy
			// routes an *indented* one here rather than to the branch above:
			// `cv: [a\n  b: c` is `line 1 to line 2`, not to EOF.
			stop := breakingLine(content, start.Line)
			if stop == 0 {
				stop = streamEndLine(content, start.Line)
			}
			if stop > end.Line {
				end = yamldoc.Position{Line: stop, Column: 1}
			}
			break
		}
	}

	return &yamldoc.Span{Start: start, End: end}
}
