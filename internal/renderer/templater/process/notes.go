package process

import (
	"strings"
	"time"
)

// The footer's two page placeholders are **Typst source, not values**
// (footer_and_top_note.py:113-114).
//
// They are substituted before the string processors run, so
// EscapeTypstCharacters sees them — and they survive only because its first
// phase recognizes `#`-commands and holds them out. That is a load-bearing
// interaction between two modules, and `TestFooterPageCountersSurviveEscaping`
// is what proves it rather than either module's own test.
const (
	pageNumberSource = "#str(here().page())"
	totalPagesSource = "#str(counter(page).final().first())"
)

// StringProcessor is one step of the chain `process_model` builds
// (model_processor.py:80-85). It is a function type rather than an interface
// because upstream's is a list of callables and the chain is applied by reduce.
type StringProcessor func(string) string

// apply is `apply_string_processors` (string_processor.py:19-37): the chain in
// order, and a no-op on an empty chain.
func apply(text string, processors []StringProcessor) string {
	for _, processor := range processors {
		text = processor(text)
	}
	return text
}

// RenderTopNote is `render_top_note_template` (footer_and_top_note.py:10-63).
//
// **Substitute first, process second.** A placeholder's *value* goes through the
// Markdown-to-Typst conversion, not only the template text around it — running
// the processors first would escape the placeholder names instead.
func RenderTopNote(
	template string,
	catalog Catalog,
	current time.Time,
	name string,
	singleDateTemplate string,
	processors []StringProcessor,
) string {
	placeholders := BuildDatePlaceholders(current, catalog)
	placeholders["CURRENT_DATE"] = FormatDate(current, catalog, singleDateTemplate)
	// `LAST_UPDATED` exists **only here**. The footer's map does not carry it,
	// so the same placeholder in a footer template stays literal.
	placeholders["LAST_UPDATED"] = catalog.LastUpdated
	placeholders["NAME"] = name

	return apply(SubstitutePlaceholders(template, placeholders), processors)
}

// RenderFooter is `render_footer_template` (footer_and_top_note.py:66-123).
//
// Three differences from the top note, all measured:
//
//   - it carries `PAGE_NUMBER` and `TOTAL_PAGES` and **not** `LAST_UPDATED`;
//   - those two substitute to Typst source (see the constants above);
//   - the result is wrapped in `context { [ … ] }`, with the spacing upstream's
//     f-string produces — a space after `{` and before the closing `}`.
func RenderFooter(
	template string,
	catalog Catalog,
	current time.Time,
	name string,
	singleDateTemplate string,
	processors []StringProcessor,
) string {
	placeholders := BuildDatePlaceholders(current, catalog)
	placeholders["CURRENT_DATE"] = FormatDate(current, catalog, singleDateTemplate)
	placeholders["NAME"] = name
	placeholders["PAGE_NUMBER"] = pageNumberSource
	placeholders["TOTAL_PAGES"] = totalPagesSource

	rendered := apply(SubstitutePlaceholders(template, placeholders), processors)

	var out strings.Builder
	out.WriteString("context { [")
	out.WriteString(rendered)
	out.WriteString("] }")
	return out.String()
}
