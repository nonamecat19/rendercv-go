package bridge_test

import (
	"os"
	"path/filepath"
	"strings"
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

// rejectWithTheme is resolveWithTheme's other half: it asserts the document
// does **not** validate, and returns the errors so a caller can check where
// they landed. A document that never validates never reaches `bridge.Resolve`,
// which is the whole point of the two cases that use it.
func rejectWithTheme(t *testing.T, theme, script, block string) []schemaerr.ValidationError {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")

	document := "cv:\n  name: John Doe\ndesign:\n  theme: " + theme + "\n" + block
	if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, theme), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, theme, "init.lua"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, theme, "Preamble.j2.typ"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatal(err)
	}
	_, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now, InputFilePath: input}, schemaerr.SourceMain)
	if len(errs) == 0 {
		t.Fatal("validated, want a rejection")
	}
	return errs
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

// A script whose shapes conflict with the design tree is **reported** now, so
// it never reaches this layer: `design.Validate` refuses the document and
// `Resolve` is not called. It used to be dropped silently here, which is what
// kept `page-size: "<map[string]interface {} Value>"` out of the artifact while
// the failure had nowhere to be reported. The mode is pinned in the design
// package's `scriptfailure_test.go`, on this exact script shape.

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

// **A document value that conflicts with a *tree*-typed field is now rejected,
// not pruned.** This used to assert the merge layer silently dropped
// `page.size: {a: 1}` and rendered the tree's default, which was the best that
// could be done while a scripted theme's document values were never validated.
// They are now, so the document never reaches `bridge.Resolve` at all and the
// merge-layer pruning this exercised is unreachable through a tree-typed
// field — `withoutTreeConflicts` still guards a *script*-declared option,
// which `TestADocumentConflictingWithTheScriptIsDropped` covers.
//
// Upstream refuses the same document, exit 1: `theme_data_model_class(**design)`
// (`design.py:135`) validates it against the class `create-theme` generated
// from `classic_theme.py`, whose `page.size` is the same literal set. It
// refuses it by *crashing* in its own error formatter rather than printing a
// table (`pydantic_error_handling.py:53-55` strips a location element a
// scripted theme's error does not have), so the port matches the exit code and
// the refusal and prints its own clean record at the true location.
func TestADocumentConflictingWithTheTreeIsRejected(t *testing.T) {
	errs := rejectWithTheme(t, "mytheme", `return {}`, "  page:\n    size:\n      a: 1\n")

	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.page.size" {
		t.Errorf("location = %q, want design.page.size", got)
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

// **The font_family exemption must work in the other direction too**: a
// script declaring a *scalar* `font_family` against a document overriding it
// with the *mapping* form. `luatheme.Validate` classifies "a value" and "a
// group of options" as different kinds and flagged this as a conflict even
// after `withoutTreeConflicts` stopped doing the same — a second place the
// same exemption was needed, found by the same verifier pass.
func TestFontFamilyMappingOverrideSurvivesScriptConflictPruning(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { typography = { font_family = "Lato" } }`,
		"  typography:\n    font_family:\n      body: Charter\n")

	if got := design.EffectiveString(doc.Design, "typography", "font_family", "body"); got != "Charter" {
		t.Errorf("font_family.body = %q, want the document's mapping override", got)
	}

	// **A partial mapping override is not a deep merge onto the script's
	// scalar.** Upstream builds a fresh `FontFamily` from the document's
	// mapping, so the four fields the document does not mention fall back to
	// `FontFamily`'s own base default ("Source Sans 3") — not the script's
	// "Lato", and not the empty string a naive merge onto the script's
	// (by-then-scalar) value produces. Found by a fresh-context verifier
	// (iteration 14's fifth re-verification).
	for _, sibling := range []string{"name", "headline", "connections", "section_titles"} {
		if got := design.EffectiveString(doc.Design, "typography", "font_family", sibling); got != "Source Sans 3" {
			t.Errorf("font_family.%s = %q, want the base FontFamily default, not the script's or empty", sibling, got)
		}
	}
}

// **A list where a scalar belongs is rejected for the same reason a mapping
// is.** This used to assert the merge layer pruned `page.size: [a4]` before it
// reached a template as `<[]string Value>`; a scripted theme's document values
// are validated against the tree now, so the document is refused at validation
// and the pruning it exercised is unreachable through a tree-typed field.
// Upstream refuses it too, exit 1, by the same crash-in-the-formatter route as
// the mapping case above.
func TestAListWhereAScalarBelongsIsRejected(t *testing.T) {
	errs := rejectWithTheme(t, "mytheme", `return {}`, "  page:\n    size:\n      - a4\n")

	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.page.size" {
		t.Errorf("location = %q, want design.page.size", got)
	}
}

// **A broken script must not be conflated with an absent one in the merge.**
// Both hand `EffectiveWithScript` a nil `script`, but only the absent case is
// upstream's `ThemeOptionsAreNotProvided` fallback, which discards the
// document's whole `design` block.
//
// It is asserted against `EffectiveWithScript` directly rather than through
// `Resolve`, because a broken script is exit 1 at validation now and never
// reaches the merge from the CLI. The distinction still has to hold: it is the
// difference between two `hasScript` values on a function the renderer calls,
// and conflating them is a defect a verifier has already found once.
func TestABrokenScriptDoesNotDiscardTheDocument(t *testing.T) {
	document := map[string]any{"colors": map[string]any{"name": "rgb(9, 9, 9)"}}

	broken := design.EffectiveWithScript("mytheme", nil, document, true)
	if got := design.EffectiveString(broken, "colors", "name"); got != "rgb(9, 9, 9)" {
		t.Errorf("colors.name = %q, want the document's value for a broken script", got)
	}

	absent := design.EffectiveWithScript("mytheme", nil, document, false)
	if got := design.EffectiveString(absent, "colors", "name"); got == "rgb(9, 9, 9)" {
		t.Error("colors.name kept the document's value for an absent script, want it discarded")
	}
}

// **A script declaring the tree's one list-valued option must not lose every
// other option in the same table.** `luatheme.Options` used to convert a Lua
// sequence to an empty map, which `design.ValidateScript` then saw as a shape
// conflict and dropped the whole script — a theme setting
// `sections.show_time_spans_in` alongside a `colors` override lost the colour
// too, silently, at exit 0. Found by a fresh-context verifier (iteration 14's
// fourth re-verification).
func TestAScriptListOptionDoesNotDropTheRestOfTheScript(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme",
		`return { sections = { show_time_spans_in = { "Experience" } }, colors = { name = "rgb(1, 2, 3)" } }`,
		"")

	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(1, 2, 3)" {
		t.Errorf("colors.name = %q, want the script's colour — the list option must not have dropped the whole script", got)
	}
}

// A *document* value that mismatches what the script declared is **rejected**,
// which is the last of these to move from the merge layer to validation.
//
// It used to assert the merge dropped `custom_note: {a: 1}` and kept the
// script's `"hello"` underneath — the same shape of assertion `page.size`'s two
// neighbours above already outgrew. A script-declared option's document value
// is validated against the script's declared type now, so this document never
// reaches `bridge.Resolve` at all. Upstream refuses it too: measured, exit 1,
// `Input should be a valid string.` at `design`, and the port's stdout for this
// vector is byte-identical to it.
func TestADocumentConflictingWithTheScriptIsRejected(t *testing.T) {
	errs := rejectWithTheme(t, "mytheme", `return { custom_note = "hello" }`,
		"  custom_note:\n    a: 1\n")

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design" {
		t.Errorf("location = %q, want design", got)
	}
	if !strings.Contains(errs[0].Message, "Input should be a valid string") {
		t.Errorf("message = %q, want the string_type message", errs[0].Message)
	}
}

// **A YAML word-form boolean against a script-declared `bool` option must not
// be pruned as a type conflict, and must reach the tree as an actual `bool`.**
// `show_footer: no` resolves to the raw string `"no"` (`ResolveScalar` only
// recognizes `true`/`false`), which pydantic's `str_as_bool` — and this port's
// own schema-validation layer — accept as a real boolean. Before this fix,
// `luatheme.kindOf` classified `"no"` as "a value" against the script's
// "true or false" and `withoutConflicts` pruned the whole override back out,
// leaving the script's own default; even past that, the raw string would have
// reached the Typst emitter as an unquoted, uncompilable token. Found by a
// fresh-context verifier (iteration 14's fifth re-verification).
func TestWordFormBooleanOverridesAScriptDeclaredBool(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { page = { show_footer = true } }`,
		"  page:\n    show_footer: no\n")

	if got := design.EffectiveBool(doc.Design, "page", "show_footer"); got {
		t.Errorf("page.show_footer = %v, want false (the document's word-form override) — got the script's true, "+
			"meaning the override was pruned as a type conflict or left as the unconverted string", got)
	}
}

