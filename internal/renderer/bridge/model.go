package bridge

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Document is everything the bridge reads: the validated model, plus the two
// resolutions the renderer is the first consumer of.
type Document struct {
	Model *models.RenderCVModel
	// Design is `design.Effective`'s output — the theme's declared defaults with
	// the theme's overrides and then the document's own block merged over them.
	Design map[string]any
	// Locale is the resolved catalog, and Settings the three resolved fields.
	Locale   locale.Catalog
	Settings settings.Resolved
}

// Resolve performs the three resolutions and returns the Document the rest of
// the bridge reads. `now` is what `settings.current_date: today` resolves to.
func Resolve(model *models.RenderCVModel, now time.Time) Document {
	resolved := settings.Resolve(model.Settings, now)
	return Document{
		Model:    model,
		Design:   design.Effective(themeOf(model), designBlock(model)),
		Locale:   locale.Resolve(model.Locale),
		Settings: resolved,
	}
}

// Model builds the `process.Model` the templater consumes — the bridge's whole
// point.
//
// **It does no processing.** `process.Run` is what escapes, bolds, formats and
// expands; everything here is a read. Doing any of it twice is the failure mode
// the split exists to prevent, and it shows up as doubly-escaped text rather
// than as an error.
func Model(document Document, registry *entries.Registry) (process.Model, error) {
	model := document.Model
	catalog := document.Locale

	connections, err := Connections(model.CvModel, ConnectionDesign{
		PhoneNumberFormat: design.EffectiveString(document.Design,
			"header", "connections", "phone_number_format"),
		DisplayURLsInsteadOfUsername: design.EffectiveBool(document.Design,
			"header", "connections", "display_urls_instead_of_usernames"),
	})
	if err != nil {
		return process.Model{}, err
	}

	return process.Model{
		Name:     text(model.CvModel.Name),
		Headline: text(model.CvModel.Headline),
		Sections: Sections(model.CvModel, registry),

		Catalog: process.Catalog{
			MonthNames:         catalog.MonthNames,
			MonthAbbreviations: catalog.MonthAbbreviations,
			Present:            catalog.Present,
			Year:               catalog.Year,
			Years:              catalog.Years,
			Month:              catalog.Month,
			Months:             catalog.Months,
			LastUpdated:        catalog.LastUpdated,
		},
		Phrases: map[string]string{"degree_with_area": catalog.DegreeWithArea},

		Templates: process.DateTemplates{
			SingleDate: design.EffectiveString(document.Design, "templates", "single_date"),
			DateRange:  design.EffectiveString(document.Design, "templates", "date_range"),
			TimeSpan:   design.EffectiveString(document.Design, "templates", "time_span"),
		},
		TopNoteTemplate: design.EffectiveString(document.Design, "templates", "top_note"),
		FooterTemplate:  design.EffectiveString(document.Design, "templates", "footer"),
		ShowTimeSpansIn: design.EffectiveStrings(document.Design,
			"sections", "show_time_spans_in"),
		EntryTemplates: entryTemplates(document.Design),

		BoldKeywords: document.Settings.BoldKeywords,
		PDFTitle:     document.Settings.PDFTitle,
		CurrentDate:  document.Settings.CurrentDate,

		RawConnections: connections,
		ConnectionOptions: process.ConnectionOptions{
			ShowIcons: design.EffectiveBool(document.Design,
				"header", "connections", "show_icons"),
			Hyperlink: design.EffectiveBool(document.Design,
				"header", "connections", "hyperlink"),
		},
	}, nil
}

// entryTemplates picks the per-entry-type blocks out of `design.templates`.
//
// The nested keys are exactly the snake-case entry-type names — `education_entry`
// — which is how `render_entry_templates` finds a type's block
// (`getattr(templates, entry.entry_type_in_snake_case)`). The scalar members of
// `templates` (`single_date`, `footer`, …) are skipped by shape rather than by
// name, so a template block added upstream arrives here without a change.
func entryTemplates(values map[string]any) map[string]map[string]string {
	block, ok := values["templates"].(map[string]any)
	if !ok {
		return nil
	}

	out := make(map[string]map[string]string, len(block))
	for name, value := range block {
		nested, isNested := value.(map[string]any)
		if !isNested {
			continue
		}
		columns := make(map[string]string, len(nested))
		for column, text := range nested {
			// **A null column is absent, not empty** — a theme that sets
			// `degree_column: null` has no degree column, and an empty string
			// would render one containing nothing.
			if text == nil {
				continue
			}
			if value, isText := text.(string); isText {
				columns[column] = value
			}
		}
		out[name] = columns
	}
	return out
}

// themeOf is the design block's discriminator, defaulting to `classic` — which
// is `Design`'s own default rather than a choice made here.
func themeOf(model *models.RenderCVModel) string {
	if model == nil {
		return "classic"
	}
	if theme := field(model.Design, "theme"); theme != "" {
		return theme
	}
	return "classic"
}

// designBlock projects the document's `design` mapping onto the plain values
// `design.Effective` merges. It is a read of already-validated nodes, so an
// unexpected shape is dropped rather than reported.
func designBlock(model *models.RenderCVModel) map[string]any {
	if model == nil {
		return nil
	}
	return mappingOf(model.Design)
}

// mappingOf projects a document mapping onto the plain values `deepMerge` walks:
// nested mappings recurse, sequences become `[]string`, and a null-valued key
// **survives as nil**.
//
// **Dropping the nulls was wrong**, and one field proves it: `degree_column` is
// the port's only `str | None` with a non-null default, so
// `design.templates.education_entry.degree_column: null` is the documented way
// to turn the degree column off. A dropped null cannot override anything, so
// upstream omitted the column and the port emitted it with the declared
// `**DEGREE**` — from a twelve-line document. The theme overrides already
// express the same thing as a nil (`overrides_generated.go:79`), so this makes
// the two layers agree rather than inventing a shape.
//
// `design.Effective` restores the default for a null on a field that is *not*
// nullable, which is the case upstream rejects at validation.
func mappingOf(node *yamldoc.Node) map[string]any {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil
	}

	out := make(map[string]any, len(node.Items))
	for _, item := range node.Items {
		if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
			out[item.Key] = nil
			continue
		}
		switch item.Value.Kind {
		case yamldoc.KindMapping:
			out[item.Key] = mappingOf(item.Value)
		case yamldoc.KindSequence:
			out[item.Key] = stringsOf(item.Value)
		case yamldoc.KindBool:
			// **A boolean has to stay a boolean.** Every option the base tree
			// declares as one is read back with `EffectiveBool`, and a document
			// that overrode it with the string `"false"` would read as the zero
			// value — `hyperlink: false` and `hyperlink: true` would both turn
			// links off.
			out[item.Key] = item.Value.Raw == "true" || item.Value.Raw == "True" ||
				item.Value.Raw == "yes" || item.Value.Raw == "on"
		default:
			out[item.Key] = item.Value.Raw
		}
	}
	return out
}

func stringsOf(node *yamldoc.Node) []string {
	out := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || elem.Kind == yamldoc.KindNull {
			continue
		}
		out = append(out, elem.Raw)
	}
	return out
}
