package bridge

import (
	"errors"
	"os"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
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
	// ScriptError is why a custom theme's `init.lua` could not be used, and nil
	// when there is no script or the script is fine. It is a
	// `*schemaerr.UserValidationError` carrying one record per failure, so the
	// CLI prints it through the same `There are validation errors!` panel every
	// other validation failure gets (D-013).
	//
	// **It is a field rather than a second return from `Resolve`** so that this
	// lands without touching `Resolve`'s eight call sites while T1 is in flight
	// in the same file.
	ScriptError error
}

// Resolve performs the three resolutions and returns the Document the rest of
// the bridge reads. `now` is what `settings.current_date: today` resolves to.
func Resolve(model *models.RenderCVModel, now time.Time) Document {
	resolved := settings.Resolve(model.Settings, now)
	theme := themeOf(model)
	block := designBlock(model)

	// **A custom theme's script runs here**, which is upstream's
	// `validate_design` position — the theme's options have to exist before
	// anything reads the effective tree. A built-in theme has no script and
	// `themeScript` returns nil, so the nine of them take the path they always
	// did (spec 014 §1 behavior 4).
	script, hasScript, scriptErr := themeScript(model, theme)

	return Document{
		Model:       model,
		Design:      design.EffectiveWithScript(theme, script, block, hasScript),
		Locale:      locale.Resolve(model.Locale),
		Settings:    resolved,
		ScriptError: scriptErr,
	}
}

// themeScript loads `<theme>/init.lua` from beside the input file — D-002's
// replacement for upstream's `<theme>/__init__.py` (spec 014 §1 behavior 1).
//
// **A missing script is not an error**, which is upstream's behavior too: a
// theme folder with no module is valid and falls back to the base tree
// (`design.py:137-142`) — measured against the vendored binary at exit 0.
//
// **A script that fails is an error**, and used to be silent: every failure
// returned the same nil map a missing file does, so the document rendered with
// the theme's base defaults at exit 0 with no signal at all. Upstream refuses to
// render and exits 1 for each of them (spec 014 §2 behavior 9, tasks 014 T4), so
// the third return carries the reason and `Resolve` hands it to the CLI.
//
// **The returned bool is whether an `init.lua` file exists at all**, distinct
// from whether the returned `map[string]any` is usable. Both a missing file
// and a script that fails to parse, run or validate return a nil map, but only
// the former is upstream's `ThemeOptionsAreNotProvided` fallback —
// `design.EffectiveWithScript` needs to tell them apart to avoid discarding a
// document on a theme whose script merely broke.
func themeScript(model *models.RenderCVModel, theme string) (
	options map[string]any, hasScript bool, failure error,
) {
	if model == nil {
		return nil, false, nil
	}
	// **A built-in theme never reads a script**, which upstream gets by only
	// entering the custom-theme path when the built-in discriminator *fails*
	// (`design.py:36-50`). Reading it for every theme meant a `classic/init.lua`
	// beside a CV silently changed a built-in theme's artifact — a parity break
	// found by a verifier, measured as `page-size: "a5"` where upstream emits
	// `"us-letter"`.
	if design.IsBuiltinTheme(theme) {
		return nil, false, nil
	}
	path, ok := model.InputFilePath()
	if !ok {
		return nil, false, nil
	}

	// **`design.ThemeScriptPath`, not `filepath.Dir` + `filepath.Join`.** Both
	// of those call `Clean`, which collapses a `..` segment; upstream's
	// `PurePath.parent` is purely lexical and keeps it, and `design.Validate`
	// resolves the same folder that way. The obvious idiomatic Go spelling is
	// the wrong one here: through a symlink the two resolutions reach different
	// directories, so the document was validated against one script and rendered
	// with another — measured, `render ./bb/../bb/CV.yaml` with `bb` a symlink,
	// two different themes at exit 0. Do not "simplify" this back.
	source, err := os.ReadFile(design.ThemeScriptPath(path, theme))
	if err != nil {
		// **Not a failure, which is the point of this whole unit**: a theme
		// folder with no script is upstream's `ThemeOptionsAreNotProvided`
		// fallback and renders at exit 0 on both sides. `nilerr` reads this as a
		// swallowed error; it is the one place here where swallowing is the
		// specified behavior.
		return nil, false, nil //nolint:nilerr // an absent script is the documented fallback
	}
	// The file exists from here on, whatever it turns out to contain.
	hasScript = true

	table, err := luatheme.Run(string(source))
	if err != nil {
		return nil, hasScript, scriptFailure(theme, err)
	}

	options = luatheme.Options(table)

	// **A script whose shapes conflict with the tree is dropped whole.** It used
	// to reach the template and print a Go type name into the artifact —
	// `page-size: "<map[string]interface {} Value>"` at exit 0, and then, once
	// dropping it was added, the theme's own defaults at exit 0 — silently wrong
	// either way. Now it is reported, which is upstream's behavior: a declared
	// default the model rejects is exit 1 with no artifact (`design.py:135`).
	if errs := design.ValidateScript(options); len(errs) > 0 {
		return nil, hasScript, scriptValidationFailure(theme, errs)
	}
	return options, hasScript, nil
}

