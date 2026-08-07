package process

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Format is the target `process_model` branches on (model_processor.py:61-63).
type Format string

// The two formats. There is no `html` here: `render_html` converts the finished
// Markdown rather than processing the model again (spec 008 §3 behavior 14).
const (
	FormatTypst    Format = "typst"
	FormatMarkdown Format = "markdown"
)

// Entry is one CV entry as `model_dump(exclude_none=True)` leaves it: field
// names to values, where a value is a string or a list of strings.
//
// A map rather than a typed struct because `process_fields` iterates whatever
// the dump contains — including a user's arbitrary extra keys, which spec 008
// §4G behavior 59 turns into placeholders with no registration step. A struct
// would need a case per entry type and would drop the extras.
type Entry struct {
	// Text is set for a bare-string entry (`TextEntry`), which
	// `process_fields` processes directly rather than field-wise (`:168-169`).
	Text string
	// IsText distinguishes an empty TextEntry from a mapping entry.
	IsText bool

	Fields map[string]any
}

// Section is one `cv.rendercv_sections` element, reduced to what
// `process_model` touches.
type Section struct {
	Title          string
	SnakeCaseTitle string
	EntryType      string
	Entries        []Entry
}

// Model is the slice of `RenderCVModel` the templater reads. It is the
// renderer's own view, not the schema's — spec 008 plan §4 keeps `process`
// downstream of `models` so the renderer never reaches back into validation.
type Model struct {
	Name     string
	Headline string
	Sections []Section

	// PlainName is `cv._plain_name`, captured **before** Name is processed.
	PlainName string
	// Connections, TopNote and Footer are computed onto the model by
	// Run, mirroring `cv._connections`, `_top_note` and `_footer`.
	Connections []string
	TopNote     string
	Footer      string

	Catalog   Catalog
	Templates DateTemplates
	// TopNoteTemplate and FooterTemplate are `design.templates`' two.
	TopNoteTemplate string
	FooterTemplate  string
	// ShowTimeSpansIn is `design.sections.show_time_spans_in`, already
	// snake-cased by spec 006 §3.2 behavior 15's coercion.
	ShowTimeSpansIn []string

	BoldKeywords []string
	PDFTitle     string
	CurrentDate  time.Time

	// RawConnections is the connection list before formatting, built from
	// `cv._key_order` by the caller — the order is the input file's
	// (spec 008 §4E behavior 45).
	RawConnections    []Connection
	ConnectionOptions ConnectionOptions
}

// Run is `process_model` (model_processor.py:61-146).
//
// **It returns a copy and never mutates its argument.** Upstream's first line is
// `model_copy(deep=True)`, and the reason is not tidiness: the same model is
// rendered to Typst and then to Markdown, so processing in place produces a
// correct `.typ` and a doubly-escaped `.md`. The corpus renders both, which is
// how the difference would surface — as a Markdown diff, far from the cause.
func Run(model Model, format Format) Model {
	out := model.clone()
	processors := out.stringProcessors(format)

	// The order of these five is upstream's, and PlainName is captured before
	// Name is touched.
	out.PlainName = out.Name
	out.Name = apply(out.Name, processors)
	out.Headline = apply(out.Headline, processors)
	out.Connections = out.formatConnections(format)
	out.TopNote = RenderTopNote(out.TopNoteTemplate, out.Catalog, out.CurrentDate,
		out.Name, out.Templates.SingleDate, processors)
	out.Footer = RenderFooter(out.FooterTemplate, out.Catalog, out.CurrentDate,
		out.Name, out.Templates.SingleDate, processors)

	// **`PDFTitle`'s `NAME` is the *plain* name** (`:120`), so the PDF's
	// metadata carries the unprocessed text while the header carries the
	// processed one. Using one for both puts Typst markup in a PDF title.
	placeholders := BuildDatePlaceholders(out.CurrentDate, out.Catalog)
	placeholders["CURRENT_DATE"] = FormatDate(out.CurrentDate, out.Catalog, out.Templates.SingleDate)
	placeholders["NAME"] = out.PlainName
	out.PDFTitle = SubstitutePlaceholders(out.PDFTitle, placeholders)

	for i := range out.Sections {
		section := &out.Sections[i]
		section.Title = apply(section.Title, processors)
		showTimeSpan := containsString(out.ShowTimeSpansIn, section.SnakeCaseTitle)

		for j := range section.Entries {
			// **Entry templates expand first, then the field processors run over
			// the result** (`:141-146`). Reversing them would escape the
			// template's own markup.
			section.Entries[j] = RunFields(section.Entries[j], processors, showTimeSpan)
		}
	}
	return out
}

