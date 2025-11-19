package locale_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// Spec 007 §2 behaviors 5 and 7, verbatim.
func TestEnglishDefaults(t *testing.T) {
	english := locale.English()

	for name, got := range map[string]string{
		"language":     english.Language,
		"last_updated": english.LastUpdated,
		"month":        english.Month,
		"months":       english.Months,
		"year":         english.Year,
		"years":        english.Years,
		"present":      english.Present,
		"phrases":      english.DegreeWithArea,
	} {
		want := map[string]string{
			"language": "english", "last_updated": "Last updated in",
			"month": "month", "months": "months", "year": "year", "years": "years",
			"present": "present", "phrases": "DEGREE in AREA",
		}[name]
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// The abbreviations come from Yale's cataloguing table, not from truncation.
//
// Three of the twelve are four letters, and a port that sliced three characters
// off the full names would get exactly those three wrong — which is why this is
// asserted as the property rather than only as a list.
func TestMonthAbbreviationsAreNotTruncations(t *testing.T) {
	english := locale.English()

	want := []string{
		"Jan", "Feb", "Mar", "Apr", "May", "June",
		"July", "Aug", "Sept", "Oct", "Nov", "Dec",
	}
	if len(english.MonthAbbreviations) != len(want) {
		t.Fatalf("abbreviations = %v", english.MonthAbbreviations)
	}
	for i := range want {
		if english.MonthAbbreviations[i] != want[i] {
			t.Errorf("abbreviation %d = %q, want %q", i, english.MonthAbbreviations[i], want[i])
		}
	}

	// The three that break the three-letter rule, named so a regression says why.
	for _, index := range []int{5, 6, 8} {
		if len(english.MonthAbbreviations[index]) == 3 {
			t.Errorf("abbreviation %d is %q; upstream's is four letters",
				index, english.MonthAbbreviations[index])
		}
	}
}

// Spec 007 §3 behavior 10: two bounds, two messages, two codes, and the count
// interpolated in both.
func TestMonthListLength(t *testing.T) {
	tests := []struct {
		name  string
		items int
		code  string
		want  string
	}{
		{
			name: "eleven is too short", items: 11, code: "too_short",
			want: "List should have at least 12 items after validation, not 11",
		},
		{
			name: "thirteen is too long", items: 13, code: "too_long",
			want: "List should have at most 12 items after validation, not 13",
		},
		{
			name: "none is too short, with its own count", items: 0, code: "too_short",
			want: "List should have at least 12 items after validation, not 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var src strings.Builder
			src.WriteString("month_abbreviations:\n")
			if test.items == 0 {
				src.Reset()
				src.WriteString("month_abbreviations: []\n")
			}
			for i := 0; i < test.items; i++ {
				src.WriteString("  - x\n")
			}

			node, err := yamlreader.ReadString(src.String())
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}

			errs := locale.ValidateCatalog(node, "english", []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if string(errs[0].Code) != test.code {
				t.Errorf("code = %q, want %q", errs[0].Code, test.code)
			}
			if errs[0].Message != test.want {
				t.Errorf("message = %q, want %q", errs[0].Message, test.want)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "locale.month_abbreviations" {
				t.Errorf("location = %q", got)
			}

			// No dictionary row matches, so the pipeline only appends a period.
			// No document: the fixture is the locale block itself rather than a
			// whole file, so there is nothing to resolve coordinates against.
			final, err := errorpipeline.Parse(errs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if final[0].Message != test.want+"." {
				t.Errorf("final = %q, want the raw text plus a period", final[0].Message)
			}
		})
	}
}

// Exactly twelve passes, which is what keeps the test above from passing for
// the wrong reason.
func TestTwelveMonthsAccepted(t *testing.T) {
	var src strings.Builder
	src.WriteString("month_abbreviations:\n")
	for i := 0; i < 12; i++ {
		src.WriteString("  - x\n")
	}
	node, err := yamlreader.ReadString(src.String())
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if errs := locale.ValidateCatalog(node, "english", []string{"locale"}, schemaerr.SourceMain); len(errs) != 0 {
		t.Errorf("errs = %+v, want none", errs)
	}
}

// Every locale model rejects unknown keys (spec 007 §3 behavior 8).
func TestCatalogRejectsUnknownKeys(t *testing.T) {
	node, err := yamlreader.ReadString("language: english\nnope: 1\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	errs := locale.ValidateCatalog(node, "english", []string{"locale"}, schemaerr.SourceMain)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Message != "Extra inputs are not permitted" {
		t.Errorf("message = %q", errs[0].Message)
	}
}

// The twelve-element bound is EnglishLocale's alone.
//
// Measured against the vendored Python: `{language: danish, month_names: [11
// items]}` validates and the same document with `english` does not. The bound
// lives in the `Annotated` metadata that `create_simple_field_spec` strips when
// it rebuilds each variant's field, so the twenty-one variants have no length
// rule at all — in validation as in their schema.
//
// A port that applied the bound to every member rejects documents upstream
// accepts, and no English fixture can see it.
func TestMonthBoundAppliesToEnglishOnly(t *testing.T) {
	var src strings.Builder
	src.WriteString("month_names:\n")
	for i := 0; i < 11; i++ {
		src.WriteString("  - x\n")
	}
	node, err := yamlreader.ReadString(src.String())
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	for _, language := range locale.Languages {
		t.Run(language, func(t *testing.T) {
			errs := locale.ValidateCatalog(node, language, []string{"locale"}, schemaerr.SourceMain)
			want := 0
			if language == "english" {
				want = 1
			}
			if len(errs) != want {
				t.Errorf("%d errors, want %d: %+v", len(errs), want, errs)
			}
		})
	}
}

// `phrases` is a nested model, so a non-mapping is a `model_type` failure that
// names it, and an explicit null is one too: the field is `Phrases` with a
// default, not `Phrases | None`.
//
// Measured on 5, a sequence, a string, a tagged scalar and null — 1411 bytes at
// exit 1 each, where the port accepted every one of them and rendered the CV.
// The name is `Phrases` for every language, measured on english, danish and
// turkish.
func TestPhrasesMustBeAMapping(t *testing.T) {
	const want = "Input should be a valid dictionary or instance of Phrases"

	for _, src := range []string{"5", "[a]", "abc", "!!str x", "null"} {
		t.Run(src, func(t *testing.T) {
			node, err := yamlreader.ReadString("language: english\nphrases: " + src + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}

			errs := locale.ValidateCatalog(node, "english", []string{"locale"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != want {
				t.Errorf("message = %q, want %q", errs[0].Message, want)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "locale.phrases" {
				t.Errorf("location = %q", got)
			}
		})
	}
}

// The failure is emitted at `phrases`'s declared position, before the two month
// lists — which an appended check cannot do. Measured on
// `{phrases: 5, month_names: 5}`, 1834 bytes on both sides.
func TestPhrasesFailsInDeclarationOrder(t *testing.T) {
	node, err := yamlreader.ReadString("language: english\nphrases: 5\nmonth_names: 5\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}

	errs := locale.ValidateCatalog(node, "english", []string{"locale"}, schemaerr.SourceMain)
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want two", errs)
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "locale.phrases" {
		t.Errorf("first failure is %q, want locale.phrases", got)
	}
}

// A mapping still passes, so the shape check did not close the field.
func TestPhrasesAcceptsAMapping(t *testing.T) {
	node, err := yamlreader.ReadString(
		"language: english\nphrases:\n  degree_with_area: \"X in Y\"\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if errs := locale.ValidateCatalog(
		node, "english", []string{"locale"}, schemaerr.SourceMain,
	); len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
}
