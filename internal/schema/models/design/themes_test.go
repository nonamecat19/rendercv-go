package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// One list, two consumers. `BuiltInThemes` is iteration 4's, written by hand for
// spec 004 §4.27's message; `Themes()` is derived from the override files. A
// ninth theme has to move both, and this is what says so.
func TestThemesMatchBuiltInThemes(t *testing.T) {
	got := design.Themes()
	if len(got) != len(design.BuiltInThemes) {
		t.Fatalf("Themes() = %q, BuiltInThemes = %q", got, design.BuiltInThemes)
	}
	for i := range got {
		if got[i] != design.BuiltInThemes[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], design.BuiltInThemes[i])
		}
	}
}

// `classic` is first and has no override file — it is the base the other eight
// are generated from.
func TestClassicIsFirstAndHasNoOverrides(t *testing.T) {
	if design.Themes()[0] != "classic" {
		t.Errorf("first theme = %q, want classic", design.Themes()[0])
	}
	if len(design.Overrides("classic")) != 0 {
		t.Errorf("classic has %d overrides, want none", len(design.Overrides("classic")))
	}
	for _, theme := range design.Themes()[1:] {
		if len(design.Overrides(theme)) == 0 {
			t.Errorf("%s has no overrides", theme)
		}
	}
}

// Which keys a theme names decides the `$defs` collision numbering, so the
// counts are pinned here rather than only inside the schema differential — where
// a change would report as many byte failures naming no cause.
//
// Measured: `1 + the number of override files naming the key` is the number of
// classes for that model (plan §4).
func TestOverrideKeyCounts(t *testing.T) {
	counts := map[string]int{}
	for _, theme := range design.Themes()[1:] {
		for key := range design.Overrides(theme) {
			if key != "theme" {
				counts[key]++
			}
		}
	}

	want := map[string]int{
		"page":           5,
		"colors":         6,
		"typography":     8,
		"links":          7,
		"header":         8,
		"section_titles": 8,
		"sections":       8,
		"entries":        8,
		"templates":      8,
	}
	if len(counts) != len(want) {
		t.Fatalf("keys = %v, want %v", counts, want)
	}
	for key, wanted := range want {
		if counts[key] != wanted {
			t.Errorf("%s named by %d themes, want %d", key, counts[key], wanted)
		}
	}
}
