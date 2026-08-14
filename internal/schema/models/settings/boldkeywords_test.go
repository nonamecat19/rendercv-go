package settings_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func boldKeywordsNode(t *testing.T, value string) *yamldoc.Node {
	t.Helper()
	doc, err := yamlreader.ReadString("bold_keywords: " + value + "\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc.Items[0].Value
}

// `settings.bold_keywords` is `list[str]` (settings.py:27).
//
// Every row was measured against the vendored binary before it was written
// here: the whole panel and the exit code, `NO_COLOR=1 TERM=dumb COLUMNS=80`,
// `render CV.yaml -nopdf -nopng`. The rows that render are as load-bearing as
// the rows that fail — the port refusing `['A']` would be the same class of
// defect in the other direction.
func TestValidateBoldKeywords(t *testing.T) {
	tests := []struct {
		name string
		// value is the YAML written after the key.
		value string
		// wantLocations is one dotted location per expected record, in order.
		wantLocations []string
		// wantMessages is the final, post-pipeline text of each record.
		wantMessages []string
		// wantInputs is the Input Value column of each record.
		wantInputs []string
	}{
		// --- accepted: a sequence of strings, however empty ---
		{name: "a list of strings", value: "['A', 'B']"},
		{name: "an empty list", value: "[]"},
		{name: "a list holding an empty string", value: "['']"},

		// --- refused at the field: not a sequence at all ---
		//
		// A Python `str` is iterable, and this is the row that proves pydantic
		// does not spread it: `'A'` is `list_type`, not `['A']`.
		{
			name: "a bare string", value: "'A'",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"A"},
		},
		{
			name: "an integer", value: "1",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"1"},
		},
		{
			name: "a float", value: "1.5",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"1.5"},
		},
		{
			name: "a bool", value: "true",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"True"},
		},
		{
			name: "an unquoted date", value: "2024-01-02",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"2024-01-02"},
		},
		{
			name: "a null", value: "null",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{"None"},
		},
		{
			name: "a mapping", value: "{a: b}",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{schemaerr.InputEllipsis},
		},
		{
			name: "an empty mapping", value: "{}",
			wantLocations: []string{"settings.bold_keywords"},
			wantMessages:  []string{messageList},
			wantInputs:    []string{schemaerr.InputEllipsis},
		},

		// --- refused per element: a sequence holding a non-`str` ---
		{
			name: "an integer element", value: "[1]",
			wantLocations: []string{"settings.bold_keywords.0"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{"1"},
		},
		{
			name: "a bool element", value: "[true]",
			wantLocations: []string{"settings.bold_keywords.0"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{"True"},
		},
		{
			name: "a null element", value: "[null]",
			wantLocations: []string{"settings.bold_keywords.0"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{"None"},
		},
		{
			name: "a nested list element", value: "[[a]]",
			wantLocations: []string{"settings.bold_keywords.0"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{schemaerr.InputEllipsis},
		},
		{
			name: "a mapping element", value: "[{a: b}]",
			wantLocations: []string{"settings.bold_keywords.0"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{schemaerr.InputEllipsis},
		},
		{
			name: "a bad element after a good one", value: "['a', 1]",
			wantLocations: []string{"settings.bold_keywords.1"},
			wantMessages:  []string{messageString},
			wantInputs:    []string{"1"},
		},
		{
			name: "two bad elements among good ones", value: "['a', 1, 'b', null]",
			wantLocations: []string{
				"settings.bold_keywords.1", "settings.bold_keywords.3",
			},
			wantMessages: []string{messageString, messageString},
			wantInputs:   []string{"1", "None"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := settings.ValidateBoldKeywords(
				boldKeywordsNode(t, test.value), []string{"settings"}, schemaerr.SourceMain,
			)
			if len(errs) != len(test.wantLocations) {
				t.Fatalf("errs = %+v, want %d record(s)", errs, len(test.wantLocations))
			}

			final, err := errorpipeline.Parse(errs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(final) != len(test.wantLocations) {
				t.Fatalf("final = %+v, want %d record(s)", final, len(test.wantLocations))
			}

			for i := range test.wantLocations {
				if got := strings.Join(errs[i].SchemaLocation, "."); got != test.wantLocations[i] {
					t.Errorf("record %d location = %q, want %q", i, got, test.wantLocations[i])
				}
				if final[i].Message != test.wantMessages[i] {
					t.Errorf("record %d message =\n  %q\nwant\n  %q",
						i, final[i].Message, test.wantMessages[i])
				}
				if final[i].Input != test.wantInputs[i] {
					t.Errorf("record %d input = %q, want %q", i, final[i].Input, test.wantInputs[i])
				}
			}
		})
	}
}

// The two final texts, after `error_dictionary.yaml` and the pipeline's
// trailing period. Row 13 rewrites the list one; no row matches the string one,
// so it keeps pydantic's own sentence.
const (
	messageList   = "This field should contain a list of items but it doesn't."
	messageString = "Input should be a valid string."
)

// An absent key is not an error: the field defaults to `[]` (settings.py:28),
// so a document with no `bold_keywords` must reach the renderer.
func TestValidateBoldKeywordsIgnoresAnAbsentField(t *testing.T) {
	if errs := settings.ValidateBoldKeywords(
		nil, []string{"settings"}, schemaerr.SourceMain,
	); len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
}
