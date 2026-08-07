package process

import (
	"sort"
	"strings"
	"time"
)

// EntryTemplateInput is everything `render_entry_templates` reads besides the
// entry itself (entry_templates_from_input.py:95-218).
type EntryTemplateInput struct {
	// Templates is the theme's block for this entry type — `main_column`,
	// `date_and_location_column`, and `degree_column` for education.
	Templates map[string]string
	// Phrases is `locale.phrases`, keyed by its snake-case field name.
	Phrases map[string]string
	// DateTemplates are the three `design.templates` date strings.
	DateTemplates DateTemplates
	Catalog       Catalog
	CurrentDate   time.Time
	ShowTimeSpan  bool

	// YearOnly names the date fields the document gave as a bare year, which
	// upstream distinguishes by `isinstance(x, int)` and the port cannot
	// re-derive from the string (see FormatDateRange).
	YearOnly map[string]bool
	// DOIURL is `PublicationEntry.doi_url`, computed by the model.
	DOIURL string
}

// RenderEntryTemplates is `render_entry_templates` (`:95-218`), the orchestrator
// that turns a theme's template strings into the fields an entry template
// interpolates.
//
// The order below is upstream's and each step depends on the one before it.
// **Two entry shapes return unchanged**: a bare string (`TextEntry`) and any
// type the theme has no template block for.
func RenderEntryTemplates(entry Entry, input EntryTemplateInput) (Entry, error) {
	if entry.IsText || len(input.Templates) == 0 {
		return entry, nil
	}

	fields := EntryFields(stringFields(entry))

	// Phrases expand **into the templates** first, leaving their
	// sub-placeholders in place for the removal passes to handle.
	templates := ExpandPhrases(input.Templates, input.Phrases)

	if _, present := fields["HIGHLIGHTS"]; present {
		if list, ok := entry.Fields["highlights"].([]string); ok {
			fields["HIGHLIGHTS"] = Highlights(list)
		}
	}
	if _, present := fields["AUTHORS"]; present {
		if list, ok := entry.Fields["authors"].([]string); ok {
			fields["AUTHORS"] = Authors(list)
		}
	}

	// **`DATE` is computed when any of the three is present**, not only when
	// `date` is — so an entry with only a range still fills it.
	_, hasDate := fields["DATE"]
	_, hasStart := fields["START_DATE"]
	_, hasEnd := fields["END_DATE"]
	if hasDate || hasStart || hasEnd {
		date, err := EntryDate(
			fields["DATE"], fields["START_DATE"], fields["END_DATE"],
			input.YearOnly["date"], input.YearOnly["start_date"], input.YearOnly["end_date"],
			input.Catalog, input.CurrentDate, input.ShowTimeSpan, input.DateTemplates)
		if err != nil {
			return entry, err
		}
		fields["DATE"] = date
	}

	if hasStart {
		fields["START_DATE"] = FormatSingleDate(fields["START_DATE"],
			input.YearOnly["start_date"], input.Catalog, input.DateTemplates.SingleDate)
	}
	if hasEnd {
		fields["END_DATE"] = FormatSingleDate(fields["END_DATE"],
			input.YearOnly["end_date"], input.Catalog, input.DateTemplates.SingleDate)
	}

	doi := fields["DOI"]
	if _, present := fields["URL"]; present {
		fields["URL"] = EntryURL(fields["URL"], doi, input.DOIURL)
	}
	// **The `DOI` branch sets both keys** (`:199-201`), so a publication with a
	// DOI shows the DOI link for its `URL` placeholder as well.
	if doi != "" {
		fields["URL"] = EntryURL(fields["URL"], doi, input.DOIURL)
		fields["DOI"] = EntryDOI(doi, input.DOIURL)
	}

	if summary, present := fields["SUMMARY"]; present && SummaryIsStandalone(templates) {
		fields["SUMMARY"] = SummaryBlock(summary)
	}

	templates = RemoveNotProvidedPlaceholders(templates, fields)

	// **Both the rendered templates and the raw fields become attributes**
	// (`:212-218`): `entry_templates | entry_fields` is one dict, and a field
	// whose name collides with a template's wins.
	out := Entry{Fields: map[string]any{}}
	for name, value := range entry.Fields {
		out.Fields[name] = value
	}
	for _, name := range sortedKeys(templates) {
		out.Fields[name] = SubstitutePlaceholders(templates[name], fields)
	}
	for name, value := range fields {
		out.Fields[strings.ToLower(name)] = SubstitutePlaceholders(value, fields)
	}
	return out, nil
}

// stringFields is `entry.model_dump(exclude_none=True)` reduced to the scalar
// values a placeholder can take. A list becomes its presence marker only — the
// special-cased `HIGHLIGHTS` and `AUTHORS` replace it with their joined form,
// and no other list field reaches a template.
func stringFields(entry Entry) map[string]string {
	out := make(map[string]string, len(entry.Fields))
	for name, value := range entry.Fields {
		switch typed := value.(type) {
		case string:
			out[name] = typed
		case []string:
			if len(typed) > 0 {
				out[name] = typed[0]
			}
		}
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
