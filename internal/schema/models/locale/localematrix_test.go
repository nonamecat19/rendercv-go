package locale_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/pkg/rendercv"
)

// matrixFixture is the CV every locale test renders.
const matrixFixture = "testdata/locale_matrix.yaml"

// renderForLocale renders the fixture under one language and returns the three
// artifacts, keyed the way the Python driver keys them.
func renderForLocale(t *testing.T, language string) map[string]string {
	t.Helper()

	fixture, err := os.ReadFile(matrixFixture)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "CV.yaml")
	document := strings.Replace(string(fixture), "cv:\n",
		"locale:\n  language: "+language+"\ncv:\n", 1)
	if err := os.WriteFile(input, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}

	_, model, err := rendercv.Build(document, rendercv.BuildOptions{
		InputFilePath:   input,
		DontGeneratePDF: true,
		DontGeneratePNG: true,
	})
	if err != nil {
		t.Fatalf("%s: %v", language, err)
	}

	typstPath, err := rendercv.GenerateTypst(model)
	if err != nil {
		t.Fatalf("%s: %v", language, err)
	}
	markdownPath, err := rendercv.GenerateMarkdown(model)
	if err != nil {
		t.Fatalf("%s: %v", language, err)
	}
	htmlPath, err := rendercv.GenerateHTML(model, markdownPath)
	if err != nil {
		t.Fatalf("%s: %v", language, err)
	}

	out := map[string]string{}
	for key, path := range map[string]string{
		"typst": typstPath, "markdown": markdownPath, "html": htmlPath,
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", language, err)
		}
		out[key] = string(content)
	}
	return out
}

// Every string a catalog carries has to reach the rendered document, or a
// locale test cannot fail.
//
// **This is the gap that made "22/22 locales byte-identical" mean less than it
// sounded.** That claim was measured on the catalog data and on the starter CV
// the `new` command writes — neither of which renders a month. A locale is only
// verified end to end if a mistranslated `month_names[7]` would change an
// artifact, and it only would if something printed August.
//
// So the fixture is checked for reachability before it is trusted for parity:
// all six phrases, all twelve month names, all twelve abbreviations, and
// `degree_with_area` in its substituted form. Thirty-one strings per locale,
// 651 across the twenty-one catalogs.
//
// English is exempt because it has no catalog file — its values are Python
// defaults (`english_locale.py`) and `Catalogs` builds it by hand.
func TestEveryCatalogStringReachesTheDocument(t *testing.T) {
	catalogs := locale.Catalogs()

	for _, language := range locale.AvailableLocales() {
		if language == "english" {
			continue
		}
		t.Run(language, func(t *testing.T) {
			catalog := catalogs[language]
			typst := renderForLocale(t, language)["typst"]

			// `degree_with_area` is a template, so the rendered form is what
			// has to be present — the raw `DEGREE in AREA` never appears.
			degree := strings.NewReplacer(
				"DEGREE", "PhD", "AREA", "Computer Science").Replace(catalog.DegreeWithArea)

			checks := map[string]string{
				"last_updated":     catalog.LastUpdated,
				"month":            catalog.Month,
				"months":           catalog.Months,
				"year":             catalog.Year,
				"years":            catalog.Years,
				"present":          catalog.Present,
				"degree_with_area": degree,
			}
			for i, name := range catalog.MonthNames {
				checks["month_names["+strconv.Itoa(i)+"]"] = name
			}
			for i, abbreviation := range catalog.MonthAbbreviations {
				checks["month_abbreviations["+strconv.Itoa(i)+"]"] = abbreviation
			}

			for field, want := range checks {
				if !strings.Contains(typst, want) {
					t.Errorf("%s = %q never reaches the rendered .typ —"+
						" the fixture stopped exercising it", field, want)
				}
			}
		})
	}
}

// Two locales that rendered the same document would make the differential
// vacuous for both, and a fixture that stopped carrying the `locale:` block at
// all would make it vacuous for every one.
func TestEveryLocaleRendersADistinctDocument(t *testing.T) {
	seen := map[string]string{}
	for _, language := range locale.AvailableLocales() {
		typst := renderForLocale(t, language)["typst"]
		if other, clash := seen[typst]; clash {
			t.Errorf("%s and %s render byte-identical documents", other, language)
		}
		seen[typst] = language
	}
	if len(seen) != len(locale.AvailableLocales()) {
		t.Errorf("%d distinct documents for %d locales",
			len(seen), len(locale.AvailableLocales()))
	}
}
