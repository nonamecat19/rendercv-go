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
			if got := parserMessage(tc.text); got != tc.want {
				t.Errorf("parserMessage(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// Spec §3.85 — a parser failure carrying no position yields an absent YAML
// location rather than a fabricated one.
func TestParserErrorWithoutMarks(t *testing.T) {
	record := yamlSyntaxValidationError(marklessParserError{}, schemaerr.SourceMain)

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
