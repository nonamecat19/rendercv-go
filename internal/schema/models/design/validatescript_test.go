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

// **Shape is not type.** Every check above asks only whether a script declared
// a map, a list or a scalar where the tree wanted one — never whether the
// scalar it declared is a value the field can hold. Upstream does ask:
// `BaseModelWithoutExtraKeys` sets `validate_default=True` (`base.py:5`) and
// `design.py:135` constructs `theme_data_model_class(**design)`, so a script
// with a bad default is exit 1 with a validation panel. In this port each of
// these used to pass `ValidateScript` clean and reach the artifact —
// `page-size: "bogus"`, `colors-name: True` (which kills the Typst engine),
// and a document silently typeset in a font called "True". Found by a
// fresh-context verifier (iteration 14's twenty-first re-verification).
func TestValidateScriptChecksDeclaredValues(t *testing.T) {
	for _, test := range []struct {
		name   string
		script map[string]any
		want   string
	}{
		{
			name:   "a literal outside its members",
			script: map[string]any{"page": map[string]any{"size": "bogus"}},
			want: "design.page.size is invalid in this theme's script: " +
				"Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
		},
		{
			name:   "a bool where a colour belongs",
			script: map[string]any{"colors": map[string]any{"name": true}},
			want: "design.colors.name is invalid in this theme's script: " +
				"value is not a valid color: value must be a tuple, list or string",
		},
		{
			name:   "a number where a colour belongs",
			script: map[string]any{"colors": map[string]any{"name": 42}},
			want: "design.colors.name is invalid in this theme's script: " +
				"value is not a valid color: value must be a tuple, list or string",
		},
		{
			name:   "an unrecognized colour name",
			script: map[string]any{"colors": map[string]any{"name": "nosuchcolor"}},
			want: "design.colors.name is invalid in this theme's script: " +
				"value is not a valid color: string not recognised as a valid color",
		},
		{
			name: "a bool where a font name belongs",
			script: map[string]any{
				"typography": map[string]any{"font_family": map[string]any{"body": true}},
			},
			want: "design.typography.font_family.body is invalid in this theme's script: " +
				"Input should be a valid string",
		},
		{
			name:   "a bool where a dimension belongs",
			script: map[string]any{"page": map[string]any{"top_margin": true}},
			want: "design.page.top_margin is invalid in this theme's script: " +
				"Input should be a valid string",
		},
		{
			name:   "a number where a dimension belongs",
			script: map[string]any{"page": map[string]any{"top_margin": 2}},
			want: "design.page.top_margin is invalid in this theme's script: " +
				"Input should be a valid string",
		},
		{
			name:   "a dimension with a space before its unit",
			script: map[string]any{"page": map[string]any{"top_margin": "2 cm"}},
			want: "design.page.top_margin is invalid in this theme's script: " +
				design.MessageBadTypstDimension,
		},
		{
			name:   "a word a bool cannot be read as",
			script: map[string]any{"page": map[string]any{"show_footer": "nope"}},
			want: "design.page.show_footer is invalid in this theme's script: " +
				"Input should be a valid boolean, unable to interpret input",
		},
		{
			// **A whole number that is not 0 or 1 is bool_parsing**, the same
			// rule `validBoolNode` follows on a document — pydantic narrows it
			// to an int and fails reading it as a bool. This arm reported the
			// `bool_type` message instead. Found by a fresh-context verifier
			// (iteration 14's twenty-second re-verification).
			name:   "a number a bool cannot be read as",
			script: map[string]any{"page": map[string]any{"show_footer": 2}},
			want: "design.page.show_footer is invalid in this theme's script: " +
				"Input should be a valid boolean, unable to interpret input",
		},
		{
			// A Lua number only stays a `float64` when it is fractional
			// (`luatheme.convert`), and a fractional value is the one case that
			// really is `bool_type`.
			name:   "a fractional number where a bool belongs",
			script: map[string]any{"page": map[string]any{"show_footer": 1.5}},
			want: "design.page.show_footer is invalid in this theme's script: " +
				"Input should be a valid boolean",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			errs := design.ValidateScript(test.script)
			if len(errs) != 1 {
				t.Fatalf("got %d errors %v, want 1", len(errs), errs)
			}
			if errs[0].Error() != test.want {
				t.Errorf("= %q, want %q", errs[0].Error(), test.want)
			}
		})
	}
}

// The values a field really does accept must still pass, or the fix above
// would drop every legitimate script instead — including the coercions
// `normalizeBools` and `widenFontFamilyIn` exist to perform.
func TestValidateScriptAllowsValidDeclaredValues(t *testing.T) {
	errs := design.ValidateScript(map[string]any{
		"page": map[string]any{
			"size": "a5", "top_margin": "2cm", "show_footer": true,
			"show_top_note": "no",
		},
		"colors":     map[string]any{"name": "rgb(1, 2, 3)", "connections": "Black"},
		"typography": map[string]any{"font_family": map[string]any{"body": "Charter"}},
	})
	if len(errs) != 0 {
		t.Errorf("= %v, want none", errs)
	}
}
