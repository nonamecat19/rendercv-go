package bridge_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// resolveWithTheme writes an input file and an optional `<theme>/init.lua`
// beside it, then resolves — which is where the script has to be found, because
// it must run before anything reads the effective tree.
func resolveWithTheme(t *testing.T, theme, script, block string) bridge.Document {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")

	document := "cv:\n  name: John Doe\ndesign:\n  theme: " + theme + "\n" + block
	if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		if err := os.MkdirAll(filepath.Join(dir, theme), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, theme, "init.lua"), []byte(script), 0o644); err != nil {
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

// Spec 014 §1 behaviors 1 and 4, at the position they actually run.
func TestAThemeScriptIsFoundAndRun(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme",
		`return { colors = { name = "rgb(1, 2, 3)" } }`, "")

	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(1, 2, 3)" {
		t.Errorf("colors.name = %q, want the script's default", got)
	}
}

// The document still wins over the script, through the whole pipeline rather
// than only in `EffectiveWithScript`'s unit test.
func TestTheDocumentBeatsTheScript(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme",
		`return { colors = { name = "rgb(1, 2, 3)" } }`,
		"  colors:\n    name: rgb(9, 9, 9)\n")

	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(9, 9, 9)" {
		t.Errorf("colors.name = %q, want the document's value", got)
	}
}

// **A theme folder with no script is valid**, and a built-in theme is
// unaffected — the path all nine of them and all 24 corpus documents take.
func TestNoScriptIsUnchanged(t *testing.T) {
	scripted := resolveWithTheme(t, "classic", "", "")
	plain := design.Effective("classic", nil)

	for _, path := range [][]string{{"colors", "name"}, {"page", "size"}} {
		if design.EffectiveString(scripted.Design, path...) !=
			design.EffectiveString(plain, path...) {
			t.Errorf("%v changed for a theme with no script", path)
		}
	}
}

// **A built-in theme ignores an `init.lua` beside the input.** Upstream only
// enters the custom-theme path when the built-in discriminator fails, so a
// `classic/init.lua` is never read there. Reading it changed a built-in theme's
// artifact — `page-size: "a5"` where upstream emits `"us-letter"` — from a file
// a user could drop next to their CV without touching the document. Found by a
// fresh-context verifier.
func TestABuiltinThemeIgnoresAScript(t *testing.T) {
	doc := resolveWithTheme(t, "classic",
		`return { page = { size = "a5" }, colors = { name = "rgb(255, 0, 0)" } }`, "")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "us-letter" {
		t.Errorf("page.size = %q, want classic's own default", got)
	}
	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(0, 79, 144)" {
		t.Errorf("colors.name = %q, want classic's own default", got)
	}
}

// Every built-in name is protected, not just the default one.
func TestEveryBuiltinIgnoresAScript(t *testing.T) {
	for _, theme := range []string{"classic", "sb2nov", "moderncv", "engineeringresumes"} {
		doc := resolveWithTheme(t, theme, `return { page = { size = "a5" } }`, "")
		if got := design.EffectiveString(doc.Design, "page", "size"); got == "a5" {
			t.Errorf("%s read a theme script", theme)
		}
	}
}
