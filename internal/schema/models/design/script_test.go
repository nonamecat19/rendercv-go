package design_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// writeTheme builds a custom theme folder the way `create-theme` does — one
// `*.j2.typ` so `ValidateCustomThemeFolder` passes — plus the `init.lua` the
// caller wants, and returns the input file path a document beside it would
// have.
func writeTheme(t *testing.T, theme, script string) string {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, theme)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "Header.j2.typ"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		if err := os.WriteFile(filepath.Join(folder, "init.lua"), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "CV.yaml")
}

// LoadThemeScript is the loading `Validate` performs, so what it reports is
// exactly what the validation layer can see about a scripted theme. Before
// this, `<theme>/init.lua` was read only at render time in `bridge.Resolve`
// and nothing about a custom theme's declared shape existed at validation
// time at all — the prerequisite upstream gets from
// `theme_data_model_class(**design)` (`design.py:135`).
func TestLoadThemeScript(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		wantExists bool
		wantFields map[string]bool
	}{
		{
			name: "a script's declared fields are visible",
			script: `return {
				custom_note = "hello",
				page = { size = "a5" },
			}`,
			wantExists: true,
			wantFields: map[string]bool{"custom_note": true, "page": true},
		},
		{
			name:       "an empty declaration is still a script",
			script:     "return {}",
			wantExists: true,
			wantFields: map[string]bool{},
		},
		{
			// A script that cannot be parsed reports no options, and
			// `Exists` is what keeps it distinguishable from a theme with
			// no script at all — the distinction
			// `EffectiveWithScript` already depends on.
			name:       "a broken script exists but declares nothing",
			script:     "return {",
			wantExists: true,
			wantFields: map[string]bool{},
		},
		{
			// A declaration the base tree cannot hold is dropped whole,
			// which is what `ValidateScript` has always done at render
			// time.
			name:       "a script conflicting with the tree declares nothing",
			script:     `return { page = "not a group" }`,
			wantExists: true,
			wantFields: map[string]bool{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := writeTheme(t, "mytheme", test.script)

			script := design.LoadThemeScript(filepath.Dir(input), "mytheme")

			if script.Exists != test.wantExists {
				t.Errorf("Exists = %v, want %v", script.Exists, test.wantExists)
			}
			for field := range test.wantFields {
				if _, ok := script.Options[field]; !ok {
					t.Errorf("Options is missing %q, has %v", field, script.Options)
				}
			}
			if len(script.Options) != len(test.wantFields) {
				t.Errorf("Options = %v, want exactly %v", script.Options, test.wantFields)
			}
		})
	}
}

// **A theme folder with no `init.lua` is not a failure**, and the two cases
// have to stay distinguishable: upstream's no-module fallback
// (`design.py:137-142`) discards the document's whole `design` block, and a
// script that merely broke must not be treated the same way.
func TestLoadThemeScriptWithoutAScript(t *testing.T) {
	input := writeTheme(t, "mytheme", "")

	script := design.LoadThemeScript(filepath.Dir(input), "mytheme")

	if script.Exists {
		t.Error("Exists = true, want false — the folder has no init.lua")
	}
	if script.Options != nil {
		t.Errorf("Options = %v, want nil", script.Options)
	}
}

// **A built-in theme never reads a script.** Upstream only enters the
// custom-theme path when the built-in discriminator fails
// (`design.py:36-50`), so a `classic/init.lua` sitting beside a CV must not
// change anything — the same rule `bridge.themeScript` already follows at
// render time.
func TestLoadThemeScriptIgnoresBuiltInThemes(t *testing.T) {
	input := writeTheme(t, "classic", `return { page = { size = "a5" } }`)

	script := design.LoadThemeScript(filepath.Dir(input), "classic")

	if script.Exists || script.Options != nil {
		t.Errorf("script = %+v, want the zero value for a built-in theme", script)
	}
}

// T1 loads the script during validation and changes nothing else. These are
// the documents that validate today: they must still validate, with the
// script now loaded on the way through.
func TestValidateWithAThemeScriptIsUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		script string
		yaml   string
	}{
		{
			name:   "a scripted theme validates",
			script: `return { custom_note = "hello" }`,
			yaml:   "theme: mytheme\n",
		},
		{
			// A scripted theme's own declared option is its own — the
			// built-in tree does not describe it, and T2/T3 are what
			// make anything here report.
			name:   "a script-declared option validates",
			script: `return { custom_note = "hello" }`,
			yaml:   "theme: mytheme\ncustom_note: world\n",
		},
		{
			name:   "a theme with no script validates",
			script: "",
			yaml:   "theme: mytheme\n",
		},
		{
			name:   "a broken script does not fail validation",
			script: "return {",
			yaml:   "theme: mytheme\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := writeTheme(t, "mytheme", test.script)
			doc, err := yamlreader.ReadString(test.yaml)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}

			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain,
				&valctx.ValidationContext{InputFilePath: input})

			if len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}
