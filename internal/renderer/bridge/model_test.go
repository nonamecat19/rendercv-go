package bridge_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var now = time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)

// bridged validates a whole document and runs it through the bridge, failing on
// any validation error so no case here asserts on a rejected document.
func bridged(t *testing.T, document string) process.Model {
	t.Helper()
	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}

	validated, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("the document did not validate: %v", errs)
	}

	model, err := bridge.Model(bridge.Resolve(validated, now), entries.Default())
	if err != nil {
		t.Fatalf("bridging: %v", err)
	}
	return model
}

const minimal = `
cv:
  name: John Doe
  sections:
    education:
      - institution: MIT
        area: CS
`

// Every field the templater reads has a value, defaulted or not — which is what
// `design.Effective` exists for. A zero here is a template rendering nothing at
// all, and the first golden would say so without saying where.
func TestModelIsFullyPopulated(t *testing.T) {
	model := bridged(t, minimal)

	if model.Name != "John Doe" {
		t.Errorf("name = %q", model.Name)
	}
	if model.PDFTitle != "NAME - CV" {
		t.Errorf("pdf title = %q", model.PDFTitle)
	}
	if !model.CurrentDate.Equal(now) {
		t.Errorf("current date = %v, want the reference", model.CurrentDate)
	}
	if model.Templates.SingleDate != "MONTH_ABBREVIATION YEAR" {
		t.Errorf("single date template = %q", model.Templates.SingleDate)
	}
	if model.Templates.DateRange == "" || model.Templates.TimeSpan == "" {
		t.Errorf("date templates = %+v, want all three", model.Templates)
	}
	if model.TopNoteTemplate == "" || model.FooterTemplate == "" {
		t.Errorf("top note = %q, footer = %q", model.TopNoteTemplate, model.FooterTemplate)
	}
	if model.Catalog.Present != "present" || len(model.Catalog.MonthNames) != 12 {
		t.Errorf("catalog = %+v, want English's", model.Catalog)
	}
	if model.Phrases["degree_with_area"] != "DEGREE in AREA" {
		t.Errorf("phrases = %v", model.Phrases)
	}
	if !model.ConnectionOptions.Hyperlink || !model.ConnectionOptions.ShowIcons {
		t.Errorf("connection options = %+v, want both defaults on", model.ConnectionOptions)
	}
}

// The entry-type blocks arrive keyed by the snake-case type name, which is how
// `render_entry_templates` finds one — and the scalar members of
// `design.templates` are not among them.
func TestEntryTemplatesAreKeyedByType(t *testing.T) {
	model := bridged(t, minimal)

	block, present := model.EntryTemplates["education_entry"]
	if !present {
		t.Fatalf("entry templates = %v, want an education_entry block", model.EntryTemplates)
	}
	if block["main_column"] == "" {
		t.Errorf("education block = %v, want a main column", block)
	}
	if _, wrong := model.EntryTemplates["single_date"]; wrong {
		t.Errorf("entry templates contain a scalar template member")
	}
}

// A theme changes the values without the document naming any of them, which is
// the "nothing validates a default" case `design.Effective` closed.
func TestAThemeChangesTheEffectiveValues(t *testing.T) {
	classic := bridged(t, minimal)
	moderncv := bridged(t, minimal+"\ndesign:\n  theme: moderncv\n")

	if classic.EntryTemplates["education_entry"]["main_column"] ==
		moderncv.EntryTemplates["education_entry"]["main_column"] {
		t.Errorf("the two themes produced the same education template")
	}
}

// A document's own `design` block merges over the theme's, one key at a time.
func TestTheDocumentsDesignBlockWins(t *testing.T) {
	model := bridged(t, minimal+`
design:
  header:
    connections:
      hyperlink: false
      phone_number_format: international
`)

	if model.ConnectionOptions.Hyperlink {
		t.Errorf("hyperlink = true, want the document's false")
	}
	// The sibling the document did not write keeps the theme's value rather
	// than being reset — the deep-merge case.
	if !model.ConnectionOptions.ShowIcons {
		t.Errorf("show_icons = false, want the default kept")
	}
}

// The locale block reaches the renderer as a catalog, not as a language name.
func TestTheLocaleBlockReachesTheCatalog(t *testing.T) {
	model := bridged(t, minimal+"\nlocale:\n  language: french\n")

	if model.Catalog.Present == "present" {
		t.Errorf("present = %q, want the French translation", model.Catalog.Present)
	}
}

// Settings' three fields arrive resolved: the date is a date, the keywords are
// deduplicated, and the title is still unsubstituted — `process.Run` does that.
func TestSettingsArriveResolved(t *testing.T) {
	model := bridged(t, minimal+`
settings:
  current_date: 2020-07-04
  bold_keywords:
    - Go
    - Go
  pdf_title: NAME's CV
`)

	if model.CurrentDate.Year() != 2020 {
		t.Errorf("current date = %v", model.CurrentDate)
	}
	if len(model.BoldKeywords) != 1 {
		t.Errorf("bold keywords = %v, want one", model.BoldKeywords)
	}
	if model.PDFTitle != "NAME's CV" {
		t.Errorf("pdf title = %q, want it unsubstituted", model.PDFTitle)
	}
}