// scriptFailure turns `luatheme.Run`'s error into the record the panel prints,
// distinguishing the two modes it can fail in.
//
// **The two must stay apart**: a script that could not be parsed or run and a
// script that ran fine but handed back a number are different mistakes, and a
// user cannot act on a message that conflates them. gopher-lua returns a
// `*lua.ApiError` for the first and `Run`'s own error for the second, so the
// distinction is already in the type.
//
// Upstream's counterparts are a `SyntaxError` and a missing `{Theme}Theme`
// class; neither text can be reproduced, because Lua has no `SyntaxError` and a
// Lua declaration is a table with no class to be missing. That is D-013, and it
// is the only part of this failure that diverges.
func scriptFailure(theme string, err error) error {
	var apiError *lua.ApiError
	if errors.As(err, &apiError) {
		// **`Object`, not `Error()`.** `ApiError.Error` appends the Lua stack
		// traceback (`state.go:49-54`), and a runtime failure's is five lines of
		// `[G]: in function 'error'` that would be rendered as five bordered rows
		// inside the panel — `Panel` treats a newline as a hard break. The object
		// alone is the one line that names the script's own line number, which is
		// the whole reason this text is worth carrying.
		reason := strings.TrimSpace(apiError.Object.String())
		return scriptRecord(theme, "'s init.lua file could not be run: "+reason, scriptInputElided)
	}
	return scriptRecord(theme, "'s init.lua file did not return a table of theme options", scriptInputElided)
}

// scriptValidationFailure reports what `design.ValidateScript` found, one record
// per finding in the order it found them — pydantic reports every problem with a
// model's declared defaults, not just the first.
func scriptValidationFailure(theme string, errs []error) error {
	records := make([]schemaerr.ValidationError, 0, len(errs))
	for _, err := range errs {
		// **A rejected declared value is upstream's own sentence, unprefixed.**
		// `ScriptValueError` carries the text `validateField` would produce for
		// the same value in a document, which is what
		// `theme_data_model_class(**design)` prints — so this mode is parity, and
		// prefixing it to name the option would be a divergence chosen for
		// helpfulness. Upstream names no option either; its location column says
		// `design` and nothing narrower.
		var valueError *design.ScriptValueError
		if errors.As(err, &valueError) {
			records = append(records, scriptRecordOf(valueError.Message, valueError.Input))
			continue
		}
		records = append(records, scriptRecordOf(
			"The custom theme "+theme+"'s init.lua file declares an option the design tree "+
				"cannot hold: "+err.Error(), scriptInputElided))
	}
	return &schemaerr.UserValidationError{Errors: records}
}

// scriptInputElided is what upstream's Input Value column holds for a script
// failure with no single offending value: pydantic's repr of the whole `design`
// dictionary, truncated to `...`. Measured against the vendored binary.
const scriptInputElided = "..."

func scriptRecord(theme, tail, input string) error {
	return &schemaerr.UserValidationError{
		Errors: []schemaerr.ValidationError{scriptRecordOf("The custom theme "+theme+tail, input)},
	}
}

// scriptRecordOf builds one record at the location upstream reports these at.
//
// **`design`, for every mode** — upstream raises inside `validate_design`, whose
// records carry the whole design block's location and nothing narrower
// (`design.py:72-133`); measured on all four modes against the vendored binary.
//
// **The trailing period is applied here** because this record is synthesized at
// render time and so never passes through `errorpipeline.Parse`, whose step 8
// (`appendPeriod`, `pydantic_error_handling.py:94-95`) puts it on every record
// the validator produces. When T1 moves script loading into `design.Validate`
// the record will flow through `Parse` like any other and **this call must come
// back out**, or the message ends in two periods.
func scriptRecordOf(message, input string) schemaerr.ValidationError {
	if !strings.HasSuffix(message, ".") {
		message += "."
	}
	return schemaerr.ValidationError{
		SchemaLocation:  []string{"design"},
		YamlSource:      schemaerr.SourceMain,
		Message:         message,
		Input:           input,
		LocationIsFinal: true,
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

	name := model.CvModel.Name

	return process.Model{
		Name:       text(name),
		Headline:   text(model.CvModel.Headline),
		NameIsNone: name == nil || name.Kind == yamldoc.KindNull,
		Sections:   Sections(model.CvModel, registry),

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
			//
			// **This must match every spelling `ResolveScalar` classifies as
			// `KindBool`, not a hand-picked subset.** `ResolveScalar` recognizes
			// `true`/`True`/`TRUE` and `false`/`False`/`FALSE` — the YAML 1.2
			// core schema's spellings. `TRUE` is not `"true"`, `"True"`, `"yes"`
			// or `"on"`, so it used to fall through to `false` — a value ending
			// up on the *wrong* side of the boolean, not merely uncoerced. Word
			// forms like `yes`/`no`/`on`/`off` never reach this branch at all —
			// `ResolveScalar` classifies them as `KindString`, and it is
			// `normalizeBools` (spec 014's fifth re-verification) that coerces
			// those once the design tree is fully merged. Found by a
			// fresh-context verifier (iteration 14's seventh re-verification).
			out[item.Key] = yamldoc.BoolIsTrue(item.Value.Raw)
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
