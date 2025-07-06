package locale_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func resolve(t *testing.T, document string) locale.Catalog {
	t.Helper()
	if document == "" {
		return locale.Resolve(nil)
	}
	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}
	return locale.Resolve(node)
}

// No block at all is English, which is the discriminator's own default.
func TestResolveDefaultsToEnglish(t *testing.T) {
	got := resolve(t, "")
	if got.Language != "english" || got.Present != locale.English().Present {
		t.Errorf("= %+v, want the English catalog", got)
	}
}

// A named language brings its whole catalog, month lists included.
func TestANamedLanguageBringsItsCatalog(t *testing.T) {
	got := resolve(t, "language: french\n")

	if got.Language != "french" {
		t.Fatalf("language = %q, want french", got.Language)
	}
	if got.Present == locale.English().Present {
		t.Errorf("present = %q, want the French translation", got.Present)
	}
	if len(got.MonthNames) != 12 {
		t.Errorf("month names = %v, want twelve", got.MonthNames)
	}
}

// A block's own keys override the named language's, one field at a time — the
// "french, but" case.
func TestABlockOverridesOneFieldAtATime(t *testing.T) {
	got := resolve(t, "language: french\npresent: maintenant\n")

	if got.Present != "maintenant" {
		t.Errorf("present = %q, want the override", got.Present)
	}
	if got.Language != "french" {
		t.Errorf("language = %q, want french", got.Language)
	}
	// The rest of the French catalog survives the override.
	if got.MonthNames[0] == locale.English().MonthNames[0] {
		t.Errorf("month names = %v, want French ones", got.MonthNames)
	}
}

// `phrases` is a nested model with one member, and overriding it replaces only
// that member.
func TestPhrasesOverride(t *testing.T) {
	got := resolve(t, "phrases:\n  degree_with_area: DEGREE en AREA\n")
	if got.DegreeWithArea != "DEGREE en AREA" {
		t.Errorf("degree_with_area = %q, want the override", got.DegreeWithArea)
	}
}

// A key written with no value keeps the catalog's, rather than blanking it.
func TestNullValuedKeysKeepTheCatalog(t *testing.T) {
	got := resolve(t, "present:\nmonth_names:\n")
	if got.Present != locale.English().Present {
		t.Errorf("present = %q, want English's", got.Present)
	}
	if len(got.MonthNames) != 12 {
		t.Errorf("month names = %v, want twelve", got.MonthNames)
	}
}
