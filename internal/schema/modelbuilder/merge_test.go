package modelbuilder

import (
	"errors"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

const minimalCV = "cv:\n  name: John Doe\n"

func mustBuild(t *testing.T, mainYaml string, args BuildArguments) *BuildResult {
	t.Helper()
	result, err := BuildDictionary(mainYaml, args)
	if err != nil {
		t.Fatalf("BuildDictionary: %v", err)
	}
	return result
}

func get(t *testing.T, node *yamldoc.Node, path ...string) *yamldoc.Node {
	t.Helper()
	current := node
	for _, key := range path {
		value, ok := mappingGet(current, key)
		if !ok {
			t.Fatalf("key %q not found in %v", key, path)
		}
		current = value
	}
	return current
}

// Spec §3.14 — settings and settings.render_command are force-created.
func TestSettingsDefaulting(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "no settings", input: minimalCV},
		{name: "settings without render_command", input: minimalCV + "settings:\n  bold_keywords: []\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mustBuild(t, tc.input, BuildArguments{})
			renderCommand := get(t, result.Document, "settings", "render_command")
			if renderCommand.Kind != yamldoc.KindMapping {
				t.Fatalf("render_command kind = %v, want mapping", renderCommand.Kind)
			}
		})
	}
}

// Spec §3.15, §3.16, §3.17 — fixed order, own top-level key only, replace not merge.
func TestOverlayReplacesWholesale(t *testing.T) {
	main := minimalCV + "design:\n  theme: classic\n  font_size: 12pt\n"
	result := mustBuild(t, main, BuildArguments{
		DesignYaml: "design:\n  theme: sb2nov\n",
	})

	design := get(t, result.Document, "design")
	if len(design.Items) != 1 || design.Items[0].Key != "theme" {
		t.Fatalf("design = %+v, want exactly {theme}", design.Items)
	}
	if got := design.Items[0].Value.Raw; got != "sb2nov" {
		t.Errorf("design.theme = %q, want %q", got, "sb2nov")
	}
}

// Spec §3.15 — an absent or empty overlay is skipped.
func TestEmptyOverlayIsSkipped(t *testing.T) {
	main := minimalCV + "design:\n  theme: classic\n"
	result := mustBuild(t, main, BuildArguments{DesignYaml: ""})

	if got := get(t, result.Document, "design", "theme").Raw; got != "classic" {
		t.Errorf("design.theme = %q, want %q", got, "classic")
	}
	if len(result.OverlaySources) != 0 {
		t.Errorf("overlay sources = %v, want none", result.OverlaySources)
	}
}

// Spec §3.15 — overlays apply in the order settings, design, locale.
func TestOverlayOrder(t *testing.T) {
	want := []schemaerr.OverlayKey{
		schemaerr.OverlaySettings,
		schemaerr.OverlayDesign,
		schemaerr.OverlayLocale,
	}
	for i, key := range overlayOrder {
		if key != want[i] {
			t.Fatalf("overlayOrder = %v, want %v", overlayOrder, want)
		}
	}
}

// Spec §3.18 — each overlay's parsed document is retained, keyed by overlay name.
func TestOverlaySourcesRetained(t *testing.T) {
	result := mustBuild(t, minimalCV, BuildArguments{
		SettingsYaml: "settings:\n  bold_keywords: []\n",
		DesignYaml:   "design:\n  theme: sb2nov\n",
		LocaleYaml:   "locale:\n  language: en\n",
	})

	for _, key := range overlayOrder {
		source, ok := result.OverlaySources[key]
		if !ok {
			t.Fatalf("overlay source for %q missing", key)
		}
		if _, ok := mappingGet(source, string(key)); !ok {
			t.Errorf("retained %q document lacks its own key", key)
		}
	}
}

// Spec §5.18 — an overlay document without its own key does not validate.
func TestOverlayWithoutItsOwnKey(t *testing.T) {
	_, err := BuildDictionary(minimalCV, BuildArguments{DesignYaml: "theme: sb2nov\n"})
	var internal *schemaerr.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("error = %v (%T), want *schemaerr.InternalError", err, err)
	}
}

