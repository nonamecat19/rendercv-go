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

// The collision suffix is iteration 6's, and asking for one is a panic rather
// than a guess: the numbering follows pydantic's emission order, so a plausible
// implementation would be a wrong answer that reads right.
func TestDefNameWithSuffixPanicsNamingIteration6(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("DefNameWithSuffix returned instead of panicking")
		}
		message, ok := recovered.(string)
		if !ok || !strings.Contains(message, "iteration 6") {
			t.Errorf("panic = %v, want it to name iteration 6", recovered)
		}
	}()

	DefNameWithSuffix("Colors", "rendercv.schema.models.design.classic_theme", 1)
}
