package luatheme_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
)

// Spec 014 §4, criterion 2's remaining half: an option a script **adds** has no
// entry in the design tree, so nothing else in the port checks what a document
// puts in it.
func TestValidateAddedOptions(t *testing.T) {
	script := map[string]any{
		"my_block": map[string]any{"width": "1cm", "enabled": true},
	}

	cases := []struct {
		name     string
		document map[string]any
		errors   int
	}{
		{
			name:     "a matching document passes",
			document: map[string]any{"my_block": map[string]any{"width": "2cm"}},
		},
		{
			name:     "a group where a value belongs is caught",
			document: map[string]any{"my_block": map[string]any{"width": map[string]any{"a": "b"}}},
			errors:   1,
		},
		{
			name:     "a value where a group belongs is caught",
			document: map[string]any{"my_block": "oops"},
			errors:   1,
		},
		{
			// **Only what the script declared.** Everything the base tree owns
			// is iteration 6's business and is validated there.
			name:     "an option the script never declared is ignored",
			document: map[string]any{"colors": map[string]any{"name": "rgb(0, 0, 0)"}},
		},
		{
			name:     "a key the document omits is ignored",
			document: map[string]any{"my_block": map[string]any{"enabled": false}},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			errs := luatheme.Validate(script, row.document)
			if len(errs) != row.errors {
				t.Errorf("got %d errors %v, want %d", len(errs), errs, row.errors)
			}
		})
	}
}

// The message names the path the user wrote, not an internal one.
func TestTypeErrorMessage(t *testing.T) {
	errs := luatheme.Validate(
		map[string]any{"my_block": map[string]any{"width": "1cm"}},
		map[string]any{"my_block": map[string]any{"width": map[string]any{}}},
	)
	if len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
	want := "design.my_block.width should be a value, not a group of options"
	if errs[0].Error() != want {
		t.Errorf("= %q, want %q", errs[0].Error(), want)
	}
}
