package modelbuilder

import (
	"errors"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
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
			if got := parserMessage(tc.text, "", yamldoc.Position{}); got != tc.want {
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
// Each `ruamel` value below was read off the exception the vendored ruamel
// raises for that same input (`e.context`) and is **written out here**, not
// taken from the row under test. The equality half used to read
// `row.ruamel`, which made it a tautology: corrupting a table value moved the
// expectation with the answer and the assertion stayed green. It is the
// measured constant that gives this test its teeth; the row is what is on
// trial.
//
// Rows are found by their `goccy` substring rather than by position, so
// reordering the table is not a failure, and every row is required to be
// covered by name at the end — a row nothing names is exactly the dead weight
// this test exists to catch.
func TestEveryPhrasingRowIsReachable(t *testing.T) {
	// One input per row, each with ruamel's own phrasing for it.
	reaching := []struct{ goccy, ruamel, src string }{
		{"sequence end token", "while parsing a flow sequence", "this: [is, not, a, cv\n"},
		{"']' must be specified", "while parsing a flow sequence", "cv: [a\nb: c\n"},
		{"'}' must be specified", "while parsing a flow mapping", "cv: {a: 1\nb: c\n"},
		{"unexpected map key", "while parsing a flow sequence", "cv: [a\n  b: {c,\n"},
		{"flow map", "while parsing a flow mapping", "a: {b\n"},
		{"quoted text", "while scanning a quoted scalar", "a: 'unterminated\n"},
		{"tab character", "while scanning for the next token", "a: |\n\tx\n"},
		{"cannot start any token", "while scanning for the next token", "a: b[\n\tc\n"},
		{"already defined", "while constructing a mapping", "a: 1\na: 2\n"},
		{
			"mapping value is not allowed in this context",
			"mapping values are not allowed here",
			"cv:\n  name: John\n   bad: 1\n",
		},
		{
			"value is not allowed in this context",
			"while parsing a block collection",
			"cv:\n  - name: John\n  bad: 1\n",
		},
		{
			"non-map value is specified",
			"while scanning a simple key",
			"cv:\n  name: John\nbad\n",
		},
	}

	covered := make(map[int]bool, len(reaching))

	for _, want := range reaching {
		i := rowIndex(want.goccy)
		if i < 0 {
			t.Errorf("no row carries %q, but an input was measured for it", want.goccy)
			continue
		}
		covered[i] = true

		t.Run(want.goccy, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(want.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
			}
			if got, expect := userErr.Errors[0].Message,
				"This is not a valid YAML file. "+want.ruamel+"."; got != expect {
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
					want.src, got, rowName(got), i, want.goccy)
			}
		})
	}

	for i, row := range ruamelPhrasing {
		if !covered[i] {
			t.Errorf("row %d (%q) has no reaching input; every new row needs one,"+
				" measured", i, row.goccy)
		}
	}
}