// Spec §3.19 — the eleven render-command keys, in upstream's order.
func TestRenderCommandOverrideKeys(t *testing.T) {
	want := []string{
		"output_folder", "typst_path", "pdf_path", "markdown_path", "html_path", "png_path",
		"dont_generate_typst", "dont_generate_html", "dont_generate_markdown",
		"dont_generate_pdf", "dont_generate_png",
	}
	got := renderCommandOverrides(BuildArguments{})
	if len(got) != len(want) {
		t.Fatalf("got %d override keys, want %d", len(got), len(want))
	}
	for i, override := range got {
		if override.key != want[i] {
			t.Errorf("override[%d] = %q, want %q", i, override.key, want[i])
		}
	}
}

// Spec §3.19 — truthy overrides are written into settings.render_command.
func TestRenderCommandOverridesWritten(t *testing.T) {
	result := mustBuild(t, minimalCV, BuildArguments{
		OutputFolder:      "out",
		PdfPath:           "cv.pdf",
		DontGeneratePng:   true,
		DontGenerateTypst: true,
	})

	renderCommand := get(t, result.Document, "settings", "render_command")
	tests := []struct {
		key  string
		want string
		kind yamldoc.Kind
	}{
		{key: "output_folder", want: "out", kind: yamldoc.KindString},
		{key: "pdf_path", want: "cv.pdf", kind: yamldoc.KindString},
		{key: "dont_generate_typst", want: "true", kind: yamldoc.KindBool},
		{key: "dont_generate_png", want: "true", kind: yamldoc.KindBool},
	}
	for _, tc := range tests {
		value, ok := mappingGet(renderCommand, tc.key)
		if !ok {
			t.Fatalf("%s not written", tc.key)
		}
		if value.Raw != tc.want || value.Kind != tc.kind {
			t.Errorf("%s = %+v, want raw %q kind %v", tc.key, value, tc.want, tc.kind)
		}
	}
}

// Spec §3.20, §5.17 — falsy overrides are dropped and do not overwrite YAML values.
func TestFalsyOverridesAreDropped(t *testing.T) {
	main := minimalCV + "settings:\n  render_command:\n    dont_generate_pdf: true\n    output_folder: from_yaml\n"
	result := mustBuild(t, main, BuildArguments{
		DontGeneratePdf: false,
		OutputFolder:    "",
	})

	renderCommand := get(t, result.Document, "settings", "render_command")
	if got := get(t, renderCommand, "dont_generate_pdf").Raw; got != "true" {
		t.Errorf("dont_generate_pdf = %q, want the YAML value %q", got, "true")
	}
	if got := get(t, renderCommand, "output_folder").Raw; got != "from_yaml" {
		t.Errorf("output_folder = %q, want the YAML value %q", got, "from_yaml")
	}
}

// Spec §3.21 — dotted-key overrides are applied last, and they are applied.
//
// **This test used to assert the opposite.** `applyOverrides` was a stub
// returning the document unchanged, and the test pinned that no-op as if it were
// behavior — so `--cv.phone`, `--design.theme` and `--settings.current_date`
// were all silently discarded, and four corpus cases could never pass.
func TestDottedOverridesAreApplied(t *testing.T) {
	result := mustBuild(t, minimalCV, BuildArguments{
		Overrides: map[string]string{"cv.name": "Jane Doe"},
	})
	if got := get(t, result.Document, "cv", "name").Raw; got != "Jane Doe" {
		t.Errorf("cv.name = %q, want the override %q", got, "Jane Doe")
	}
}

// A path whose intermediate mappings do not exist grows them
// (override_dictionary.py:74-76).
func TestDottedOverridesCreateMissingMappings(t *testing.T) {
	result := mustBuild(t, minimalCV, BuildArguments{
		Overrides: map[string]string{"design.typography.font_size.body": "12pt"},
	})
	got := get(t, result.Document, "design", "typography", "font_size", "body")
	if got.Raw != "12pt" {
		t.Errorf("= %q, want the override", got.Raw)
	}
}

