package design_test

import (
	"sort"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// `available_font_families` is `sorted()`, and the sort is over the display
// names — so `EB Garamond` precedes `Fontin` and `Open Sans` precedes
// `Open Sauce Sans`, which a port sorting case-insensitively or by the source
// grouping would get wrong.
func TestFontFamiliesAreSorted(t *testing.T) {
	if len(design.FontFamilies) != 17 {
		t.Fatalf("%d families, want 17", len(design.FontFamilies))
	}
	if !sort.StringsAreSorted(design.FontFamilies) {
		t.Errorf("not sorted: %q", design.FontFamilies)
	}
	// The three Typst built-ins are first in the source and scattered here,
	// which is the whole point of asserting the order.
	if design.FontFamilies[0] != "DejaVu Sans Mono" {
		t.Errorf("first = %q, want %q", design.FontFamilies[0], "DejaVu Sans Mono")
	}
	if design.FontFamilies[16] != "XCharter" {
		t.Errorf("last = %q, want %q", design.FontFamilies[16], "XCharter")
	}
}

// Any font name validates: the union's string arm is hidden from the schema but
// not from validation (spec 006 §3.1 behavior 12). Measured — a design naming
// `NoSuchFont` is accepted by upstream.
func TestAnyFontNameValidates(t *testing.T) {
	for _, name := range []string{"NoSuchFont", "", "Source Sans 3", "Comic Sans MS"} {
		if !design.ValidFontFamily(name) {
			t.Errorf("ValidFontFamily(%q) = false, want true", name)
		}
	}
	if design.IsKnownFontFamily("NoSuchFont") {
		t.Error("NoSuchFont is not one of the seventeen")
	}
	if !design.IsKnownFontFamily("Source Sans 3") {
		t.Error("Source Sans 3 is one of the seventeen")
	}
}

// A bare string widens to every element (spec 006 §3.2 behavior 14).
func TestWidenFontFamily(t *testing.T) {
	widened := design.WidenFontFamily("Roboto")
	if len(widened) != len(design.FontFamilyElements) {
		t.Fatalf("%d elements, want %d", len(widened), len(design.FontFamilyElements))
	}
	for _, element := range design.FontFamilyElements {
		if widened[element] != "Roboto" {
			t.Errorf("%s = %q, want %q", element, widened[element], "Roboto")
		}
	}
}
