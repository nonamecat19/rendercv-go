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
