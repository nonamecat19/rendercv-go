package jsonschema

import (
	"strings"
	"testing"
)

// Every row of spec 005 plan §4's table, each measured against
// `json.dumps(..., indent=2, ensure_ascii=False)` on the vendored Python.
func TestMarshalMatchesPythonDumps(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			// The row the current schema cannot exercise: Go escapes these three
			// by default, Python never does, and `schema.json` contains none of
			// them today. An encoder that gets this wrong passes the gate.
			name:  "HTML characters are not escaped",
			value: NewObject().Set("a", "<&>"),
			want:  "{\n  \"a\": \"<&>\"\n}",
		},
		{
			name:  "non-ASCII is literal",
			value: NewObject().Set("c", NewObject().Set("d", "ü")),
			want:  "{\n  \"c\": {\n    \"d\": \"ü\"\n  }\n}",
		},
		{
			name:  "an array of mixed kinds",
			value: NewObject().Set("b", []any{"x", 1, true, nil}),
			want:  "{\n  \"b\": [\n    \"x\",\n    1,\n    true,\n    null\n  ]\n}",
		},
		{
			name:  "the five short escapes and the two structural ones",
			value: NewObject().Set("s", "a\"b\\c\nd\te"),
			want:  "{\n  \"s\": \"a\\\"b\\\\c\\nd\\te\"\n}",
		},
		{
			name:  "an empty array stays on one line",
			value: NewObject().Set("e", []any{}),
			want:  "{\n  \"e\": []\n}",
		},
		{
			name:  "so does an empty object",
			value: NewObject().Set("e", NewObject()),
			want:  "{\n  \"e\": {}\n}",
		},
		{
			// Spec 005 §5 behavior 18: a present nil is `null`, not an omission.
			name:  "a nil value is null",
			value: NewObject().Set("description", nil),
			want:  "{\n  \"description\": null\n}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Marshal(test.value)
			if err != nil {
				t.Fatalf("Marshal = %v", err)
			}
			if got != test.want {
				t.Errorf("Marshal =\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

// No trailing newline. `dumps` does not append one and `write_text` adds
// nothing, so the file's last three bytes are `"\n}`.
func TestMarshalHasNoTrailingNewline(t *testing.T) {
	got, err := Marshal(NewObject().Set("a", 1))
	if err != nil {
		t.Fatalf("Marshal = %v", err)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("Marshal ends with a newline: %q", got)
	}
	if !strings.HasSuffix(got, "}") {
		t.Errorf("Marshal = %q, want it to end at the brace", got)
	}
}

// A kind the schema cannot contain is an error rather than a guess.
func TestMarshalRejectsAnUnknownKind(t *testing.T) {
	if _, err := Marshal(NewObject().Set("a", 1.5)); err == nil {
		t.Error("Marshal accepted a float; the schema has no float values")
	}
}
