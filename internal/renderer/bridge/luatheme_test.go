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
		// A custom theme folder with no `*.j2.typ` in it is rejected during
		// validation (`design.py:82-86`), so the folder needs one before the
		// script can be reached at all.
		if err := os.WriteFile(filepath.Join(dir, theme, "Preamble.j2.typ"), nil, 0o644); err != nil {
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

// A script whose shapes conflict with the design tree is dropped, rather than
// reaching a template and printing a Go type name into the artifact — the
// verifier's blocker 2, which produced `page-size: "<map[string]interface {}
// Value>"` at exit 0 under "Your CV is ready".
func TestAConflictingScriptIsDropped(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme",
		`return { page = { size = { a = 1 } }, colors = { name = { r = 1 } } }`, "")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "us-letter" {
		t.Errorf("page.size = %q, want the theme's own default", got)
	}
	if got := design.EffectiveString(doc.Design, "colors", "name"); got == "" ||
		got[0] == '<' {
		t.Errorf("colors.name = %q, want a real colour", got)
	}
}

// A script that is *correct* still applies — the drop is targeted, not a
// blanket refusal of scripts that touch declared options.
func TestACorrectScriptStillApplies(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { page = { size = "a5" } }`, "")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "a5" {
		t.Errorf("page.size = %q, want the script's value", got)
	}
}

// **A script-less custom theme discards the whole document design block**, not
// just the options a script would have declared. Upstream's fallback
// (`ThemeOptionsAreNotProvided(theme=theme_name)`, `design.py:139-142`) carries
// only `theme`; a document overriding `colors.name` on a theme with no
// `init.lua` is silently ignored there, and the port used to merge it anyway
// (verifier finding, iteration 14's second re-verification).
func TestANoScriptCustomThemeDiscardsTheWholeDocument(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	document := "cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n  colors:\n    name: rgb(9, 9, 9)\n"
	if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mytheme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mytheme", "Preamble.j2.typ"), nil, 0o644); err != nil {
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

	classic := design.EffectiveString(design.Effective("classic", nil), "colors", "name")
	if got := design.EffectiveString(doc.Design, "colors", "name"); got != classic {
		t.Errorf("colors.name = %q, want classic's own default %q (document discarded)", got, classic)
	}
}

// **A document value that conflicts with a *tree*-typed field leaks the same
// way a script conflict does, on the theme `create-theme` itself writes.**
// `create-theme` generates `init.lua` as `return {}` — an empty, but present,
// script — so `luatheme.Validate` (which only walks keys the script declared)
// sees nothing to flag, and `page.size: {a: 1}` used to merge straight through
// to the template as a Go type name at exit 0.
func TestADocumentConflictingWithTheTreeIsDropped(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return {}`, "  page:\n    size:\n      a: 1\n")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "us-letter" {
		t.Errorf("page.size = %q, want the tree's own default, document's conflicting override dropped", got)
	}
}

// **The one documented scalar-over-mapping override must survive tree-conflict
// pruning.** `typography.font_family: Charter` legitimately replaces the
// five-element `FontFamily` model wholesale (spec 006 §3.2 behavior 14); an
// earlier version of `withoutTreeConflicts` did not know that and silently
// discarded it on any custom theme, a regression a verifier caught the same
// day the pruning shipped.
func TestFontFamilyStringOverrideSurvivesTreeConflictPruning(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return {}`,
		"  typography:\n    font_family: Charter\n")

	if got := design.EffectiveString(doc.Design, "typography", "font_family", "body"); got != "Charter" {
		t.Errorf("font_family.body = %q, want the document's override widened", got)
	}
}

// **A list where a scalar belongs leaks the same way a mapping does.** The
// first version of `withoutTreeConflicts` only compared map-vs-non-map, so
// `page.size: [a4]` on an empty (`create-theme`'s own) script fell through
// unchanged and printed `<[]string Value>` into the artifact.
func TestAListWhereAScalarBelongsIsDropped(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return {}`, "  page:\n    size:\n      - a4\n")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "us-letter" {
		t.Errorf("page.size = %q, want the tree's own default, document's list dropped", got)
	}
}

// **A script that fails to parse must not discard the document.** Both a
// missing `init.lua` and a broken one hand `EffectiveWithScript` a nil
// `script`, but only the missing case is upstream's
// `ThemeOptionsAreNotProvided` fallback. Conflating them used to discard a
// user's whole `design` block on a theme whose script merely had a typo — a
// worse outcome than before the no-script fix shipped.
func TestABrokenScriptStillAppliesTheDocument(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `this is not lua ((`,
		"  colors:\n    name: rgb(9, 9, 9)\n")

	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(9, 9, 9)" {
		t.Errorf("colors.name = %q, want the document's value even though the script is broken", got)
	}
}

// A *document* value that mismatches what the script declared is dropped, and
// the script's own value survives underneath it. `ValidateScript` alone
// cannot catch this: `page.size` declared as a string is a perfectly valid
// script on its own, and only fails once the document's own
// `page: {size: {a: 1}}` is checked against it — `luatheme.Validate`'s job,
// which nothing called before this test (iteration 14's Finding 5).
func TestADocumentConflictingWithTheScriptIsDropped(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { page = { size = "a5" } }`,
		"  page:\n    size:\n      a: 1\n")

	if got := design.EffectiveString(doc.Design, "page", "size"); got != "a5" {
		t.Errorf("page.size = %q, want the script's own value, document's conflicting override dropped", got)
	}
}
