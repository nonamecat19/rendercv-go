package modelbuilder

import (
	"errors"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func TestReadYamlWithValidationErrorsSyntaxError(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		source schemaerr.YamlSource
	}{
		{
			name:   "unclosed flow mapping",
			input:  "cv:\n  name: {John\n",
			source: schemaerr.SourceMain,
		},
		{
			name:   "tab indentation",
			input:  "cv:\n\tname: John\n",
			source: schemaerr.SourceDesign,
		},
		{
			name:   "unclosed quote",
			input:  "cv:\n  name: \"John\n",
			source: schemaerr.SourceLocale,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(tc.input, tc.source)
			if err == nil {
				t.Fatalf("expected a validation error, got nil")
			}

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			if len(userErr.Errors) != 1 {
				t.Fatalf("expected exactly one record, got %d", len(userErr.Errors))
			}

			record := userErr.Errors[0]
			if record.SchemaLocation != nil {
				t.Errorf("schema location = %v, want nil", record.SchemaLocation)
			}
			if record.YamlSource != tc.source {
				t.Errorf("yaml source = %q, want %q", record.YamlSource, tc.source)
			}
			if record.Input != "..." {
				t.Errorf("input = %q, want %q", record.Input, "...")
			}
			if !strings.HasPrefix(record.Message, "This is not a valid YAML file. ") {
				t.Errorf("message = %q, want the §4.17 prefix", record.Message)
			}
			if !strings.HasSuffix(record.Message, ".") {
				t.Errorf("message = %q, want a trailing period", record.Message)
			}
			if strings.Contains(record.Message, "\n") {
				t.Errorf("message = %q, want a single line", record.Message)
			}
			if record.YamlLocation == nil {
				t.Fatalf("yaml location = nil, want coordinates")
			}
			if record.YamlLocation.Start.Line < 1 || record.YamlLocation.Start.Column < 1 {
				t.Errorf("yaml location = %+v, want 1-indexed coordinates", *record.YamlLocation)
			}
		})
	}
}

func TestReadYamlWithValidationErrorsValidInput(t *testing.T) {
	node, err := ReadYamlWithValidationErrors("cv:\n  name: John Doe\n", schemaerr.SourceMain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node == nil {
		t.Fatal("expected a parsed document, got nil")
	}
}

func TestReadYamlWithValidationErrorsPassesThroughNonParserErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty input", input: ""},
		{name: "scalar string root", input: "just a string"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(tc.input, schemaerr.SourceMain)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			var userErr *schemaerr.UserValidationError
			if errors.As(err, &userErr) {
				t.Fatalf("non-parser failure was wrapped as a validation error: %v", err)
			}
		})
	}
}

