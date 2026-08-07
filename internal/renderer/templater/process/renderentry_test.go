package process_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// The classic theme's education block, which is what these rows are rendered
// against.
var classicEducation = map[string]string{
	"main_column":              "**INSTITUTION**, AREA\nSUMMARY\nHIGHLIGHTS",
	"degree_column":            "**DEGREE**",
	"date_and_location_column": "LOCATION\nDATE",
}

func educationInput(showTimeSpan bool) process.EntryTemplateInput {
	return process.EntryTemplateInput{
		Templates:     classicEducation,
		Phrases:       map[string]string{"degree_with_area": "DEGREE in AREA"},
		DateTemplates: templates,
		Catalog:       english,
		CurrentDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		ShowTimeSpan:  showTimeSpan,
	}
}

// `render_entry_templates` end to end, measured by running upstream's on a
// validated `EducationEntry`.
//
// The second of the two functions a fresh verifier found marked done and not
// ported. This is the orchestrator — the one that makes the nine leaf
// processors reachable — so its absence meant `process.Run` skipped template
// expansion entirely.
func TestRenderEntryTemplates(t *testing.T) {
	entry := process.Entry{Fields: map[string]any{
		"institution": "MIT",
		"area":        "CS",
		"degree":      "BS",
		"start_date":  "2020-06",
		"end_date":    "2023-09",
		"location":    "Boston",
		"summary":     "Sum",
		"highlights":  []string{"a", "b"},
	}}

	got, err := process.RenderEntryTemplates(entry, educationInput(false))
	if err != nil {
		t.Fatalf("RenderEntryTemplates: %v", err)
	}

	want := map[string]string{
		// SUMMARY is wrapped because the theme has a line of exactly SUMMARY,
		// and HIGHLIGHTS is the joined bullet list.
		"main_column": "**MIT**, CS\n!!! summary\n    Sum\n- a\n- b",
		// DATE is computed from the range even though no `date` field exists.
		"date_and_location_column": "Boston\nJune 2020 – Sept 2023",
		"degree_column":            "**BS**",
	}
	for name, wanted := range want {
		if got.Fields[name] != wanted {
			t.Errorf("%s = %#v, want %q", name, got.Fields[name], wanted)
		}
	}

	// **The raw fields survive alongside the rendered templates**, because
	// upstream sets both onto the entry.
	if got.Fields["institution"] != "MIT" || got.Fields["area"] != "CS" {
		t.Errorf("the raw fields were lost: %v", got.Fields)
	}
}

// A minimal entry: the removal passes take the missing placeholders **and their
// surrounding punctuation** with them, and a line that became empty disappears
// rather than leaving a blank row.
func TestRenderEntryTemplatesWithMissingFields(t *testing.T) {
	entry := process.Entry{Fields: map[string]any{"institution": "MIT", "area": "CS"}}

	got, err := process.RenderEntryTemplates(entry, educationInput(false))
	if err != nil {
		t.Fatalf("RenderEntryTemplates: %v", err)
	}

	want := map[string]string{
		"main_column":              "**MIT**, CS",
		"date_and_location_column": "",
		"degree_column":            "",
	}
	for name, wanted := range want {
		if got.Fields[name] != wanted {
			t.Errorf("%s = %#v, want %q", name, got.Fields[name], wanted)
		}
	}
}

// The time span reaches the column through DATE's blank-line join.
func TestRenderEntryTemplatesWithATimeSpan(t *testing.T) {
	entry := process.Entry{Fields: map[string]any{
		"institution": "MIT", "area": "CS",
		"start_date": "2020-06", "end_date": "2023-09", "location": "Boston",
	}}

	got, err := process.RenderEntryTemplates(entry, educationInput(true))
	if err != nil {
		t.Fatalf("RenderEntryTemplates: %v", err)
	}

	const want = "Boston\nJune 2020 – Sept 2023\n\n3 years 4 months"
	if got.Fields["date_and_location_column"] != want {
		t.Errorf("= %#v, want %q", got.Fields["date_and_location_column"], want)
	}
}

// **Two shapes return unchanged**: a bare string entry, and any entry type the
// theme has no template block for.
func TestRenderEntryTemplatesPassesSomeEntriesThrough(t *testing.T) {
	text := process.Entry{Text: "hello", IsText: true}
	got, err := process.RenderEntryTemplates(text, educationInput(false))
	if err != nil {
		t.Fatalf("RenderEntryTemplates: %v", err)
	}
	if !got.IsText || got.Text != "hello" {
		t.Errorf("= %+v, want it unchanged", got)
	}

	entry := process.Entry{Fields: map[string]any{"a": "b"}}
	got, err = process.RenderEntryTemplates(entry, process.EntryTemplateInput{})
	if err != nil {
		t.Fatalf("RenderEntryTemplates: %v", err)
	}
	if got.Fields["a"] != "b" || len(got.Fields) != 1 {
		t.Errorf("= %+v, want it unchanged", got.Fields)
	}
}
