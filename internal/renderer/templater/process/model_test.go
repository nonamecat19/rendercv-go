package process_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

func baseModel() process.Model {
	catalog := english
	catalog.LastUpdated = "Last updated in"
	return process.Model{
		Name:            "John Doe",
		Headline:        "Engineer",
		Catalog:         catalog,
		Templates:       templates,
		TopNoteTemplate: "*LAST_UPDATED CURRENT_DATE*",
		FooterTemplate:  "*NAME*",
		PDFTitle:        "NAME - CV",
		CurrentDate:     time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
	}
}

// **The copy is deep, and this is the test that says why it has to be.**
//
// Upstream renders Typst and then Markdown from one model. Processing in place
// would leave the first render's Typst markup in the model, so the second render
// would escape it again — a correct `.typ` and a doubly-escaped `.md`, with the
// difference surfacing far from its cause.
func TestRunDoesNotMutateItsArgument(t *testing.T) {
	model := baseModel()
	model.Name = "a_b"
	model.Sections = []process.Section{{
		Title:   "Experience",
		Entries: []process.Entry{{Fields: map[string]any{"company": "a_b"}}},
	}}

	first := process.Run(model, process.FormatTypst)
	if model.Name != "a_b" {
		t.Errorf("the argument's name became %q", model.Name)
	}
	if model.Sections[0].Entries[0].Fields["company"] != "a_b" {
		t.Errorf("the argument's entry became %v", model.Sections[0].Entries[0].Fields["company"])
	}

	// And the second render is not compounded on the first.
	second := process.Run(model, process.FormatTypst)
	if first.Name != second.Name {
		t.Errorf("two renders of one model differ: %q and %q", first.Name, second.Name)
	}
	if first.Name != `a\_b` {
		t.Errorf("name = %q, want %q", first.Name, `a\_b`)
	}
}

// The chain is format-dependent: Markdown output is **not** markdown-parsed,
// because it is already Markdown.
func TestTheProcessorChainDependsOnTheFormat(t *testing.T) {
	model := baseModel()
	model.Name = "a_b"

	if got := process.Run(model, process.FormatTypst).Name; got != `a\_b` {
		t.Errorf("typst name = %q, want it escaped", got)
	}
	if got := process.Run(model, process.FormatMarkdown).Name; got != "a_b" {
		t.Errorf("markdown name = %q, want it untouched", got)
	}
}

// Bolding runs **before** the Typst conversion, so the `**` it inserts is
// Markdown and gets converted rather than emitted raw.
func TestBoldingRunsBeforeTheTypstConversion(t *testing.T) {
	model := baseModel()
	model.Name = "Python developer"
	model.BoldKeywords = []string{"Python"}

	got := process.Run(model, process.FormatTypst).Name
	if got != "#strong[Python] developer" {
		t.Errorf("= %q, want the markers converted", got)
	}
	if strings.Contains(got, "sym.ast.basic") {
		t.Error("the inserted markers were escaped instead of converted")
	}
}

// **`PDFTitle` carries the plain name and the header carries the processed
// one.** Using one for both puts Typst markup in a PDF metadata field.
func TestPDFTitleUsesThePlainName(t *testing.T) {
	model := baseModel()
	model.Name = "a_b"

	got := process.Run(model, process.FormatTypst)
	if got.PlainName != "a_b" {
		t.Errorf("plain name = %q, want it unprocessed", got.PlainName)
	}
	if got.Name != `a\_b` {
		t.Errorf("name = %q, want it processed", got.Name)
	}
	if got.PDFTitle != "a_b - CV" {
		t.Errorf("pdf title = %q, want the plain name", got.PDFTitle)
	}
}

// The top note and footer are computed onto the model, and the footer keeps its
// wrapper.
func TestNotesAreComputedOntoTheModel(t *testing.T) {
	got := process.Run(baseModel(), process.FormatTypst)
	if got.TopNote != "#emph[Last updated in Mar 2025]" {
		t.Errorf("top note = %q", got.TopNote)
	}
	if !strings.HasPrefix(got.Footer, "context { [") {
		t.Errorf("footer = %q, want the context wrapper", got.Footer)
	}
}

// `process_fields`' three value shapes and its skip set.
func TestRunFields(t *testing.T) {
	processors := []process.StringProcessor{process.MarkdownToTypst}

	entry := process.Entry{Fields: map[string]any{
		"company":     "a_b",
		"highlights":  []string{"x_y", "z"},
		"url":         "https://a_b.com",
		"start_date":  "2020-01",
		"_private":    "a_b",
		"page_number": 42,
	}}

	got := process.RunFields(entry, processors, false)

	if got.Fields["company"] != `a\_b` {
		t.Errorf("company = %v, want it processed", got.Fields["company"])
	}
	list, _ := got.Fields["highlights"].([]string)
	if len(list) != 2 || list[0] != `x\_y` {
		t.Errorf("highlights = %v, want each element processed", got.Fields["highlights"])
	}
	// The four skipped names keep their value exactly, which is what makes the
	// link and the date work downstream.
	if got.Fields["url"] != "https://a_b.com" {
		t.Errorf("url = %v, want it untouched", got.Fields["url"])
	}
	if got.Fields["start_date"] != "2020-01" {
		t.Errorf("start_date = %v, want it untouched", got.Fields["start_date"])
	}
	if got.Fields["_private"] != "a_b" {
		t.Errorf("_private = %v, want it untouched", got.Fields["_private"])
	}
	// A non-string is `str()`-ed **first**, so it arrives as a string.
	if got.Fields["page_number"] != "42" {
		t.Errorf("page_number = %#v, want the string \"42\"", got.Fields["page_number"])
	}
}

