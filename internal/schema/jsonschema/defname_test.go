package jsonschema

import (
	"strings"
	"testing"
)

// Spec 005 §3.3 behaviors 11 and 12, on upstream's own two shapes.
func TestDefName(t *testing.T) {
	tests := []struct{ name, class, module, want string }{
		{
			name:  "a unique class name is bare",
			class: "Cv",
			want:  "Cv",
		},
		{
			name:  "so is a type alias",
			class: "ArbitraryDate",
			want:  "ArbitraryDate",
		},
		{
			// Upstream's own key, verbatim.
			name:   "a colliding name is qualified with its module path",
			class:  "EducationEntry",
			module: "rendercv.schema.models.cv.entries.education",
			want:   "rendercv__schema__models__cv__entries__education__EducationEntry",
		},
		{
			name:   "every dot becomes a double underscore",
			class:  "FontFamily",
			module: "rendercv.schema.models.design.font_family",
			want:   "rendercv__schema__models__design__font_family__FontFamily",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DefName(test.class, test.module); got != test.want {
				t.Errorf("DefName = %q, want %q", got, test.want)
			}
		})
	}
}

// Spec 005 §3.3 behaviors 12 and 13, on upstream's own locale keys.
func TestDefNameWithSuffix(t *testing.T) {
	const module = "rendercv.schema.models.locale.english_locale"

	got := DefNameWithSuffix("Phrases", module, 1)
	const want = "rendercv__schema__models__locale__english_locale__Phrases__1"
	if got != want {
		t.Errorf("=\n  %q\nwant\n  %q", got, want)
	}
}

// The numbering follows the **emission order** it is given, not the alphabet.
//
// That distinction is the whole difficulty and it is invisible in the output:
// `$defs` sorts its keys, so an alphabetically-assigned suffix produces a file
// that looks correctly sorted while pairing every model with the wrong number.
// The test therefore checks the pairing, on an order chosen so the two disagree.
func TestSuffixedNamesFollowsEmissionOrder(t *testing.T) {
	const module = "rendercv.schema.models.locale.english_locale"

	// Deliberately not alphabetical: `english` leads, as Languages does.
	emission := []string{"english", "arabic", "danish"}
	got := SuffixedNames("Phrases", module, emission)

	if len(got) != len(emission) {
		t.Fatalf("got %d names, want %d", len(got), len(emission))
	}
	for i, name := range got {
		want := DefNameWithSuffix("Phrases", module, i+1)
		if name != want {
			t.Errorf("name %d = %q, want %q", i, name, want)
		}
	}

	// `english` is first in emission order and third alphabetically, so a
	// sorted implementation would give it __3.
	if !strings.HasSuffix(got[0], "__1") {
		t.Errorf("the first emitted model got %q; emission order was ignored", got[0])
	}
}