func TestParserMessageNormalization(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "first line only", text: "bad thing\nsource line\n  ^", want: "bad thing."},
		{name: "already a period", text: "bad thing.", want: "bad thing."},
		{name: "surrounding space", text: "  bad thing  \nmore", want: "bad thing."},
		{name: "empty", text: "", want: "."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parserMessage(tc.text, ""); got != tc.want {
				t.Errorf("parserMessage(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// Spec §3.85 — a parser failure carrying no position yields an absent YAML
// location rather than a fabricated one.
func TestParserErrorWithoutMarks(t *testing.T) {
	record := yamlSyntaxValidationError(marklessParserError{}, "", schemaerr.SourceMain)

	if record.YamlLocation != nil {
		t.Errorf("yaml location = %+v, want absent", record.YamlLocation)
	}
	if record.Message != "This is not a valid YAML file. no marks here." {
		t.Errorf("message = %q", record.Message)
	}
}

// marklessParserError is a parser error with no offending token, which is how
// upstream's "neither mark" case reaches the location extractor.
type marklessParserError struct{}

func (marklessParserError) Error() string                { return "no marks here" }
func (marklessParserError) GetMessage() string           { return "no marks here" }
func (marklessParserError) GetToken() *token.Token       { return nil }
func (marklessParserError) FormatError(_, _ bool) string { return "no marks here" }

// Spec 004 §3.18 behaviors 68-70: everything about the record **except** the
// interpolated parser text.
//
// The interpolation is the one place in the iteration where parity is not
// currently reachable — ruamel's phrasing and goccy's differ — and spec §7.5
// makes it a decision rather than a fix. Every other member is decidable now,
// so it is pinned now: a later decision about the sentence must not be able to
// change the record's shape without failing here.
func TestYamlSyntaxRecordShape(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("cv: [\n  a: 1\n", schemaerr.SourceDesign)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
	}
	if len(userErr.Errors) != 1 {
		t.Fatalf("errors = %+v, want exactly one", userErr.Errors)
	}
	record := userErr.Errors[0]

	// Behavior 68: no schema location. The failure is about the document, not
	// about a field in it.
	if len(record.SchemaLocation) != 0 {
		t.Errorf("schema location = %v, want none", record.SchemaLocation)
	}

	// The source of the document being parsed, not the main file.
	if record.YamlSource != schemaerr.SourceDesign {
		t.Errorf("source = %q, want %q", record.YamlSource, schemaerr.SourceDesign)
	}

	// §4.15: the input echo is the three dots.
	if record.Input != "..." {
		t.Errorf("input = %q, want the three-dot echo", record.Input)
	}

	// Coordinates are 1-indexed in both line and column.
	if record.YamlLocation == nil {
		t.Fatal("coordinates are absent; the parser supplied marks")
	}
	if record.YamlLocation.Start.Line < 1 || record.YamlLocation.Start.Column < 1 {
		t.Errorf("coordinates = %+v, want them 1-indexed", *record.YamlLocation)
	}

	// The sentence's prefix is RenderCV's and is decidable; only what follows
	// is deferred.
	if !strings.HasPrefix(record.Message, "This is not a valid YAML file. ") {
		t.Errorf("message = %q, want RenderCV's prefix", record.Message)
	}
	// Whatever the parser says, the period rule of behavior 68 has run.
	if !strings.HasSuffix(record.Message, ".") {
		t.Errorf("message = %q, want a trailing period", record.Message)
	}
	// And it is one line.
	if strings.Contains(record.Message, "\n") {
		t.Errorf("message spans lines: %q", record.Message)
	}
}

// Spec 004 §7.5 and plan §6 option B: goccy's error taxonomy mapped onto
// ruamel's phrasing, for the syntax failures the corpus contains.
//
// Each `want` is ruamel's verbatim first line, measured against the vendored
// Python on the same input. The first row is the corpus case, whose golden
// output reads `This is not a valid YAML file. while parsing a flow sequence.`
func TestParserMessageUsesRuamelPhrasing(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{
			name: "an unterminated flow sequence — the corpus case",
			src:  "this: [is, not, a, cv\n",
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "an unterminated flow mapping",
			src:  "a: {b\n",
			want: "This is not a valid YAML file. while parsing a flow mapping.",
		},
		{
			name: "an unterminated quoted scalar",
			src:  "a: 'unterminated\n",
			want: "This is not a valid YAML file. while scanning a quoted scalar.",
		},
		{
			// These two reach the sentence through yamlreader.TabError, not
			// through ruamelPhrasing: the tab check runs before the parser and
			// phrases the failure itself. They pin that branch's wording, which
			// has to agree with the mapped rows below.
			name: "a tab where a key belongs",
			src:  "\ta: 1\n",
			want: "This is not a valid YAML file. while scanning for the next token.",
		},
		{
			name: "a tab indenting a nested key",
			src:  "cv:\n\tname: a\n",
			want: "This is not a valid YAML file. while scanning for the next token.",
		},
		{
			// The tabs TabError deliberately lets past, and so the only inputs
			// that exercise the two tab rows of ruamelPhrasing. Without these
			// the rows can be deleted with the suite still green, because the
			// two rows above pass on the TabError branch either way.
			//
			// A tab indenting a block scalar's content: goccy says "found a tab
			// character where an indentation space is expected".
			name: "a tab indenting a block scalar",
			src:  "a: |\n\tx\n",
			want: "This is not a valid YAML file. while scanning for the next token.",
		},
		{
			// A `[` inside a plain scalar, which the tab check reads as an open
			// flow collection (where tabs are legal) and goccy does not. goccy
			// says "found character '\t' that cannot start any token".
			name: "a tab after a bracket in a plain scalar",
			src:  "a: b[\n\tc\n",
			want: "This is not a valid YAML file. while scanning for the next token.",
		},
		{
			name: "a duplicate key",
			src:  "a: 1\na: 2\n",
			want: "This is not a valid YAML file. while constructing a mapping.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
			}
			if userErr.Errors[0].Message != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", userErr.Errors[0].Message, test.want)
			}
		})
	}
}