// A bare-string entry is processed directly rather than field-wise.
func TestRunFieldsOnATextEntry(t *testing.T) {
	got := process.RunFields(
		process.Entry{Text: "a_b", IsText: true},
		[]process.StringProcessor{process.MarkdownToTypst}, false)

	if !got.IsText || got.Text != `a\_b` {
		t.Errorf("= %+v, want a processed text entry", got)
	}
}

// The section title is processed and `show_time_spans_in` is matched against the
// **snake-cased** title — spec 006 §3.2 behavior 15's coercion, finally
// observable.
func TestSectionTitlesAreProcessed(t *testing.T) {
	model := baseModel()
	model.ShowTimeSpansIn = []string{"work_experience"}
	model.Sections = []process.Section{{
		Title:          "Work Experience",
		SnakeCaseTitle: "work_experience",
		Entries:        []process.Entry{{Fields: map[string]any{"a": "b_c"}}},
	}}

	got := process.Run(model, process.FormatTypst)
	if got.Sections[0].Title != "Work Experience" {
		t.Errorf("title = %q", got.Sections[0].Title)
	}
	if got.Sections[0].Entries[0].Fields["a"] != `b\_c` {
		t.Errorf("entry = %v", got.Sections[0].Entries[0].Fields["a"])
	}
}

// **Entry templates expand before the field processors run**, which is the
// ordering spec 008 §4A behavior 23 pins and which `process.Run` could not
// honour until RenderEntryTemplates existed — before that it called RunFields
// directly and no template was ever expanded.
func TestRunExpandsEntryTemplatesFirst(t *testing.T) {
	model := baseModel()
	model.Phrases = map[string]string{"degree_with_area": "DEGREE in AREA"}
	model.EntryTemplates = map[string]map[string]string{
		"education_entry": {"main_column": "**INSTITUTION**, AREA"},
	}
	model.Sections = []process.Section{{
		Title:     "Education",
		EntryType: "EducationEntry",
		Entries: []process.Entry{{Fields: map[string]any{
			"institution": "MIT", "area": "CS",
		}}},
	}}

	got := process.Run(model, process.FormatTypst)
	column := got.Sections[0].Entries[0].Fields["main_column"]

	// The template expanded **and then** the Typst conversion ran over the
	// result, so the `**` became `#strong[…]` rather than being escaped.
	if column != "#strong[MIT], CS" {
		t.Errorf("main_column = %#v, want %q", column, "#strong[MIT], CS")
	}
}

// The entry type's snake-case name is what `design.templates` is keyed by, so a
// type with no block is passed through — `TextEntry`'s case.
func TestRunPassesThroughAnUntemplatedEntryType(t *testing.T) {
	model := baseModel()
	model.EntryTemplates = map[string]map[string]string{
		"education_entry": {"main_column": "X"},
	}
	model.Sections = []process.Section{{
		EntryType: "TextEntry",
		Entries:   []process.Entry{{Text: "a_b", IsText: true}},
	}}

	got := process.Run(model, process.FormatTypst)
	if got.Sections[0].Entries[0].Text != `a\_b` {
		t.Errorf("= %#v, want the text processed and not templated",
			got.Sections[0].Entries[0].Text)
	}
}

// A bare-integer date belongs to the entry that wrote it.
//
// `YearOnly` used to sit on the Model, one set for the whole CV, so the first
// entry that wrote `start_date: 2000` made **every** entry's start date format
// as a year — `2000-01` rendering as `2000` in an entry whose own YAML says
// nothing of the kind. The two entries below differ only in how they wrote the
// same two dates, and that is the whole assertion.
func TestRunKeepsIntegerDatesPerEntry(t *testing.T) {
	model := baseModel()
	model.EntryTemplates = map[string]map[string]string{
		"education_entry": {"main_column": "DATE"},
	}
	model.Sections = []process.Section{{
		Title:     "Education",
		EntryType: "EducationEntry",
		Entries: []process.Entry{
			{
				Fields:   map[string]any{"start_date": 2000, "end_date": 2005},
				YearOnly: map[string]bool{"start_date": true, "end_date": true},
			},
			{
				Fields: map[string]any{"start_date": "2000-01", "end_date": "2005-06"},
			},
		},
	}}

	got := process.Run(model, process.FormatTypst)
	years := got.Sections[0].Entries[0].Fields["main_column"]
	months := got.Sections[0].Entries[1].Fields["main_column"]

	if !strings.Contains(months.(string), "Jan") {
		t.Errorf("the second entry's date = %#v, want a month in it", months)
	}
	if strings.ContainsAny(years.(string), "JFMASOND") {
		t.Errorf("the first entry's date = %#v, want years only", years)
	}
}
