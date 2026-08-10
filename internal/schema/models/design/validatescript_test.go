package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// The verifier's blocker 2: a script declaring a group where the tree wants a
// value produced `page-size: "<map[string]interface {} Value>"` in the artifact,
// at exit 0. Nothing caught it because the validator that would have was never
// called.
func TestValidateScriptCatchesShapeConflicts(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"page":   map[string]any{"size": map[string]any{"a": 1}},
		"colors": map[string]any{"name": map[string]any{"r": 1}},
	})
	if len(errs) != 2 {
		t.Fatalf("got %d errors %v, want 2", len(errs), errs)
	}
	want := "design.page.size is a group of options in this theme's script, but should be a value"
	if errs[0].Error() != want && errs[1].Error() != want {
		t.Errorf("messages = %v, want one naming design.page.size", errs)
	}
}

// A value where the tree declares a group is the same mistake inverted.
func TestValidateScriptCatchesAValueForAGroup(t *testing.T) {
	errs := design.ValidateScript(map[string]any{"page": "oops"})
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
}

// A correct script and an invented option both pass: the tree has no shape for
// an option it does not declare.
func TestValidateScriptAllowsCorrectAndInventedOptions(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"page":     map[string]any{"size": "a5"},
		"my_thing": map[string]any{"width": "1cm"},
	})
	if len(errs) != 0 {
		t.Errorf("= %v, want none", errs)
	}
}

// A list where a scalar belongs (the ninth re-verification's second finding)
// used to fall through every check: not `KindNested`, not the `font_family`
// carve-out, and — before this fix — not checked against a list shape either.
func TestValidateScriptCatchesAListForAScalar(t *testing.T) {
	errs := design.ValidateScript(map[string]any{"page": map[string]any{"size": []string{"a4"}}})
	if len(errs) != 1 {
		t.Fatalf("got %v, want 1 error", errs)
	}
	want := "design.page.size is a list of items in this theme's script, but should be a value"
	if errs[0].Error() != want {
		t.Errorf("= %q, want %q", errs[0].Error(), want)
	}
}

// A script declaring `typography.font_family` as a mapping used to be waved
// through with no check on what was *inside* it — the eighth re-verification's
// carve-out only checked the outer shape. A nested table one field too deep
// must still be caught.
func TestValidateScriptCatchesAFontFamilyElementNestedTooDeep(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"typography": map[string]any{
			"font_family": map[string]any{"body": map[string]any{"x": 1}},
		},
	})
	if len(errs) != 1 {
		t.Fatalf("got %v, want 1 error", errs)
	}
	want := "design.typography.font_family.body is a group of options in this theme's script, but should be a value"
	if errs[0].Error() != want {
		t.Errorf("= %q, want %q", errs[0].Error(), want)
	}
}

// A script declaring `typography.font_family` as a mapping with legal string
// elements must still pass, alongside an unrelated option — the whole point
// of the eighth re-verification's fix.
func TestValidateScriptAllowsAFontFamilyMappingWithSiblingOptions(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"typography": map[string]any{"font_family": map[string]any{"body": "Charter"}},
		"page":       map[string]any{"size": "a5"},
	})
	if len(errs) != 0 {
		t.Errorf("= %v, want none", errs)
	}
}

// An empty Lua table (`{}`, literally what `create-theme` writes for a field
// it doesn't set) becomes an empty Go map, not an empty `[]string` — Lua has
// no way to distinguish an empty sequence from an empty mapping. The tree's
// one `KindStringList` field must accept that ambiguous shape rather than
// having `ValidateScript` flag it as a conflict and `themeScript` drop the
// whole script over an option the script never meant to touch. The ninth
// re-verification's first finding.
func TestValidateScriptAllowsAnEmptyTableForAStringList(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"sections": map[string]any{"show_time_spans_in": map[string]any{}},
		"page":     map[string]any{"size": "a5"},
	})
	if len(errs) != 0 {
		t.Errorf("= %v, want none", errs)
	}
}

// A *non-empty* mapping for a `KindStringList` field is still a real conflict
// — only the empty-table ambiguity gets a pass.
func TestValidateScriptCatchesANonEmptyMappingForAStringList(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"sections": map[string]any{"show_time_spans_in": map[string]any{"a": "b"}},
	})
	if len(errs) != 1 {
		t.Fatalf("got %v, want 1 error", errs)
	}
	want := "design.sections.show_time_spans_in is a group of options in this theme's script, but should be a list of items"
	if errs[0].Error() != want {
		t.Errorf("= %q, want %q", errs[0].Error(), want)
	}
}

// A valid colour tuple passes; an invalid one must still be flagged, not
// waved through as any list shape — a shape-only carve-out would leave an
// unparseable tuple to reach the artifact as a raw Go slice, the exact
// failure the carve-out family exists to prevent. Found by a fresh-context
// verifier (iteration 14's fourteenth re-verification).
func TestValidateScriptColorTuple(t *testing.T) {
	if errs := design.ValidateScript(map[string]any{
		"colors": map[string]any{"name": []string{"1", "2", "3"}},
	}); len(errs) != 0 {
		t.Errorf("valid tuple: = %v, want none", errs)
	}

	errs := design.ValidateScript(map[string]any{
		"colors": map[string]any{"name": []string{"300", "2", "3"}},
	})
	if len(errs) != 1 {
		t.Fatalf("invalid tuple: got %v, want 1 error", errs)
	}
}