// TestEveryPhrasingRowIsReachable is the anti-vacuity guard for the mapping
// table: a row whose goccy substring no longer occurs for any input is dead
// weight that reads like coverage, the same class of defect as a test whose
// assertion never runs.
//
// The table is order-sensitive — `parserMessage` takes the *first* row whose
// substring matches — so reachability is asserted at the level of the row
// **index**, not of the phrasing it produces. Four rows share a `ruamel` value
// in pairs, so comparing the message alone would let a row be shadowed by its
// twin and still look alive.
//
// Two failures never reach the table at all and so are not rows: an ordinary
// tab (`\ta: 1`), which `yamlreader.TabError` intercepts before the parser
// runs, and a flow collection left waiting for a node (`cv: [`), which
// `flowNodeExpectedAtEOF` answers first. The inputs below are chosen to avoid
// both interceptors.
//
// Each `ruamel` value was read off the exception the vendored ruamel raises for
// that same input (`e.context`).
func TestEveryPhrasingRowIsReachable(t *testing.T) {
	// One input per row, in the table's own order.
	reaching := []struct{ goccy, src string }{
		{"sequence end token", "this: [is, not, a, cv\n"},
		{"']' must be specified", "cv: [a\nb: c\n"},
		{"'}' must be specified", "cv: {a: 1\nb: c\n"},
		{"flow map", "a: {b\n"},
		{"quoted text", "a: 'unterminated\n"},
		{"tab character", "a: |\n\tx\n"},
		{"cannot start any token", "a: b[\n\tc\n"},
		{"already defined", "a: 1\na: 2\n"},
	}

	if len(reaching) != len(ruamelPhrasing) {
		t.Fatalf("the table has %d rows and %d have a reaching input; every new"+
			" row needs one, measured", len(ruamelPhrasing), len(reaching))
	}

	for i, want := range reaching {
		row := ruamelPhrasing[i]
		if row.goccy != want.goccy {
			t.Fatalf("row %d is %q, but the reaching input was measured for %q",
				i, row.goccy, want.goccy)
		}

		t.Run(row.goccy, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(want.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
			}
			if got, expect := userErr.Errors[0].Message,
				"This is not a valid YAML file. "+row.ruamel+"."; got != expect {
				t.Fatalf("message =\n  %q\nwant\n  %q", got, expect)
			}

			// And it was *this* row that produced it, not an earlier one that
			// happens to carry the same phrasing. The reader is asked again for
			// the raw parser failure, because the record above has already
			// consumed it.
			_, raw := yamlreader.ReadString(want.src)
			var parserErr goyaml.Error
			if !errors.As(raw, &parserErr) {
				// The message came from a branch ahead of the table, so the row
				// itself is still unreached.
				t.Fatalf("%q never reaches the phrasing table: %v", want.src, raw)
			}
			if got := firstMatchingRow(parserErr.Error()); got != i {
				t.Errorf("%q selects row %d (%q), want row %d (%q) — the row is"+
					" shadowed and can be deleted with the suite still green",
					want.src, got, rowName(got), i, row.goccy)
			}
		})
	}
}

// firstMatchingRow is parserMessage's own row selection, exposed so a test can
// assert *which* row answered rather than only what it said.
func firstMatchingRow(text string) int {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSpace(text)
	for i, row := range ruamelPhrasing {
		if strings.Contains(text, row.goccy) {
			return i
		}
	}
	return -1
}

func rowName(i int) string {
	if i < 0 || i >= len(ruamelPhrasing) {
		return "<none>"
	}
	return ruamelPhrasing[i].goccy
}