// rowIndex is the position of the row carrying exactly this goccy substring, or
// -1 when the table has none.
func rowIndex(goccy string) int {
	for i, row := range ruamelPhrasing {
		if row.goccy == goccy {
			return i
		}
	}
	return -1
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
			// The inner `[` opens on line 2, so goccy's token and ruamel's
			// context mark are different lines here and nowhere else.
			name: "an indented break whose line opens a flow", src: "cv: [a\n  b: [c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
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

// TestOutermostDelimiterIsTheContextMark pins whose delimiter ruamel names when
// an unterminated flow collection is nested across lines.
//
// goccy's token is the delimiter *it* gave up on, which is the inner one:
// `cv: [a\n  b: [c,` came out as `line 2 to line 3`, naming a collection the
// user did not open and a line the failure is not about. ruamel's context mark
// is the outermost collection still open, `line 1 to line 2`. Where both
// delimiters sit on the same line the two agree, which is why every shape
// measured before this one was unaffected.
//
// The quoted rows are the control: an unterminated quoted scalar shares the
// span shape but keeps its *own* line even with a flow open above it.
func TestOutermostDelimiterIsTheContextMark(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
		want               string
	}{
		{
			name: "an inner sequence opening on the break", src: "cv: [a\n  b: [c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		// `cv: [a\n  b: {c,` is this same break under goccy's fourth phrasing,
		// `unexpected map key`. It and its nine siblings are pinned by
		// TestFlowMapUnderAnOpenFlowSequence, which is where that phrasing's
		// own evidence lives.
		{
			name: "an inner sequence with no trailing comma", src: "cv: [a\n  b: [c\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			// The outer flow itself opens on line 2, so that is the answer —
			// the rule is "outermost still open", not "line 1".
			name: "the outer flow opens below the root", src: "cv:\n  x: [a\n    y: [b,\n",
			startLine: 2, endLine: 3, want: "while parsing a flow sequence",
		},
		{
			// A closed flow above must not be mistaken for the open one.
			name: "a closed flow on an earlier line", src: "a: [1]\ncv: [x\n  y: [z,\n",
			startLine: 2, endLine: 3, want: "while parsing a flow sequence",
		},
		{
			// Both delimiters on one line: goccy and ruamel already agree.
			name: "both delimiters on one line", src: "cv: [a, [b\n  c: d,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "an empty outer flow swallows the pair", src: "cv: [\n  a: [b,\n",
			startLine: 3, endLine: 3, want: "while parsing a flow node",
		},
		// Controls: a quoted scalar keeps its own line.
		{
			name: "a single-quoted scalar under an open flow", src: "cv: [a,\n  b: 'c\n",
			startLine: 2, endLine: 3, want: "while scanning a quoted scalar",
		},
		{
			name: "a double-quoted scalar under an open flow", src: "cv: [a\n  b: \"c\n",
			startLine: 2, endLine: 3, want: "while scanning a quoted scalar",
		},
		{
			name: "a quoted scalar on the flow's own line", src: "cv: ['a\n",
			startLine: 1, endLine: 2, want: "while scanning a quoted scalar",
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

// TestFlowMapUnderAnOpenFlowSequence pins goccy's *fourth* phrasing for a block
// line that breaks an open flow collection.
//
// When the breaking line's value opens a flow mapping, goccy abandons its three
// earlier spellings and says `unexpected map key` — a message no
// `ruamelPhrasing` row covered, so its raw text reached the user with goccy's
// own `[2:6]` position prefix still on it, at a location goccy's token supplied
// rather than ruamel's marks. ruamel calls every one of these
// `while parsing a flow sequence`, from the line the sequence opened on to the
// line the block key broke it.
//
// The 35 inputs goccy answers this way (found by enumerating every combination
// of an opening delimiter, a first element, an indent of 0-6 and nine inner
// values) are **all** open flow *sequences*: no flow mapping reaches this
// phrasing, so the row is unconditional. Every row below was read off ruamel's
// own `context`, `context_mark` and `problem_mark` for that exact input.
func TestFlowMapUnderAnOpenFlowSequence(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		{
			name: "a flow map opening on the breaking line", src: "cv: [a\n  b: {c,\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "the inner map has no trailing comma", src: "cv: [a\n  b: {c\n",
			startLine: 1, endLine: 2,
		},
		{
			// The inner map is complete; it is the *outer* sequence that is
			// still open, which is why ruamel names the sequence either way.
			name: "the inner map is closed", src: "cv: [a\n  b: {c}\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "the inner map holds a pair", src: "cv: [a\n  b: {c: d,\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "the inner map is empty", src: "cv: [a\n  b: {\n",
			startLine: 1, endLine: 2,
		},
		{
			// goccy's phrasing here is indentation-sensitive; a one-space
			// indent with a two-character key reaches it too.
			name: "a one-space indent", src: "cv: [a\n bb: {c,\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "a key containing a space", src: "cv: [a, b\n   b c: {c,\n",
			startLine: 1, endLine: 2,
		},
		{
			// A closed flow above must not be mistaken for the open one.
			name: "a closed flow on an earlier line", src: "a: [1]\ncv: [x\n  y: {z,\n",
			startLine: 2, endLine: 3,
		},
		{
			name: "a second block line after the break", src: "cv: [a\n  b: {c,\n  d: e\n",
			startLine: 1, endLine: 2,
		},
		{
			// The scan stopped on line 2, so the marker below it changes
			// nothing — the span still ends where the block key broke the flow.
			name: "a document marker after the break", src: "cv: [a\n  b: {c,\n...\n",
			startLine: 1, endLine: 2,
		},
	}

	const want = "This is not a valid YAML file. while parsing a flow sequence."

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
				t.Fatal("yaml location = nil, want a location")
			}
			if span.Start.Line != test.startLine || span.End.Line != test.endLine {
				t.Errorf("location = line %d to line %d, want line %d to line %d",
					span.Start.Line, span.End.Line, test.startLine, test.endLine)
			}
		})
	}
}

// TestFlowMapPhrasingFollowsTheOpenDelimiter pins which construct ruamel names
// when goccy says it could not finish a flow *mapping*.
//
// goccy names the collection **it** was building; ruamel names the one it was
// *parsing* when it stopped, which is the innermost flow collection open at
// that point — and those disagree whenever the inner `{` sits on a line the
// block key broke. The `flow map` row answered `while parsing a flow mapping`
// unconditionally, so `cv: [a\n   b: {c,` claimed a mapping the user never
// opened.
//
// The condition cannot come from goccy's text: goccy chooses between
// `could not find flow map content` and its two `must be specified` spellings
// by indentation and key length, so `cv: [a\n  b: {c,` and `cv: [a\n   b: {c,`
// — one space apart — take different branches to the same ruamel answer. It
// comes from the source instead.
//
// Enumerated: 309 inputs reach this row's substring (265 `flow map content`,
// 44 `flow mapping end token '}'`). ruamel answers `flow node` for 168 (a
// branch ahead of the table), `flow mapping` for 125 and `flow sequence` for
// 16. The 16 are the rows below; the mapping rows are the control, because the
// point is not to trade one misphrasing for the other.
func TestFlowMapPhrasingFollowsTheOpenDelimiter(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
		want               string
	}{
		// The innermost collection open at the break is a sequence.
		{
			name: "a three-space indent", src: "cv: [a\n   b: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a four-space indent", src: "cv: [a\n    b: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a two-character key", src: "cv: [a\n  bb: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a key containing a space", src: "cv: [a\n   b c: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a doubled sequence", src: "cv: [[a\n    b: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "a sequence opening below the root", src: "cv:\n  x: [a\n    y: {b,\n",
			startLine: 2, endLine: 3, want: "while parsing a flow sequence",
		},
		// The control: the innermost open collection really is a mapping, and
		// must keep saying so.
		{
			name: "a mapping opened on the first line", src: "cv: [a, {b\n  b: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow mapping",
		},
		{
			name: "a mapping under a sequence, wider indent", src: "cv: [a, {b\n    b: {c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow mapping",
		},
		{
			name: "a bare unterminated mapping", src: "a: {b\n",
			startLine: 1, endLine: 2, want: "while parsing a flow mapping",
		},
	}
	// The shape `cv: [a,\n  b: {c` — where the outer sequence was still
	// expecting an element, so it swallowed the block line and the inner `{`
	// ran to EOF — is deliberately absent. Its *phrasing* is already right
	// (`flow mapping`, measured), but its location is not: the port reports
	// line 1 to line 3 where ruamel reports line 2 to line 3, because the
	// context mark follows the same innermost rule this test pins for the
	// phrasing. Twelve inputs in the enumeration are in that class; they are a
	// location defect of their own and are not fixed here.

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

// TestBadIndentationReportsTheOffendingLine pins the failure an ordinary typo
// produces: a key indented deeper than its siblings.
//
// Two defects in one shape, both live at specs/012-cli/gaps.md:80-83.
// `cv:\n  name: John\n   bad: 1` is upstream's `mapping values are not allowed
// here.` at line 3; the port said `[2:9] mapping value is not allowed in this
// context.` at line 2 — goccy's own text with its coordinate prefix intact, at
// the line *above* the one the user mistyped.
//
// ruamel reports no context mark for this failure, so the location is a single
// line and not a span: its problem mark is the offending line, at the column of
// that line's colon. goccy's token is instead the end of the *previous* line's
// value, which is why the line came out one too small — and not always by one:
// a blank line between them makes it two.
//
// Enumerated over indent width (1-7), key length, nesting depth, whether the
// bad line has successors, and the edge shapes below: 115 inputs produce
// goccy's `mapping value is not allowed in this context`, and every one of them
// is ruamel's `mapping values are not allowed here` at the first following line
// carrying a key indicator. The correspondence is exact in both line and
// column on all 115.
func TestBadIndentationReportsTheOffendingLine(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		line, column int
	}{
		{
			name: "the shape from the gap report", src: "cv:\n  name: John\n   bad: 1\n",
			line: 3, column: 7,
		},
		{
			name: "a deeper indent", src: "cv:\n  name: John\n       bad: 1\n",
			line: 3, column: 11,
		},
		{
			name: "a longer key", src: "cv:\n  name: John\n   longerkey: 1\n",
			line: 3, column: 13,
		},
		{
			name: "at the top level", src: "a: 1\n b: 2\n",
			line: 2, column: 3,
		},
		{
			name: "two levels deep", src: "cv:\n  a:\n    b: 1\n     c: 2\n",
			line: 4, column: 7,
		},
		{
			// The good lines after it do not move the answer.
			name: "the bad line has successors", src: "cv:\n  name: John\n   bad: 1\n  ok: 2\n",
			line: 3, column: 7,
		},
		{
			// **Not always the next line.** A blank line between goccy's token
			// and the mistyped key puts them two apart.
			name: "a blank line before it", src: "cv:\n  name: John\n\n   bad: 1\n",
			line: 4, column: 7,
		},
		{
			name: "under a document marker", src: "---\ncv:\n  name: John\n   bad: 1\n",
			line: 4, column: 7,
		},
		{
			name: "no trailing newline", src: "cv:\n  name: John\n   bad: 1",
			line: 3, column: 7,
		},
		{
			// ruamel points *inside* the quotes here — its column is the first
			// colon on the line, not the one that makes the line a key.
			name: "a quoted key containing a colon", src: "cv:\n  name: John\n   \"a: b\": 1\n",
			line: 3, column: 6,
		},
		{
			name: "a value containing a colon", src: "cv:\n  name: John\n   bad: \"v: w\"\n",
			line: 3, column: 7,
		},
		{
			name: "carriage returns", src: "cv:\n  name: John\r\n   bad: 1\r\n",
			line: 3, column: 7,
		},
	}

	const want = "This is not a valid YAML file. mapping values are not allowed here."

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
				t.Fatal("yaml location = nil, want a location")
			}
			// No context mark upstream, so both marks are the same place.
			if span.Start.Line != test.line || span.End.Line != test.line {
				t.Errorf("location = line %d to line %d, want line %d alone",
					span.Start.Line, span.End.Line, test.line)
			}
			if span.Start.Column != test.column {
				t.Errorf("column = %d, want %d", span.Start.Column, test.column)
			}
		})
	}
}

// TestContextMarkIsTheInnermostOpenFlow pins where ruamel's context mark
// points when an unterminated flow collection is nested across lines.
//
// The port named the *outermost* collection still open at the end of the file.
// ruamel names the innermost one open **where the parser stopped**, and the two
// differ whenever the inner delimiter opens on a later line than the outer:
// `cv: [\n  b: {c` is ruamel's line 2 to line 3, and the port reported line 1
// to line 3 — a collection the failure is not about.
//
// Where the parser stopped is the earlier of two places, both recoverable from
// the source and goccy's own error:
//
//   - the block line that broke the flow, when one did (`cv: [a\n  b: [c,`
//     stops at the `:` on line 2, so the inner `[` after it was never opened
//     and the answer is the outer one on line 1);
//   - the token goccy names, which is **inclusive** when goccy ran to the end
//     of the stream — it names the delimiter it could not close, and that one
//     is open — and **exclusive** when goccy names an offending token, which
//     was never consumed (`cv: [a\n  {d` fails *at* the `{`, so the `{` is not
//     open and the answer is the `[` on line 1).
//
// Measured over 1050 enumerated shapes: 458 start lines disagreed with ruamel
// before, 12 after, and no shape whose start line was already right changes.
// The 12 are not a location defect at all — see
// TestGoccyRejectsAFoldedFlowScalar.
func TestContextMarkIsTheInnermostOpenFlow(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
		want               string
	}{
		// The swallowed shapes: the outer flow was still expecting an element,
		// so the block line became one and the inner delimiter on it is what
		// ran to EOF.
		{
			name: "an empty sequence swallows the pair", src: "cv: [\n  b: {c\n",
			startLine: 2, endLine: 3, want: "while parsing a flow mapping",
		},
		{
			name: "a trailing comma swallows the pair", src: "cv: [a,\n  b: {c\n",
			startLine: 2, endLine: 3, want: "while parsing a flow mapping",
		},
		{
			name: "an inner sequence swallowed", src: "cv: [\n  b: [c\n",
			startLine: 2, endLine: 3, want: "while parsing a flow sequence",
		},
		{
			name: "a mapping value swallows the pair", src: "cv: {a: [\n  b: {c\n",
			startLine: 2, endLine: 3, want: "while parsing a flow mapping",
		},
		{
			name: "an empty mapping swallows the pair", src: "cv: {\n  b: {c\n",
			startLine: 2, endLine: 3, want: "while parsing a flow mapping",
		},
		// The controls: a block line that really did break the flow still
		// reports the collection that was open *before* it.
		{
			name: "a block line breaks the flow", src: "cv: [a\n  b: [c,\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			name: "the corpus case", src: "this: [is, not, a, cv\n",
			startLine: 1, endLine: 2, want: "while parsing a flow sequence",
		},
		{
			// A quoted scalar keeps its own line either way.
			name: "a quoted scalar under an open flow", src: "cv: [a\n  b: 'c\n",
			startLine: 2, endLine: 3, want: "while scanning a quoted scalar",
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

	// **The shapes where goccy's own token is the stop**, asserted on the start
	// line alone. Their *end* line is wrong for a different reason — the scan's
	// stopping point is read from the source rather than from the token, which
	// is a defect of its own and not this one — so pinning the whole span here
	// would pin a value ruamel does not report. ruamel's measured spans are
	// line 1 to line 2 and line 2 to line 3 respectively; the port reports the
	// ends as line 3 and line 4.
	startOnly := []struct {
		name      string
		src       string
		startLine int
	}{
		{
			// goccy fails *at* the `{`, which is therefore not open: the
			// answer is the sequence that was.
			name: "an offending token is not open", src: "cv: [a\n  {d\n", startLine: 1,
		},
		{
			name: "an offending token two levels in", src: "cv: [\n  b: [c\n  {h\n", startLine: 2,
		},
	}

	for _, test := range startOnly {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			span := userErr.Errors[0].YamlLocation
			if span == nil {
				t.Fatal("yaml location = nil, want a location")
			}
			if span.Start.Line != test.startLine {
				t.Errorf("start line = %d, want %d", span.Start.Line, test.startLine)
			}
		})
	}
}

// TestGoccyRejectsAFoldedFlowScalar records why twelve shapes in the
// enumeration cannot be located correctly, and it is **not** a location bug.
//
// A plain scalar may span lines inside a flow collection, and ruamel folds it
// in both kinds: `cv: {a: 1\n  d}` loads as `{'cv': {'a': '1 d'}}` and
// `cv: [a\n  b]` as `{'cv': ['a b']}`. goccy folds it in a flow *sequence* and
// rejects it in a flow *mapping*, with `',' or '}' must be specified`.
//
// So for `cv: {a: 1\n  d,` the two parsers disagree about what the document
// says, not about where an error is: goccy stops at line 2 where ruamel reads
// on to the end of the stream. No rule over goccy's token can recover ruamel's
// mark there, because goccy's token is a position in a parse ruamel never made.
//
// **The wider consequence is a document the port rejects and upstream renders**
// — `cv: {a: 1\n  d}` is valid YAML — which is a bigger finding than the
// location residue and belongs to whoever takes it on. This test pins the
// divergence so it cannot be mistaken for a fix's leftovers.
func TestGoccyRejectsAFoldedFlowScalar(t *testing.T) {
	// The sequence form round-trips, so folding is not absent from goccy.
	if _, err := yamlreader.ReadString("cv: [a\n  b]\n"); err != nil {
		t.Errorf("a folded scalar in a flow sequence = %v, want it accepted", err)
	}

	// The mapping form does not, and upstream loads it as `{'a': '1 d'}`.
	_, err := yamlreader.ReadString("cv: {a: 1\n  d}\n")
	if err == nil {
		t.Skip("goccy now folds a plain scalar in a flow mapping; the twelve" +
			" shapes this explains should be re-measured against ruamel")
	}
	if !strings.Contains(err.Error(), "must be specified") {
		t.Errorf("err = %v, want goccy's flow-mapping refusal", err)
	}
}

// TestSpanEndsWhereTheScanStopped pins ruamel's *problem* mark — the end of the
// span — for the two classes where the port computed it from the wrong thing.
//
// **A quoted scalar is not stopped by a block line.** The scan hunts for the
// closing quote to the end of the stream, and a `b: c` on the way is just more
// scalar, so `cv: [\n  b: 'c\n  e: f` ends at line 4 and not at line 3. The
// port reused the flow rule, which does stop at a block line.
//
// **goccy's token is not a floor.** The port never ended a span before the
// token goccy blamed, but ruamel can stop earlier: in `cv: [a\n  b: [c,\n
// e: {f` the block key on line 2 ends ruamel's scan while goccy reads on to
// line 3, and the answer is line 2.
//
// Measured over the same 1050 shapes: end lines wrong 231 -> 135, no shape
// whose end was already right changes, and the start and the message are
// untouched by both rules.
func TestSpanEndsWhereTheScanStopped(t *testing.T) {
	tests := []struct {
		name               string
		src                string
		startLine, endLine int
	}{
		// A quoted scalar runs to the end of the stream.
		{
			name: "a block line after a single quote", src: "cv: [\n  b: 'c\n  e: f\n",
			startLine: 2, endLine: 4,
		},
		{
			name: "a block line after a double quote", src: "cv: [\n  b: \"c\n  e: f\n",
			startLine: 2, endLine: 4,
		},
		{
			name: "a flow opener after a quote", src: "cv: [\n  b: 'c\n  e: {f\n",
			startLine: 2, endLine: 4,
		},
		{
			// The control: with nothing after it, the stream ends where it
			// always did.
			name: "an unterminated quote at the end", src: "cv: [\n  b: 'c\n",
			startLine: 2, endLine: 3,
		},
		// The scan stops at the block line even though goccy read past it.
		{
			name: "goccy reads past the breaking line", src: "cv: [a\n  b: [c,\n  e: {f\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "a sequence opener on the third line", src: "cv: [a\n  b: c,\n  e: [g\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "a bare flow map on the third line", src: "cv: [a\n  b: c,\n  {h\n",
			startLine: 1, endLine: 2,
		},
		// Controls for the shapes the earlier units fixed, so this one cannot
		// move them.
		{
			name: "a block line breaks the flow", src: "cv: [a\n  b: [c,\n",
			startLine: 1, endLine: 2,
		},
		{
			name: "a swallowed inner mapping", src: "cv: [\n  b: {c\n",
			startLine: 2, endLine: 3,
		},
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

// TestBlockContextNamesTheInnermostOpenConstruct pins goccy's shorter spelling,
// `value is not allowed in this context`, onto the four things ruamel says
// about a badly indented line in block context.
//
// goccy reports one failure here; ruamel reports four, and which one it is
// depends on the source rather than on anything in goccy's text:
//
//   - `while parsing a block mapping` and `while parsing a block collection`
//     are its *parser* failing at a token that cannot follow the construct it
//     is inside. Which construct that is, is the innermost block level still
//     open at the offending line's own indentation.
//   - `mapping values are not allowed here` is its *scanner*, when a plain
//     scalar runs across lines and the next one carries a colon.
//   - `while scanning a simple key` is the same scanner refusing a key that
//     does not fit on one line.
//
// Every expectation below was measured against the vendored Python, message
// and both marks. `blockScan` is the reconstruction; the rows here are the
// shapes where competing readings of it disagree.
func TestBlockContextNamesTheInnermostOpenConstruct(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		message  string
		from, to yamldoc.Position
	}{{
		// The two indentations that bracket a sequence item: one step left of
		// the item's keys is the *sequence*, two more is the mapping above it.
		name:    "aligned with the dash",
		src:     "cv:\n  - name: John\n  bad: 1\n",
		message: "while parsing a block collection",
		from:    yamldoc.Position{Line: 2, Column: 3},
		to:      yamldoc.Position{Line: 3, Column: 3},
	}, {
		name:    "left of the dash",
		src:     "cv:\n  - name: John\n bad: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 3, Column: 2},
	}, {
		name:    "between the dash and the item keys",
		src:     "cv:\n  - name: John\n    extra: 1\n   bad: 2\n",
		message: "while parsing a block collection",
		from:    yamldoc.Position{Line: 2, Column: 3},
		to:      yamldoc.Position{Line: 4, Column: 4},
	}, {
		name:    "under-indented key in a plain mapping",
		src:     "cv:\n    name: John\n  bad: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 3, Column: 3},
	}, {
		name:    "a top-level sequence",
		src:     "- name: John\n bad: 1\n",
		message: "while parsing a block collection",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 2, Column: 2},
	}, {
		// A sequence whose dash sits at its parent mapping's own column adds no
		// indentation level, so ruamel names the mapping and not the sequence.
		name:    "a sequence at its parent's column",
		src:     "name:\n- bad: 1\n\n tail: 2\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 2},
	}, {
		name:    "three levels, the innermost mapping",
		src:     "cv:\n  a:\n    - b: 1\n      c: 2\n     bad: 3\n",
		message: "while parsing a block collection",
		from:    yamldoc.Position{Line: 3, Column: 5},
		to:      yamldoc.Position{Line: 5, Column: 6},
	}, {
		name:    "three levels, the sequence",
		src:     "cv:\n  a:\n    - b: 1\n      c: 2\n    bad: 3\n",
		message: "while parsing a block collection",
		from:    yamldoc.Position{Line: 3, Column: 5},
		to:      yamldoc.Position{Line: 5, Column: 5},
	}, {
		// A bare scalar at exactly the open construct's indentation is a key
		// ruamel *requires* to fit on one line.
		name:    "a required key that never ends",
		src:     "cv:\n  - name: John\n  bad\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 3},
		to:      yamldoc.Position{Line: 4, Column: 1},
	}, {
		name:    "a required key run into the next line",
		src:     "cv:\n  - name: John\n  bad\n    tail: 2\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 3},
		to:      yamldoc.Position{Line: 4, Column: 9},
	}, {
		// One step further left the key is not required, so the scalar folds
		// into the next line instead and the colon there is the failure.
		name:    "a plain scalar folded into a colon",
		src:     "cv:\n    name:\n   bad\n    deep:\n      x: 1\n",
		message: "mapping values are not allowed here",
		from:    yamldoc.Position{Line: 4, Column: 9},
		to:      yamldoc.Position{Line: 4, Column: 9},
	}, {
		name:    "the same, two levels out",
		src:     "cv:\n  sections:\n    name: John\n bad\n    tail: 2\n",
		message: "mapping values are not allowed here",
		from:    yamldoc.Position{Line: 5, Column: 9},
		to:      yamldoc.Position{Line: 5, Column: 9},
	}, {
		// The successor is *not* deeper than the level the scalar sits in, so
		// it does not fold and the parser reports the scalar itself.
		name:    "a successor that does not continue it",
		src:     "cv:\n  - name: \"John\"\n     bad\n    deep:\n      x: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 2, Column: 5},
		to:      yamldoc.Position{Line: 3, Column: 6},
	}, {
		name:    "a comment ends the scalar",
		src:     "cv:\n   name: John\n   bad: 1\n# c\n     tail: 2\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 2, Column: 4},
		to:      yamldoc.Position{Line: 5, Column: 6},
	}, {
		name:    "a trailing comment ends it too",
		src:     "cv:\n  name: John\n  bad: 1 # c\n    tail: 2\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 2, Column: 3},
		to:      yamldoc.Position{Line: 4, Column: 5},
	}, {
		name:    "a block scalar's content is not a continuation",
		src:     "a: |\n  x\n bad: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 3, Column: 2},
	}, {
		name:    "a document marker restarts the levels",
		src:     "---\ncv:\n  name: John\n bad: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 2, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 2},
	}, {
		// The shape an independent panel-vs-panel audit found and demoted
		// iteration 12 for: the offending line sits between the sequence's dash
		// and the key that opened it, so neither the sequence nor `cv` is the
		// answer — the mapping `a` belongs to is. The port reported line 6
		// alone, with goccy's `[6:6]` coordinate still in the text.
		name:    "the iteration 12 audit shape",
		src:     "cv:\n  name: A\n  sections:\n    a:\n      - hi\n     b: 2\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 4, Column: 5},
		to:      yamldoc.Position{Line: 6, Column: 6},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			record := userErr.Errors[0]
			if want := "This is not a valid YAML file. " + test.message + "."; record.Message != want {
				t.Errorf("message =\n  %q\nwant\n  %q", record.Message, want)
			}
			if record.YamlLocation == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if got := *record.YamlLocation; got.Start != test.from || got.End != test.to {
				t.Errorf("location = %v to %v, want %v to %v",
					got.Start, got.End, test.from, test.to)
			}
		})
	}
}

// TestNonMapValueNamesTheBlockConstruct pins goccy's *other* spelling for a
// badly indented line, `non-map value is specified` (`parser.go:499`), which it
// uses when the offending line is a value and its parser was building a
// mapping. It is the same failure as the shorter "not allowed in this context"
// one and ruamel splits it the same way — between its scanner's `while scanning
// a simple key` and its parser's `while parsing a block mapping` — so it is
// decided by `blockScan` rather than by a phrasing of its own.
//
// Enumerated over 18,730 block shapes, of which 5,429 reach this spelling:
// 3,404 are the scanner's and 1,977 the parser's. **Before this row every one
// of them leaked goccy's own text with its `[n:m]` coordinate**; after it,
// 4,810 carry ruamel's phrasing and 3,980 carry both its marks as well.
//
// **The measured remainder, so it is not mistaken for coverage**: 595 shapes
// whose offending line is a block scalar header (`cv:\n  name: John\n|`) are
// answered `while scanning a simple key` where ruamel says `while parsing a
// block mapping` — a pre-existing `blockScan` shape, since the same 108+282
// misattributions occur through the neighbouring row — and 830 whose end mark
// is wrong, driven by a trailing comment (427) and a flow collection on the
// offending line (331). Each row below was measured against the vendored
// Python, message and both marks.
func TestNonMapValueNamesTheBlockConstruct(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		message  string
		from, to yamldoc.Position
	}{{
		// A bare scalar at column 1 under a mapping is a key ruamel requires to
		// fit on one line, so its *scanner* reports it and there is no context
		// mark from the parser — the span opens on the scalar itself.
		name:    "a scalar left of the mapping",
		src:     "cv:\n  name: John\nbad\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 1},
	}, {
		name:    "a quoted scalar left of the mapping",
		src:     "cv:\n  name: John\n\"bad\"\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 1},
	}, {
		name:    "a successor deeper than the scalar",
		src:     "cv:\n  name: John\nbad\n  tail: 2\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 7},
	}, {
		name:    "two levels above the scalar",
		src:     "cv:\n  a:\n    b: 1\nbad\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 4, Column: 1},
		to:      yamldoc.Position{Line: 5, Column: 1},
	}, {
		name:    "a scalar left of a sequence's keys",
		src:     "cv:\n  - name: John\nbad\n",
		message: "while scanning a simple key",
		from:    yamldoc.Position{Line: 3, Column: 1},
		to:      yamldoc.Position{Line: 4, Column: 1},
	}, {
		// A sequence indicator cannot be a key at all, so the *parser* rejects
		// it and names the mapping it was inside, from where that mapping began.
		name:    "a sequence item where a key belongs",
		src:     "cv:\n  name: John\n- bad\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 3, Column: 1},
	}, {
		name:    "a sequence item carrying a mapping",
		src:     "cv:\n  name: John\n- bad: 1\n",
		message: "while parsing a block mapping",
		from:    yamldoc.Position{Line: 1, Column: 1},
		to:      yamldoc.Position{Line: 3, Column: 1},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.src, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
			}
			record := userErr.Errors[0]
			if want := "This is not a valid YAML file. " + test.message + "."; record.Message != want {
				t.Errorf("message =\n  %q\nwant\n  %q", record.Message, want)
			}
			if record.YamlLocation == nil {
				t.Fatal("yaml location = nil, want a span")
			}
			if got := *record.YamlLocation; got.Start != test.from || got.End != test.to {
				t.Errorf("location = %v to %v, want %v to %v",
					got.Start, got.End, test.from, test.to)
			}
		})
	}
}

// TestNonMapValueReachesGoccysOwnSpelling is the anti-vacuity half of the row
// above: every input there must genuinely arrive through goccy's `non-map value
// is specified`, not through the neighbouring "not allowed in this context"
// spelling that `blockScan` already answered. Without it the row could be
// deleted with the table's own reachability test still green, because both
// spellings end at the same reconstruction.
func TestNonMapValueReachesGoccysOwnSpelling(t *testing.T) {
	for _, src := range []string{
		"cv:\n  name: John\nbad\n",
		"cv:\n  name: John\n- bad\n",
	} {
		_, err := yamlreader.ReadString(src)
		if err == nil {
			t.Fatalf("%q parses; the row's inputs must fail", src)
		}
		if !strings.Contains(err.Error(), "non-map value is specified") {
			t.Errorf("%q = %v, want goccy's non-map spelling", src, err)
		}
	}
}

// TestBlockContextWithoutAnOpenConstructFallsThrough is the boundary of the row
// above: when the offending line is indented less than everything the document
// opened, ruamel is no longer inside a block construct at all and says
// `expected '<document start>', but found ...` — a phrasing whose found-token
// spelling is not reconstructible from goccy's text. The row declines rather
// than borrowing the nearest block phrase, so goccy's own line reaches the
// user, visibly wrong instead of silently misattributed.
func TestBlockContextWithoutAnOpenConstructFallsThrough(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("  cv:\n bad: 1\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected *schemaerr.UserValidationError, got %T: %v", err, err)
	}
	message := userErr.Errors[0].Message
	if !strings.Contains(message, "value is not allowed in this context") {
		t.Errorf("message = %q, want goccy's own text to reach the user", message)
	}
	for _, phrase := range []string{
		"while parsing a block mapping",
		"while parsing a block collection",
		"mapping values are not allowed here",
		"while scanning a simple key",
	} {
		if strings.Contains(message, phrase) {
			t.Errorf("a declined shape borrowed %q: %q", phrase, message)
		}
	}
}