// **A YAML boolean spelled `TRUE` must reach the design tree as `true`, not
// `false`.** `ResolveScalar` classifies `TRUE` (along with `True`/`true`) as
// `KindBool`, but `mappingOf`'s own conversion only recognized the literal
// spellings `"true"`, `"True"`, `"yes"`, `"on"` for that kind — `"TRUE"`
// fell through to `false`, landing on the *wrong* boolean rather than an
// uncoerced one, and reaching a built-in theme with no script involved.
// `normalizeBools` (iteration 14's fifth re-verification) never sees this,
// because by the time it runs the value is already a Go `bool`. Found by a
// fresh-context verifier (iteration 14's seventh re-verification).
func TestAllCapsBooleanReachesTheDesignTree(t *testing.T) {
	node, err := yamlreader.ReadString(minimal + "\ndesign:\n  typography:\n    bold:\n      connections: TRUE\n")
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}
	validated, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("the document did not validate: %v", errs)
	}
	doc := bridge.Resolve(validated, now)

	if got := design.EffectiveBool(doc.Design, "typography", "bold", "connections"); !got {
		t.Errorf("typography.bold.connections = %v, want true", got)
	}
}

// A broken theme script must be **reported**, not discarded (spec 014 §2
// behavior 9, tasks 014 T4). Every one of these used to return the same `nil`
// `themeScript` returns for a theme folder with no script at all, so the
// document rendered with the theme's base defaults at exit 0 with no signal —
// a silently wrong CV from a script the user got wrong.
//
// **The four modes stay distinguishable**: a user needs to know whether their
// script failed to parse, returned the wrong type, declared a shape the design
// tree cannot hold, or declared a value the field rejects.
func TestABrokenThemeScriptIsReported(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
		input  string
	}{{
		name:   "a parse error",
		script: "return {",
		want: "The custom theme mytheme's init.lua file could not be run: " +
			"<string> at EOF:   syntax error.",
		input: "...",
	}, {
		name:   "a runtime failure",
		script: "error('boom')",
		want: "The custom theme mytheme's init.lua file could not be run: " +
			"<string>:1: boom.",
		input: "...",
	}, {
		name:   "a non-table return",
		script: "return 42",
		want:   "The custom theme mytheme's init.lua file did not return a table of theme options.",
		input:  "...",
	}, {
		name:   "a shape the design tree cannot hold",
		script: `return { page = { size = { a = 1 } } }`,
		want: "The custom theme mytheme's init.lua file declares an option the design tree " +
			"cannot hold: design.page.size is a group of options in this theme's script, " +
			"but should be a value.",
		input: "...",
	}, {
		// **Upstream's own sentence, unprefixed.** `theme_data_model_class(**design)`
		// validates the declared defaults (`design.py:135`), so this text and this
		// input value are parity, not this port's wording.
		name:   "a value the field rejects",
		script: `return { page = { size = "bogus" } }`,
		want:   "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'.",
		input:  "bogus",
	}, {
		// A Lua boolean is echoed as Lua spells it (D-013): upstream prints
		// Python's `True` for the same mistake in `__init__.py`.
		name:   "a boolean where a value belongs",
		script: `return { page = { size = true } }`,
		want:   "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'.",
		input:  "true",
	}} {
		t.Run(test.name, func(t *testing.T) {
			doc := resolveWithTheme(t, "mytheme", test.script, "")

			var reported *schemaerr.UserValidationError
			if !errors.As(doc.ScriptError, &reported) {
				t.Fatalf("ScriptError = %v, want a validation error", doc.ScriptError)
			}
			if len(reported.Errors) != 1 {
				t.Fatalf("got %d records, want 1: %v", len(reported.Errors), reported.Errors)
			}
			record := reported.Errors[0]
			if record.Message != test.want {
				t.Errorf("message = %q, want %q", record.Message, test.want)
			}
			if record.Input != test.input {
				t.Errorf("input = %q, want %q", record.Input, test.input)
			}
			// Upstream's location column reads `design` for every one of these
			// (`design.py:67`'s `loc` override and the model's own position),
			// measured against the vendored binary.
			if got := strings.Join(record.SchemaLocation, "."); got != "design" {
				t.Errorf("location = %q, want %q", got, "design")
			}
		})
	}
}

// **"Absent" and "broken" must take different paths**, which is the whole of
// T4's finding: a theme folder with no `init.lua` is not a failure. Measured on
// both sides — upstream renders it at exit 0 and so does this port.
func TestAnAbsentThemeScriptIsSilent(t *testing.T) {
	doc := resolveWithThemeFolder(t, "")
	if doc.ScriptError != nil {
		t.Fatalf("ScriptError = %v, want nil for a theme folder with no script", doc.ScriptError)
	}
	// The fallback it already had: the theme's base defaults, still rendering.
	if got := design.EffectiveString(doc.Design, "page", "size"); got != "us-letter" {
		t.Errorf("page.size = %q, want the base default", got)
	}
}

// A **working** script is not a failure either — the guard must not fire on the
// path every scripted theme takes.
func TestAWorkingThemeScriptReportsNothing(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { page = { size = "a5" } }`, "")
	if doc.ScriptError != nil {
		t.Fatalf("ScriptError = %v, want nil", doc.ScriptError)
	}
	if got := design.EffectiveString(doc.Design, "page", "size"); got != "a5" {
		t.Errorf("page.size = %q, want a5", got)
	}
}

// resolveWithThemeFolder is `resolveWithTheme` for the one case it cannot
// express: a custom theme folder that **exists** — it has to, or validation
// rejects the theme before the script is ever looked for (`design.py:82-86`) —
// but contains no `init.lua`. That is the absent-script vector, and telling it
// apart from a broken script is T4's whole finding.
func resolveWithThemeFolder(t *testing.T, script string) bridge.Document {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	document := "cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"

	if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mytheme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mytheme", "Preamble.j2.typ"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		if err := os.WriteFile(filepath.Join(dir, "mytheme", "init.lua"), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatal(err)
	}
	model, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now, InputFilePath: input}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("did not validate: %v", errs)
	}
	return bridge.Resolve(model, now)
}