// An unmapped failure falls through to goccy's own first line. That is option A
// for the remainder — wrong, but visibly wrong rather than silently
// misattributed to the nearest mapped construct.
//
// The assertion is that it does **not** borrow a ruamel phrase it was not
// measured for.
//
// The fixture used to be `a: !!unknowntag@@ b`, which **parses**: goccy's
// scanTag rejects only `{` and `}` and accepts every other byte in a tag, so
// the test skipped itself and asserted nothing from the day it was written.
// `!!tag{x}` is the shape that actually fails there, with goccy's own
// "found invalid tag character '{'" and its `[line:col]` prefix intact.
func TestUnmappedParserMessageFallsThrough(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("a: !!tag{x} b\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
	}

	message := userErr.Errors[0].Message
	if !strings.Contains(message, "found invalid tag character") {
		t.Errorf("message = %q, want goccy's own text to reach the user", message)
	}
	for _, row := range ruamelPhrasing {
		if strings.Contains(message, row.ruamel) {
			t.Errorf("an unmapped failure borrowed %q: %q", row.ruamel, message)
		}
	}
}

// TestUnterminatedFlowSequenceSpansToEOF is the corpus's one syntax case,
// `this: [is, not, a, cv\n`, at the level of its location — `err_not_yaml`'s
// remaining defect before this fix (`STATE.md`): the port reported `line 1`
// where upstream reports `line 1 to line 2`, because goccy's token is only
// the start of the unterminated construct (ruamel's context_mark) and has no
// second mark of its own. `yamlErrorLocation` now synthesizes ruamel's
// problem_mark as the document's true EOF for this one mapped shape.
//
// Byte-diffed against the vendored CLI this pass: identical, not just
// normalized-equal.
func TestUnterminatedFlowSequenceSpansToEOF(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("this: [is, not, a, cv\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
	}

	span := userErr.Errors[0].YamlLocation
	if span == nil {
		t.Fatal("yaml location = nil, want a start-to-EOF span")
	}
	if span.Start.Line != 1 {
		t.Errorf("start line = %d, want 1 (the opening `[`)", span.Start.Line)
	}
	if span.End.Line != 2 {
		t.Errorf("end line = %d, want 2 (EOF, one newline after the `[`)", span.End.Line)
	}
}

// TestMultiDocumentStreamIsRejected pins ruamel's composer failure. Upstream
// loads the input with a plain `YAML().load`, which composes one document and
// raises when a second begins; the port read `Docs[0]` and ignored the rest,
// so `---\ncv:\n  name: A\n---\nb: 2\n` rendered A's CV at exit 0 where
// upstream exits 1.
//
// The marks are not the failing token's: ruamel's context mark is where the
// first document's *content* began and its problem mark is the second
// document's `---`. Every pair below was read off the raised ruamel exception
// and confirmed end to end against the vendored CLI.
func TestMultiDocumentStreamIsRejected(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		{
			name: "an explicit first document",
			src:  "---\ncv:\n  name: A\n---\nb: 2\n", startLine: 2, endLine: 4,
		},
		{
			name: "an implicit first document",
			src:  "cv:\n  name: A\n---\nb: 2\n", startLine: 1, endLine: 3,
		},
		{
			name: "an end marker between the two",
			src:  "a: 1\n...\n---\nb: 2\n", startLine: 1, endLine: 3,
		},
		{
			// Three documents report the *first* extra one, not the last.
			name: "three documents",
			src:  "---\na: 1\n---\nb: 2\n---\nc: 3\n", startLine: 2, endLine: 3,
		},
	}

	const want = "This is not a valid YAML file. expected a single document in the stream."

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			if got := userErr.Errors[0].Message; got != want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, want)
			}

			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// A single document carrying the optional `---` and `...` markers is not a
