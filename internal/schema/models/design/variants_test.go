package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// How many classes each nested model produces, measured against upstream's
// `schema.json` by counting its `$defs` keys.
//
// **The whole table is the point.** "Nine themes, nine classes" is the obvious
// wrong answer and it is right for only nine of the twenty models; the other
// eleven range from one to eight. Asserting the counts here rather than only
// through the schema differential means a change reports as one failure naming
// the model, not as many byte diffs naming nothing.
func TestModelCounts(t *testing.T) {
	want := map[string]int{
		// Every theme overrides these.
		"Typography": 9, "Header": 9, "SectionTitles": 9, "Sections": 9,
		"Entries": 9, "Templates": 9,
		"EducationEntry": 9, "NormalEntry": 9, "ExperienceEntry": 9,

		// Fewer, because a theme that omits the block reuses the base's.
		"Links": 8, "Highlights": 8,
		"Colors": 7, "Bold": 7, "FontSize": 7, "Connections": 7, "Summary": 7,
		"Page":      6,
		"SmallCaps": 4,

		// **No theme overrides these two**, so each is a single class whose
		// qualified name is already unique — and carries no suffix at all.
		"OneLineEntry": 1, "PublicationEntry": 1,
	}

	got := design.ModelCounts()
	if len(got) != len(want) {
		t.Fatalf("%d models numbered, want %d: %v", len(got), len(want), got)
	}
	for model, wanted := range want {
		if got[model] != wanted {
			t.Errorf("%s has %d classes, want %d", model, got[model], wanted)
		}
	}
}

// The two shapes a `$defs` key takes, and the trap in each.
func TestDefNameOf(t *testing.T) {
	const prefix = "rendercv__schema__models__design__classic_theme__"

	tests := []struct {
		name  string
		model string
		theme string
		want  string
	}{
		// A model only one theme produced carries **no** suffix. Suffixing
		// unconditionally gets 159 of 161 right.
		{"unsuffixed", "OneLineEntry", "classic", prefix + "OneLineEntry"},
		{"unsuffixed for a variant too", "PublicationEntry", "ink", prefix + "PublicationEntry"},

		// A theme that omits the block points back at the base's `__1` rather
		// than getting a number of its own — harvard names no `links`, sb2nov
		// names no `page`.
		{"harvard reuses the base links", "Links", "harvard", prefix + "Links__1"},
		{"sb2nov reuses the base page", "Page", "sb2nov", prefix + "Page__1"},

		// And a theme that does name it is numbered by its position among the
		// themes that do, not by its position in the union.
		{"ember is the second page", "Page", "ember", prefix + "Page__2"},
		{"engineeringresumes is the third", "Page", "engineeringresumes", prefix + "Page__3"},
		{"classic is always first", "Colors", "classic", prefix + "Colors__1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := design.DefNameOf(tc.model, tc.theme); got != tc.want {
				t.Errorf("DefNameOf(%q, %q) = %q, want %q", tc.model, tc.theme, got, tc.want)
			}
		})
	}
}

// A theme model's key is bare, and `engineeringclassic` capitalizes as one word
// because Python's `str.capitalize()` sees no underscore to split on.
func TestThemeDefName(t *testing.T) {
	tests := map[string]string{
		"classic":            "ClassicTheme",
		"ember":              "EmberTheme",
		"engineeringclassic": "EngineeringclassicTheme",
		"sb2nov":             "Sb2novTheme",
	}
	for theme, want := range tests {
		t.Run(theme, func(t *testing.T) {
			if got := design.ThemeDefName(theme); got != want {
				t.Errorf("= %q, want %q", got, want)
			}
		})
	}
}
