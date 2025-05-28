//go:build conformance

package locale_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
)

const overridesDir = "../../../../third_party/rendercv/src/rendercv/schema/models/locale/other_locales"

// upstreamCatalog is one `other_locales/*.yaml` file.
type upstreamCatalog struct {
	Locale struct {
		Language           string   `yaml:"language"`
		LastUpdated        string   `yaml:"last_updated"`
		Month              string   `yaml:"month"`
		Months             string   `yaml:"months"`
		Year               string   `yaml:"year"`
		Years              string   `yaml:"years"`
		Present            string   `yaml:"present"`
		MonthAbbreviations []string `yaml:"month_abbreviations"`
		MonthNames         []string `yaml:"month_names"`
		Phrases            struct {
			DegreeWithArea string `yaml:"degree_with_area"`
		} `yaml:"phrases"`
	} `yaml:"locale"`
}

// Every catalog, field by field, against the submodule.
//
// Nothing else stands between the port and a mistranslated month. The data is
// compiled-in Go source because the submodule is absent at runtime, and this is
// the only thing that makes the copy trustworthy — the same arrangement, and the
// same reasoning, as the error dictionary.
//
// **Every field is compared, and that is meaningful here** rather than merely
// thorough: upstream generates these variants with `require_all_fields=True`, so
// each YAML file really does carry all ten. A field absent from the Go data
// cannot hide behind an inherited English default.
func TestCatalogsMatchTheSubmodule(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(overridesDir, "*.yaml"))
	if err != nil {
		t.Fatalf("globbing %s: %v", overridesDir, err)
	}
	if len(files) != 21 {
		t.Fatalf("found %d override files, want 21 — is the submodule initialized?", len(files))
	}

	catalogs := locale.Catalogs()

	for _, path := range files {
		language := strings.TrimSuffix(filepath.Base(path), ".yaml")
		t.Run(language, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var upstream upstreamCatalog
			if err := yaml.Unmarshal(raw, &upstream); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			want := upstream.Locale

			got, ok := catalogs[language]
			if !ok {
				t.Fatalf("the port has no %s catalog", language)
			}

			for _, field := range []struct{ name, got, want string }{
				{"language", got.Language, want.Language},
				{"last_updated", got.LastUpdated, want.LastUpdated},
				{"month", got.Month, want.Month},
				{"months", got.Months, want.Months},
				{"year", got.Year, want.Year},
				{"years", got.Years, want.Years},
				{"present", got.Present, want.Present},
				{"phrases.degree_with_area", got.DegreeWithArea, want.Phrases.DegreeWithArea},
			} {
				if field.got != field.want {
					t.Errorf("%s = %q, want %q", field.name, field.got, field.want)
				}
			}

			assertList(t, "month_abbreviations", got.MonthAbbreviations, want.MonthAbbreviations)
			assertList(t, "month_names", got.MonthNames, want.MonthNames)
		})
	}
}

// The language set matches the list iteration 4 already ships for spec 004
// §4.30's message. One list, two consumers, so a twenty-third language is one
// edit rather than two places that can disagree.
func TestCatalogsCoverEveryLanguage(t *testing.T) {
	catalogs := locale.Catalogs()

	for _, language := range locale.Languages {
		if _, ok := catalogs[language]; !ok {
			t.Errorf("no catalog for %q, which Languages lists", language)
		}
	}
	for language := range catalogs {
		if !contains(locale.Languages, language) {
			t.Errorf("catalog %q is not in Languages", language)
		}
	}
}

func assertList(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s has %d entries, want %d", name, len(got), len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// `Languages` is in **upstream's union order**, not merely upstream's set
// (spec 007 §5 criterion 2).
//
// `discover_other_locales` globs `other_locales/` and sorts by filename
// (locale.py:26-38), and `Locale` puts `EnglishLocale` ahead of the reduction
// (locale.py:43-46). So the order is english, then the stems sorted.
//
// It is asserted here rather than left to the `$defs` differential because the
// order decides the `Phrases__N` numbering: a submodule bump that reordered the
// union would surface as forty-five byte failures naming no cause.
func TestLanguagesAreInUnionOrder(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(overridesDir, "*.yaml"))
	if err != nil {
		t.Fatalf("globbing %s: %v", overridesDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no override files under %s — is the submodule initialized? `just setup`", overridesDir)
	}
	sort.Strings(files)

	want := []string{"english"}
	for _, file := range files {
		want = append(want, strings.TrimSuffix(filepath.Base(file), ".yaml"))
	}

	assertList(t, "Languages", locale.Languages, want)
}
