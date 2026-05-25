package bridge_test

import (
	"os"
	"path/filepath"
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
//
// The block names its theme because a `design` mapping with no `theme` key is a
// `union_tag_not_found` record now (`validate.go`), matching upstream's exit 1.
func TestTheDocumentsDesignBlockWins(t *testing.T) {
	model := bridged(t, minimal+`
design:
  theme: classic
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
	node, err := yamlreader.ReadString(minimal +
		"\ndesign:\n  theme: classic\n  typography:\n    bold:\n      connections: TRUE\n")
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

// The four broken-script modes, the absent script and the working script are
// pinned where the record is now produced: `design.Validate`, whose raw records
// `internal/schema/models/design/scriptfailure_test.go` asserts, and the panel
// a user reads, which `internal/cli/themescript_test.go` asserts after
// `errorpipeline.Parse`. They lived here while `bridge.themeScript` synthesized
// the record; it does not any more, and `Document.ScriptError` is gone with it.

// **Validation and render must resolve the theme folder to the same directory.**
//
// `design.Validate` resolves the input file's parent with `uncleanedDir`, which
// is `PurePath.parent` — purely lexical, a `..` segment kept verbatim, because
// that is what upstream does. `themeScript` used `filepath.Dir`, which calls
// `Clean` and collapses `..`. On an ordinary tree the two spellings name the
// same file and nothing is observable; **through a symlink they do not**, and
// the document is then validated against one script and rendered with another.
//
// Measured on this exact layout against the vendored binary: upstream renders
// the *lexical* directory's theme, `bb/../bb`, and it writes its output there
// too. `filepath.Dir` is the side that does not match.
func TestTheThemeScriptResolvesTheWayValidationDoes(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	lexical := filepath.Join(root, "other", "bb")
	cleaned := filepath.Join(root, "other", "real")

	for _, dir := range []string{work, filepath.Join(lexical, "mytheme"), filepath.Join(cleaned, "mytheme")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// `work/bb` points at `other/real`, so `work/bb/..` is `other` and
	// `work/bb/../bb` is `other/bb` — a different directory from `work/bb`.
	if err := os.Symlink(cleaned, filepath.Join(work, "bb")); err != nil {
		t.Skipf("this filesystem does not do symlinks: %v", err)
	}

	for dir, size := range map[string]string{lexical: "a4", cleaned: "a5"} {
		theme := filepath.Join(dir, "mytheme")
		script := `return { page = { size = "` + size + `" } }`
		if err := os.WriteFile(filepath.Join(theme, "init.lua"), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(theme, "Preamble.j2.typ"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	document := "cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"
	// **Assembled by hand, because `filepath.Join` would clean it** — the `..`
	// segment is the whole point of the vector, and `Join` collapses it before
	// the code under test ever sees it. This is the string the CLI receives from
	// `rendercv-go render ./bb/../bb/CV.yaml`.
	input := work + "/bb/../bb/CV.yaml"
	if err := os.WriteFile(filepath.Join(lexical, "CV.yaml"), []byte(document), 0o644); err != nil {
		t.Fatal(err)
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

	doc := bridge.Resolve(model, now)
	if got := design.EffectiveString(doc.Design, "page", "size"); got != "a4" {
		t.Errorf("page.size = %q, want a4 — the render read %s's script where validation "+
			"read %s's", got, cleaned, lexical)
	}
}
