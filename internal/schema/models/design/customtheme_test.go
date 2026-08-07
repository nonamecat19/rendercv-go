package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// Spec 014 §1 behavior 4: **a custom theme with no script is valid**, and falls
// back to `ClassicTheme` with only its `theme` field set
// (`design.py:137-142`).
//
// **This is the path that can regress what already works**, which is why spec
// 014 orders it first: every one of the nine built-in themes and all 24 corpus
// documents reach `Effective` the same way, so a change made for scripted themes
// breaks the unscripted ones before it helps anyone.
//
// It needs no new code — iteration 6's `Effective` already produces it — and
// this test exists to say so in a way that stays true, rather than to be
// asserted in a spec and discovered wrong later.
func TestCustomThemeWithoutAScriptIsClassic(t *testing.T) {
	classic := design.Effective("classic", nil)
	custom := design.Effective("mytheme", nil)

	if custom["theme"] != "mytheme" {
		t.Errorf("theme = %v, want the custom name", custom["theme"])
	}

	// Every other value is ClassicTheme's, so the two trees differ in exactly
	// one key. Comparing a few load-bearing ones by path rather than the whole
	// map keeps the failure readable.
	for _, path := range [][]string{
		{"colors", "name"},
		{"typography", "font_size", "body"},
		{"page", "size"},
		{"templates", "single_date"},
		{"templates", "education_entry", "main_column"},
	} {
		if got, want := design.EffectiveString(custom, path...),
			design.EffectiveString(classic, path...); got != want {
			t.Errorf("%v = %q, want classic's %q", path, got, want)
		}
	}
}

// A named built-in theme still gets its overrides, so the fallback above cannot
// be swallowing them.
func TestABuiltinThemeStillOverrides(t *testing.T) {
	classic := design.EffectiveString(design.Effective("classic", nil), "colors", "name")
	sb2nov := design.EffectiveString(design.Effective("sb2nov", nil), "colors", "name")

	if classic == sb2nov {
		t.Errorf("sb2nov's name colour = %q, same as classic's — the override was lost", sb2nov)
	}
}

// Spec 014 §4, criterion 3: a script **changes a default**, and a document
// override still wins.
//
// **The layer order is the whole question.** A script's declarations are
// defaults, so they must sit above the theme's and below the document's. Put
// them above the document and a user could not override their own theme; put
// them below the theme and a custom theme could not change anything.
func TestAScriptsDefaultLosesToTheDocument(t *testing.T) {
	script := map[string]any{"colors": map[string]any{"name": "rgb(1, 2, 3)"}}

	// With no document, the script's value is what the renderer sees.
	withScript := design.EffectiveWithScript("mytheme", script, nil)
	if got := design.EffectiveString(withScript, "colors", "name"); got != "rgb(1, 2, 3)" {
		t.Errorf("colors.name = %q, want the script's default", got)
	}

	// The document overrides it, and the script's *other* values survive —
	// the merge is deep at this layer too.
	document := map[string]any{"colors": map[string]any{"name": "rgb(9, 9, 9)"}}
	overridden := design.EffectiveWithScript("mytheme", script, document)
	if got := design.EffectiveString(overridden, "colors", "name"); got != "rgb(9, 9, 9)" {
		t.Errorf("colors.name = %q, want the document's value", got)
	}
	if got := design.EffectiveString(overridden, "colors", "body"); got == "" {
		t.Errorf("colors.body was lost when the script layer merged")
	}
}

// A nil script is the built-in case: nothing changes for the nine themes that
// have no script at all.
func TestANilScriptChangesNothing(t *testing.T) {
	for _, theme := range []string{"classic", "sb2nov", "moderncv"} {
		plain := design.Effective(theme, nil)
		scripted := design.EffectiveWithScript(theme, nil, nil)
		if design.EffectiveString(plain, "colors", "name") !=
			design.EffectiveString(scripted, "colors", "name") {
			t.Errorf("%s changed when a nil script was merged", theme)
		}
	}
}
