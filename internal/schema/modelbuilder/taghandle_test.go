package modelbuilder

import (
	"errors"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// An undefined tag handle is a ruamel `ParserError`, so it travels the same
// single-record route every other syntax failure does: no schema location, the
// literal input echo, and the first line of ruamel's own text interpolated into
// `This is not a valid YAML file.`
//
// Measured through the vendored CLI on the sample CV — `locale.language:
// [!e!x v]` with no `%TAG` line reports `main_yaml_file: line 7` and
// `while parsing a node.`, where the port reported `cv.sections.x` and an
// entry-type mismatch.
func TestAnUndefinedTagHandleIsAValidationRecord(t *testing.T) {
	tests := []struct {
		name  string
		input string
		start yamldoc.Position
		end   yamldoc.Position
	}{
		{
			name:  "flow sequence value",
			input: "cv:\n  name: John Doe\nlocale:\n  language: [!e!x v]\n",
			start: yamldoc.Position{Line: 4, Column: 14},
			end:   yamldoc.Position{Line: 4, Column: 14},
		},
		{
			name:  "anchored before the tag",
			input: "cv:\n  name: John Doe\nlocale:\n  language: [&a !e!x v]\n",
			start: yamldoc.Position{Line: 4, Column: 14},
			end:   yamldoc.Position{Line: 4, Column: 17},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadYamlWithValidationErrors(test.input, schemaerr.SourceMain)

			var userErr *schemaerr.UserValidationError
			if !errors.As(err, &userErr) {
				t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
			}
			if len(userErr.Errors) != 1 {
				t.Fatalf("records = %d, want 1", len(userErr.Errors))
			}

			record := userErr.Errors[0]
			if want := "This is not a valid YAML file. while parsing a node."; record.Message != want {
				t.Errorf("message = %q, want %q", record.Message, want)
			}
			if record.SchemaLocation != nil {
				t.Errorf("schema location = %v, want nil", record.SchemaLocation)
			}
			if record.Input != schemaerr.InputEllipsis {
				t.Errorf("input = %q, want %q", record.Input, schemaerr.InputEllipsis)
			}
			if record.YamlSource != schemaerr.SourceMain {
				t.Errorf("source = %q, want %q", record.YamlSource, schemaerr.SourceMain)
			}
			if record.YamlLocation == nil {
				t.Fatalf("yaml location = nil, want coordinates")
			}
			if got := *record.YamlLocation; got.Start != test.start || got.End != test.end {
				t.Errorf("yaml location = %+v, want %+v to %+v", got, test.start, test.end)
			}
		})
	}
}
