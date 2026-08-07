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
