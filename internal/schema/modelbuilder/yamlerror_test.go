package modelbuilder

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/token"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
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
			name: "a tab where a key belongs",
			src:  "\ta: 1\n",
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

// An unmapped failure falls through to goccy's own first line. That is option A
// for the remainder — wrong, but visibly wrong rather than silently
// misattributed to the nearest mapped construct.
//
// The assertion is that it does **not** borrow a ruamel phrase it was not
// measured for.
func TestUnmappedParserMessageFallsThrough(t *testing.T) {
	_, err := ReadYamlWithValidationErrors("a: !!unknowntag@@ b\n", schemaerr.SourceMain)

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Skip("this input parses; the fallthrough needs an unmapped failure")
	}

	message := userErr.Errors[0].Message
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