// multi-document stream, and must keep parsing as it always did.
func TestSingleDocumentWithMarkersStillParses(t *testing.T) {
	for _, src := range []string{"---\ncv:\n  name: A\n", "a: 1\n...\n", "---\na: 1\n...\n"} {
		t.Run(src, func(t *testing.T) {
			if _, err := ReadYamlWithValidationErrors(src, schemaerr.SourceMain); err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

// TestFlowNodeShapeReportsAtEOF pins the second of ruamel's two phrasings for
// an unterminated flow collection. goccy reports `cv: [` and `cv: [a`
// identically (`sequence end token ']' not found`), but ruamel does not: it
// says `while parsing a flow node` when the stream ended while it was waiting
// for a *node*, and puts both of its marks at EOF, so the location is a single
// line rather than a span.
//
// Before this fix the port answered `line 1 to line 2` / `flow sequence` for
// every one of these, which relaid the whole error table and put it 186 bytes
// away from upstream's.
//
// Every want below is measured against ruamel on the same input: `context`,
// `context_mark.line+1` and `problem_mark.line+1` read off the raised
// exception, then confirmed end to end against the vendored CLI.
func TestFlowNodeShapeReportsAtEOF(t *testing.T) {
	tests := []struct {
		name string
		src  string
		line int
	}{
		{name: "an empty flow sequence", src: "cv: [\n", line: 2},
		{name: "an empty flow mapping", src: "cv: {\n", line: 2},
		{name: "a trailing comma in a sequence", src: "cv: [a,\n", line: 2},
		{name: "a trailing comma in a mapping", src: "cv: {a: 1,\n", line: 2},
		{name: "a comment after the delimiter", src: "cv: [ # hi\n", line: 2},
		{name: "a blank line after the delimiter", src: "cv: [\n   \n", line: 3},
		{name: "a nested empty sequence", src: "cv: [[\n", line: 2},
		{name: "a comment after a comma", src: "cv: [a, # hi\n", line: 2},
	}

	const want = "This is not a valid YAML file. while parsing a flow node."

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			if got := userErr.Errors[0].Message; got != want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, want)
			}

			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a single-line location at EOF")
			}
			// Both of ruamel's marks are at EOF here, so the two must be equal:
			// an unequal pair renders as `line N to line M` and would be the
			// sequence form's answer.
			if span.Start.Line != test.line || span.End.Line != test.line {
				t.Errorf("location = line %d to line %d, want line %d only",
					span.Start.Line, span.End.Line, test.line)
			}
		})
	}
}

// TestUnterminatedQuotedScalarSpansToEOF is the third construct whose scanner
// runs to the end of the stream. goccy's token is the opening quote — ruamel's
// context_mark — and ruamel's problem_mark is EOF, so the location is a span
// and not the single line the port reported before this fix.
//
// The last row is the reason the quoted case is kept out of
// unterminatedConstructMessages: its content ends in a comma, which is the
// flow-node discriminator, but ruamel calls it a quoted-scalar failure.
func TestUnterminatedQuotedScalarSpansToEOF(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		endLine int
	}{
		{name: "a double-quoted scalar", src: "cv: \"a\n", endLine: 2},
		{name: "a single-quoted scalar", src: "cv: 'a\n", endLine: 2},
		{name: "spanning several lines", src: "cv: \"a\nb\nc\n", endLine: 4},
		{name: "inside a flow sequence", src: "cv: ['a\n", endLine: 2},
		{name: "content ending in a comma", src: "cv: [\"a,\n", endLine: 2},
	}

	const want = "This is not a valid YAML file. while scanning a quoted scalar."

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			if got := userErr.Errors[0].Message; got != want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, want)
			}

			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a start-to-EOF span")
			}
			if span.Start.Line != 1 || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line 1 to line %d",
					span.Start.Line, span.End.Line, test.endLine)
			}
		})
	}
}

// TestQuotedContentIsNotADelimiter guards the quote-awareness of
// lastSignificantByte: a `[` or a `#` inside a quoted scalar is content, so
// these stay the *sequence* form. Measured against ruamel: both report
// `while parsing a flow sequence`, line 1 to line 2.
func TestQuotedContentIsNotADelimiter(t *testing.T) {
	for _, src := range []string{"cv: [\"a#b\"\n", "cv: [\"a[\"\n"} {
		t.Run(src, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			const want = "This is not a valid YAML file. while parsing a flow sequence."
			if got := userErr.Errors[0].Message; got != want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, want)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil || span.Start.Line != 1 || span.End.Line != 2 {
				t.Errorf("location = %+v, want line 1 to line 2", span)
			}
		})
	}
}