// **A list does not grow to meet the path**, which is the asymmetry with
// mappings: an out-of-range index is a user error rather than a new element.
func TestDottedOverrideIndexOutOfRange(t *testing.T) {
	main := "cv:\n  sections:\n    education:\n      - institution: A\n        area: B\n"
	_, err := BuildDictionary(main, BuildArguments{
		Overrides: map[string]string{"cv.sections.education.3.institution": "MIT"},
	})
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("err = %v, want an out-of-range complaint", err)
	}
}

// An index into a list has to be an integer.
func TestDottedOverrideNonIntegerIndex(t *testing.T) {
	main := "cv:\n  sections:\n    education:\n      - institution: A\n        area: B\n"
	_, err := BuildDictionary(main, BuildArguments{
		Overrides: map[string]string{"cv.sections.education.first.institution": "MIT"},
	})
	if err == nil || !strings.Contains(err.Error(), "not an integer") {
		t.Errorf("err = %v, want a non-integer complaint", err)
	}
}

// The indexed form the corpus uses (`render_override_indexed`).
func TestDottedOverrideIndexedEntry(t *testing.T) {
	main := "cv:\n  sections:\n    education:\n      - institution: A\n        area: B\n"
	result := mustBuild(t, main, BuildArguments{
		Overrides: map[string]string{"cv.sections.education.0.institution": "MIT"},
	})
	entry := get(t, result.Document, "cv", "sections", "education").Elems[0]
	if got := get(t, entry, "institution").Raw; got != "MIT" {
		t.Errorf("institution = %q, want MIT", got)
	}
	if got := get(t, entry, "area").Raw; got != "B" {
		t.Errorf("area = %q, want the sibling untouched", got)
	}
}

// Spec §3.14 — an existing settings mapping keeps its position and its own keys.
func TestSettingsDefaultingPreservesKeyOrder(t *testing.T) {
	main := "settings:\n  bold_keywords: []\ncv:\n  name: John Doe\n"
	result := mustBuild(t, main, BuildArguments{})

	if result.Document.Items[0].Key != "settings" || result.Document.Items[1].Key != "cv" {
		t.Fatalf("top-level order = %q, %q", result.Document.Items[0].Key, result.Document.Items[1].Key)
	}
	settings := get(t, result.Document, "settings")
	if settings.Items[0].Key != "bold_keywords" {
		t.Errorf("settings[0] = %q, want bold_keywords first", settings.Items[0].Key)
	}
	if settings.Items[len(settings.Items)-1].Key != "render_command" {
		t.Errorf("settings last key = %q, want render_command appended", settings.Items[len(settings.Items)-1].Key)
	}
}

// A YAML syntax error in an overlay is reported against that overlay's source (spec §3.83).
func TestOverlaySyntaxErrorCarriesOverlaySource(t *testing.T) {
	_, err := BuildDictionary(minimalCV, BuildArguments{LocaleYaml: "locale:\n\tlanguage: en\n"})

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("error = %v (%T), want *schemaerr.UserValidationError", err, err)
	}
	if got := userErr.Errors[0].YamlSource; got != schemaerr.SourceLocale {
		t.Errorf("yaml source = %q, want %q", got, schemaerr.SourceLocale)
	}
}

// Spec §5.28 — a `design` overlay leaves a `locale` in the main document alone.
func TestOverlaysAreIndependent(t *testing.T) {
	main := minimalCV + "locale:\n  language: tr\ndesign:\n  theme: classic\n"
	result := mustBuild(t, main, BuildArguments{DesignYaml: "design:\n  theme: sb2nov\n"})

	if got := get(t, result.Document, "design", "theme").Raw; got != "sb2nov" {
		t.Errorf("design.theme = %q, want %q", got, "sb2nov")
	}
	if got := get(t, result.Document, "locale", "language").Raw; got != "tr" {
		t.Errorf("locale.language = %q, want the main document's %q", got, "tr")
	}
}