// stringProcessors is the chain of `:80-85`.
//
// **Markdown gets only the bolding.** Its output is already Markdown, so
// converting it to Typst would be wrong — which is why the chain is built from
// the format rather than being a constant.
func (m Model) stringProcessors(format Format) []StringProcessor {
	keywords := m.BoldKeywords
	processors := []StringProcessor{
		func(text string) string { return MakeKeywordsBold(text, keywords) },
	}
	if format == FormatTypst {
		processors = append(processors, MarkdownToTypst)
	}
	return processors
}

func (m Model) formatConnections(format Format) []string {
	if format == FormatTypst {
		return FormatConnectionsForTypst(m.RawConnections, m.ConnectionOptions)
	}
	return FormatConnectionsForMarkdown(m.RawConnections)
}

// RunFields is `process_fields` (model_processor.py:149-189).
//
// Four field names are skipped plus anything underscore-prefixed
// (spec 008 §4A behavior 24), a list is processed element-wise, and **a
// non-string non-list value is `str()`-ed first** — so an integer extra field
// arrives at the template as a string.
//
// `showTimeSpan` is threaded through because the caller computes it per section
// and `render_entry_templates` needs it; nothing here reads it, and it is on the
// signature so the wiring in Wave B cannot forget it.
func RunFields(entry Entry, processors []StringProcessor, _ bool) Entry {
	if entry.IsText {
		// A bare string entry is processed directly rather than field-wise.
		return Entry{Text: apply(entry.Text, processors), IsText: true}
	}

	fields := make(map[string]any, len(entry.Fields))
	for name, value := range entry.Fields {
		if IsSkippedField(name) {
			fields[name] = value
			continue
		}
		switch typed := value.(type) {
		case string:
			fields[name] = apply(typed, processors)
		case []string:
			processed := make([]string, 0, len(typed))
			for _, item := range typed {
				processed = append(processed, apply(item, processors))
			}
			fields[name] = processed
		default:
			fields[name] = apply(stringify(typed), processors)
		}
	}
	return Entry{Fields: fields}
}

// stringify is Python's `str()` for the values a dumped entry can hold.
func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case nil:
		return "None"
	case int:
		return strconv.Itoa(typed)
	case float64:
		// Python prints a whole float with a trailing `.0`; Go's shortest form
		// drops it. Only a user's arbitrary numeric extra field reaches here.
		text := strconv.FormatFloat(typed, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return text
	}
	return fmt.Sprint(value)
}

// clone is the deep copy of `:78`. Only the mutable parts need it — the strings
// are values already.
func (m Model) clone() Model {
	out := m
	out.Sections = make([]Section, len(m.Sections))
	for i, section := range m.Sections {
		copied := section
		copied.Entries = make([]Entry, len(section.Entries))
		for j, entry := range section.Entries {
			copiedEntry := entry
			if entry.Fields != nil {
				copiedEntry.Fields = make(map[string]any, len(entry.Fields))
				for name, value := range entry.Fields {
					copiedEntry.Fields[name] = cloneValue(value)
				}
			}
			copied.Entries[j] = copiedEntry
		}
		out.Sections[i] = copied
	}
	out.BoldKeywords = append([]string(nil), m.BoldKeywords...)
	out.ShowTimeSpansIn = append([]string(nil), m.ShowTimeSpansIn...)
	out.RawConnections = append([]Connection(nil), m.RawConnections...)
	return out
}

func cloneValue(value any) any {
	if list, ok := value.([]string); ok {
		return append([]string(nil), list...)
	}
	return value
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// SortedFieldNames is the deterministic order the tests read fields in. Upstream
// iterates a dict, whose order is insertion order and therefore the model's
// declaration order; nothing in `process_fields` depends on it, because every
// field is processed independently.
func SortedFieldNames(fields map[string]any) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