// TestUnterminatedFlowMappingSpansToEOF is the flow-mapping half of the same
// fix: goccy's token for `{John` unterminated is the `{` (the context mark),
// the same shape as the flow-sequence case, just a different delimiter.
// Measured against the vendored CLI: `line 2 to line 3`. A first version of
// this fix widened only the flow-*sequence* message and left this shape
// narrow — a fresh-context verifier caught it by sweeping more inputs than
// the corpus has.
func TestUnterminatedFlowMappingSpansToEOF(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("cv:\n  name: {John\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
	}

	span := userErr.Errors[0].YamlLocation
	if span == nil {
		t.Fatal("yaml location = nil, want a start-to-EOF span")
	}
	if span.Start.Line != 2 {
		t.Errorf("start line = %d, want 2 (the opening `{`)", span.Start.Line)
	}
	if span.End.Line != 3 {
		t.Errorf("end line = %d, want 3 (EOF)", span.End.Line)
	}
}

// TestSpanWideningIsScopedToTheMappedCases guards the other half: a syntax
// error that is not one of the two unterminated-construct shapes must keep a
// single-line span rather than being pushed to EOF on the assumption every
// syntax error works the same way — a guess the corpus cannot check. Bad
// indentation is measured (against the vendored CLI) to stay single-line
// upstream too, unlike the two unterminated-construct shapes.
func TestSpanWideningIsScopedToTheMappedCases(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("cv:\n  name: John\n   bad: 1\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
	}

	span := userErr.Errors[0].YamlLocation
	if span == nil {
		t.Fatal("yaml location = nil")
	}
	if span.Start != span.End {
		t.Errorf("span = %+v, want a single-line span (not an unterminated construct)", span)
	}
}