// **The bool-word carve-out must not swallow a legal string that merely spells
// one.** A script-declared *string* option (`custom_note`, no base-tree shape
// to fall back on) against a document override of `'On'` — a quoted, legal
// YAML string — is not a bool-vs-string conflict at all; both sides are
// strings. An earlier fix made `kindOf` itself classify any bool-word string
// as kind "true or false" regardless of what the *other* side was, which made
// this exact case look like agreement and silently dropped the document's
// override. Found by a fresh-context verifier (iteration 14's sixth
// re-verification).
func TestABoolWordStringOverridesAScriptDeclaredStringOption(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme", `return { custom_note = "hello" }`,
		"  custom_note: 'On'\n")

	if got := design.EffectiveString(doc.Design, "custom_note"); got != "On" {
		t.Errorf("custom_note = %q, want the document's string override %q", got, "On")
	}
}

// **A script declaring `typography.font_family` as the five-element mapping
// must not be dropped whole.** `ValidateScript` used to flag the mapping
// shape against `KindFontFamily` (only `KindNested` fields are allowed a
// mapping) as a value-where-a-group conflict, and `themeScript` discards the
// **entire script** on any `ValidateScript` error — so a script setting
// `colors.name` alongside a mapping `font_family` lost the colour too, along
// with every other option, silently, at exit 0. Found by a fresh-context
// verifier (iteration 14's seventh re-verification).
func TestAScriptDeclaringFontFamilyAsAMappingDoesNotDropTheWholeScript(t *testing.T) {
	doc := resolveWithTheme(t, "mytheme",
		`return { colors = { name = "rgb(1, 2, 3)" }, typography = { font_family = { body = "Lato", name = "Lato" } } }`,
		"")

	if got := design.EffectiveString(doc.Design, "colors", "name"); got != "rgb(1, 2, 3)" {
		t.Errorf("colors.name = %q, want the script's colour — the font_family mapping must not have dropped the whole script", got)
	}
	if got := design.EffectiveString(doc.Design, "typography", "font_family", "body"); got != "Lato" {
		t.Errorf("font_family.body = %q, want the script's own mapping default", got)
	}
}