// TestDuplicateKeySpansItsMapping pins ruamel's third span shape. Its context
// mark for a duplicate key is where the enclosing **mapping** began, not the
// key's first occurrence and not the key itself, so `cv: 1\ncv: 2` reports
// `line 1 to line 2` where the port reported `line 2` alone.
//
// goccy's own message names the first occurrence (`already defined at [1:1]`),
// which is a *different* line whenever the duplicated key is not the mapping's
// first — the `dup not first` rows below are exactly that case, and taking
// goccy's number there would be wrong. The mapping's start is computed from
// the source instead.
//
// Every pair was read off the raised ruamel exception.
func TestDuplicateKeySpansItsMapping(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		{name: "adjacent", src: "cv: 1\ncv: 2\n", startLine: 1, endLine: 2},
		{name: "distant", src: "cv: 1\nx: 2\ny: 3\ncv: 4\n", startLine: 1, endLine: 4},
		{name: "dup not first", src: "a: 1\nb: 2\nb: 3\n", startLine: 1, endLine: 3},
		{name: "dup not first, nested", src: "x:\n  p: 0\n  q: 1\n  q: 2\n", startLine: 2, endLine: 4},
		{name: "nested", src: "a:\n  b: 1\n  b: 2\n", startLine: 2, endLine: 3},
		{name: "deeply nested", src: "x:\n  y:\n    k: 1\n    k: 2\n", startLine: 3, endLine: 4},
		{name: "a value block between", src: "a: 1\nb:\n  c: 1\na: 2\n", startLine: 1, endLine: 4},
		{name: "blank lines between", src: "a: 1\n\n\na: 2\n", startLine: 1, endLine: 4},
		{name: "a comment between", src: "a: 1\n# hi\na: 2\n", startLine: 1, endLine: 3},
		{name: "a sequence of mappings", src: "x:\n  - a: 1\n    a: 2\n", startLine: 2, endLine: 3},
		{name: "after a document marker", src: "---\na: 1\na: 2\n", startLine: 2, endLine: 3},
	}

	// `mappingStartLine`'s doc comment names this count and enumerates these
	// shapes. Prose drifts silently — it already said "ten" while eleven rows
	// stood here — so the number is asserted rather than described.
	if len(tests) != 11 {
		t.Fatalf("%d shapes, but mappingStartLine's comment claims eleven;"+
			" update both together", len(tests))
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// Bad indentation is the shape that genuinely has no context mark, so it must
// keep a single-line location. It is the control for the span rules above.
func TestBadIndentationStaysSingleLine(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("a: 1\n  b: 2\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
	}
	span := userErr.Errors[0].YamlLocation
	if span == nil {
		t.Fatal("yaml location = nil")
	}
	if span.Start.Line != span.End.Line {
		t.Errorf("location = line %d to line %d, want a single line",
			span.Start.Line, span.End.Line)
	}
}

// TestFlowInterruptedByABlockLine pins ruamel's fourth span shape, and the
// third distinct phrasing for an unterminated flow collection.
//
// goccy reports `',' or ']' must be specified` when a block line breaks an
// open flow collection, which no phrasing row covered — so the raw goccy text,
// `[line:col]` prefix included, reached the user, at the wrong location.
//
// It is **not** the unterminated-to-EOF shape: ruamel stops at the line that
// broke the flow rather than running to the end of the stream, so
// `cv: [a\nb: c\nd: e` is `line 1 to line 2`. Here goccy's token is ruamel's
// *problem* mark, the opposite of the EOF shapes where it is the context mark,
// and the opening delimiter is recovered from the source.
//
// The last row is the control: `c: d` is legal flow content, so that document
// really does run to EOF and must keep taking the older path.
func TestFlowInterruptedByABlockLine(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
		want               string
	}{
		{
			name: "a block line after a flow sequence", src: "cv: [a\nb: c\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "the break is not the end of the file", src: "cv: [a\nb: c\nd: e\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "a block line after a flow mapping", src: "cv: {a: 1\nb: c\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow mapping.",
		},
		{
			// The outermost delimiter, not the nearest.
			name: "nested flow collections", src: "cv: [[a\nb: c\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "the flow opens on a later line", src: "x: 1\ncv: [a\nb: c\n",
			startLine: 2, endLine: 3,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			// `c: d` is a legal flow pair, so this one genuinely reaches EOF.
			name: "a legal flow pair runs to EOF", src: "cv: [a,\n  b,\nc: d\n",
			startLine: 1, endLine: 4,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		// The breaking line ending in a **comma** is the shape STATE.md's pass
		// 22 recorded as a fourth, unmapped goccy phrasing leaking raw text at
		// the wrong location. goccy in fact says `',' or ']' must be specified`
		// here, the same as without the comma, so the row above already covers
		// it — these pin that, because the trailing comma is otherwise the
		// discriminator `flowNodeExpectedAtEOF` keys on and misrouting it would
		// silently give the flow-*node* answer at a single line.
		{
			name: "the breaking line ends in a comma", src: "cv: [a\nb: c,\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "a comma-ended break is not the end of the file", src: "cv: [a\nb: c,\nd: e\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "a comma-ended break in a flow mapping", src: "cv: {a: 1\nb: c,\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow mapping.",
		},
		{
			// Indented, which goccy routes through `sequence end token` instead.
			name: "an indented comma-ended break", src: "cv: [a\n  b: c,\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "nested, with a comma-ended break", src: "cv: [[a\nb: c,\n",
			startLine: 1, endLine: 2,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			name: "a comma-ended break below another key", src: "x: 1\ncv: [a\nb: c,\n",
			startLine: 2, endLine: 3,
			want: "This is not a valid YAML file. while parsing a flow sequence.",
		},
		{
			// The control: the flow was already expecting an element, so `b: c`
			// is a legal single-pair flow mapping and the *trailing* comma is
			// what ends the stream — the flow-node form, at EOF alone.
			name: "a comma-ended break the flow swallows", src: "cv: [a,\nb: c,\n",
			startLine: 3, endLine: 3,
			want: "This is not a valid YAML file. while parsing a flow node.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			if got := userErr.Errors[0].Message; got != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, test.want)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// TestFlowNodeDiscriminatorAcrossABreak pins the interaction the trailing-comma
// fix turned on: `flowNodeExpectedAtEOF` reads the last significant character
// of the file, and that character only answers its question when ruamel's scan
// actually reached the end.
//
// Every row is a shape where the two halves disagree — the file ends in `[`,
// `{` or `,` (the node-form discriminator) while a block line did or did not
// break the flow first. Read off ruamel's own `context`, `context_mark` and
// `problem_mark` for the same input.
func TestFlowNodeDiscriminatorAcrossABreak(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
		want               string
	}{
		// Broken first by an **indented** line, which is the half goccy routes
		// through `sequence end token` and so the half the discriminator
		// actually decides. Unindented breaks reach it as `']' must be
		// specified`, which is not an unterminated-construct message at all.
		{
			name: "nested, with an indented comma-ended break", src: "cv: [[a\n  b: c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "an indented comma-ended break below another key", src: "x: 1\ncv: [a\n  b: c,\n",
			startLine: 2, endLine: 3, want: "while parsing a flow sequence",
		},
		{
			name: "an indented break, then a bare opening bracket", src: "cv: [a\n  b: c\n  d: [\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "an indented comma-ended break, then an end marker", src: "cv: [a\n  b: c,\n...\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "an indented comma-ended break in a flow mapping", src: "cv: {a: 1\n  b: c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow mapping",
		},
		// Broken first, so the delimiter or comma at EOF is never reached.
		{
			name: "a break, then a bare opening bracket", src: "cv: [a\nb: c\nd: [\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a break, then a bare opening brace", src: "cv: [a\nb: c\nd: {\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a break whose own line opens a flow", src: "cv: [a\nb: [c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a comma-ended break past a start marker", src: "cv: [a\n---\nb: c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a break, then an end marker", src: "cv: [a\nb: c\n...\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		// Never broken, so the trailing comma really is where the stream ended.
		{
			name: "an empty sequence swallows the pair", src: "cv: [\nb: c,\n",
			startLine: 3, endLine: 3, want: "while parsing a flow node",
		},
		{
			name: "an empty mapping swallows the pair", src: "cv: {\nb: c,\n",
			startLine: 3, endLine: 3, want: "while parsing a flow node",
		},
		{
			name: "an indented pair after a comma", src: "cv: [a,\n  b: c,\n",
			startLine: 3, endLine: 3, want: "while parsing a flow node",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}

			want := "This is not a valid YAML file. " + test.want + "."
			if got := userErr.Errors[0].Message; got != want {
				t.Errorf("message =\n  %q\nwant\n  %q", got, want)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a location")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// TestScanStopsAtADocumentMarker pins that a `---` or `...` ends the stream
// ruamel's scanner was reading, so an unterminated construct spans to the
// marker rather than to the physical end of the file.
//
// A marker *before* the construct began does not count, and a marker must be
// unquoted at column 1 — a `"..."` scalar is ordinary content. Measured
// against ruamel's own marks on every row.
func TestScanStopsAtADocumentMarker(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		{name: "an end marker", src: "cv: [a\n...\n", startLine: 1, endLine: 2},
		// The `b: 1` here belongs to the *next* document, so it is not the line
		// that broke this flow — the marker ends the scan before it.
		{name: "a start marker", src: "cv: [a\n---\nb: 1\n", startLine: 1, endLine: 2},
		{name: "a marker further down", src: "cv: [a\nb\n...\n", startLine: 1, endLine: 3},
		// A leading marker precedes the flow, so the scan still reaches EOF.
		{name: "a leading marker does not clamp", src: "---\ncv: [a\n", startLine: 2, endLine: 3},
		{name: "leading and trailing markers", src: "---\ncv: [a\n...\n", startLine: 2, endLine: 3},
		// Quoted dots are content, not a marker, so this one runs to EOF.
		{name: "dots inside a quoted scalar", src: "cv: [a\n\"...\"\n", startLine: 1, endLine: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// TestAKeyIndicatorOnlyBreaksASatisfiedFlow pins the rule that decides whether
// a later line ends an open flow collection or is swallowed by it.
//
// A colon followed by whitespace makes a line a block-mapping entry, which a
// flow collection cannot contain — but only when the flow is not already
// expecting an element. Straight after a comma or the opening delimiter the
// same text is a legal single-pair flow mapping, and the scan runs on.
//
// The empty-flow row is the control for the marker interaction: `cv: [` with a
// `...` after it is the flow-*node* form, reported at the marker's line alone.
func TestAKeyIndicatorOnlyBreaksASatisfiedFlow(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		{name: "a block line after an element", src: "cv: [a\nb: c\n", startLine: 1, endLine: 2},
		{name: "indented, which goccy routes elsewhere", src: "cv: [a\n  b: c\n", startLine: 1, endLine: 2},
		// After a comma the flow wants an element, and `c: d` is one.
		{name: "a flow pair after a comma", src: "cv: [a,\n  b,\nc: d\n", startLine: 1, endLine: 4},
		// No space after the colon, so not a key indicator at all.
		{name: "a colon with no space", src: "cv: [a\nb:c\n", startLine: 1, endLine: 3},
		{name: "a bare scalar continues the flow", src: "cv: [a\nb\n", startLine: 1, endLine: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}
