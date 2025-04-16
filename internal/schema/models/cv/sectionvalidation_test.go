package cv_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// sectionReference is the date `present` resolves to in these tests. Fixed
// rather than time.Now(), so a section carrying `end_date: present` is
// reproducible (spec 002 §3.73 case 5).
var sectionReference = time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC)

// fixtureRegistry is the real registry (spec §3.56, §7.1). Iteration 2 stood in
// a hand-written one here because the concrete entry types did not exist yet;
// iteration 3 T17 replaced it with entries.Default(), and every test below
// passes unchanged — which is the point of the swap.
func fixtureRegistry() *entries.Registry {
	return entries.Default()
}

func section(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	return parse(t, "section:\n"+src).Items[0].Value
}

// Spec §3.53, §4.8 — a section value must be a list.
func TestSectionMustBeAList(t *testing.T) {
	for _, src := range []string{"  a: 1\n", "  just text\n"} {
		_, errs := cv.ValidateSection(section(t, src), fixtureRegistry(), []string{"cv", "sections", "x"}, schemaerr.SourceMain, sectionReference)
		if len(errs) != 1 {
			t.Fatalf("errs = %+v, want exactly one", errs)
		}
		want := "Each section should be a list of entries! This is not a list."
		if errs[0].Message != want {
			t.Errorf("message = %q, want %q", errs[0].Message, want)
		}
	}
}

// Spec §3.54 — an empty list infers nothing and produces no error.
func TestEmptySection(t *testing.T) {
	entryType, errs := cv.ValidateSection(section(t, "  []\n"), fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entryType != "" {
		t.Errorf("entry type = %q, want none inferred", entryType)
	}
}

// Spec §3.58 — per-entry inference.
func TestInferEntryType(t *testing.T) {
	registry := fixtureRegistry()
	tests := []struct {
		name    string
		src     string
		want    entries.TypeName
		wantErr string
	}{
		{name: "education", src: "  - institution: MIT\n", want: "EducationEntry"},
		{name: "experience", src: "  - company: Acme\n", want: "ExperienceEntry"},
		{name: "bullet", src: "  - bullet: A point\n", want: "BulletEntry"},
		{name: "bare string", src: "  - just text\n", want: cv.TextEntry},
		{name: "no characteristic field", src: "  - x: 1\n", wantErr: "The entry does not match any entry type."},
		{name: "null", src: "  - null\n", wantErr: "The entry cannot be None."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			elem := section(t, tc.src).Elems[0]
			got, err := cv.InferEntryType(elem, registry)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("entry type = %q, want %q", got, tc.want)
			}
		})
	}
}

// Spec §3.57 — priority order: an entry carrying characteristic fields of two
// types resolves to the earlier one in the declared order.
func TestDiscriminationPriority(t *testing.T) {
	elem := section(t, "  - institution: MIT\n    company: Acme\n").Elems[0]
	got, err := cv.InferEntryType(elem, fixtureRegistry())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "ExperienceEntry" {
		t.Errorf("entry type = %q, want ExperienceEntry — it comes first", got)
	}
}

// Spec §3.59, §5.6 — the first resolvable entry decides; a null entry is skipped
// during inference, so §4.10 never surfaces here.
//
// Inference skipping the null does not make the null valid: every entry is then
// validated against the decided type, and the null fails as a non-mapping.
// Verified upstream — this exact section reports
// `rendercv_entry_validation_error`, not the null-entry error. Iteration 2
// asserted no errors here only because the registered validator accepted
// everything (T19 replaced it).
func TestFirstResolvableEntryWins(t *testing.T) {
	entryType, errs := cv.ValidateSection(
		section(t, "  - null\n  - institution: MIT\n"),
		fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference,
	)
	if entryType != "EducationEntry" {
		t.Errorf("entry type = %q, want EducationEntry", entryType)
	}

	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly the entry-problems wrapper", errs)
	}
	if errs[0].Message == messageNullEntryText {
		t.Errorf("errs[0] is the null-entry error, which inference must have skipped")
	}
	if len(errs[0].Children) == 0 {
		t.Errorf("wrapper carries no children, want the per-entry failures")
	}
}

// messageNullEntryText is spec 002 §4.10, the error inference must never produce
// for a skipped null (section.py:167-171).
const messageNullEntryText = "The entry cannot be None."

// Spec §3.60, §4.11, §5.6 — nothing resolves, including the `[null]` case.
func TestNoEntryResolves(t *testing.T) {
	want := "RenderCV couldn't match this section with any entry types." +
		" Please check the entries and make sure they are provided correctly."
	for _, src := range []string{"  - null\n", "  - x: 1\n", "  - x: 1\n  - y: 2\n"} {
		_, errs := cv.ValidateSection(section(t, src), fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference)
		if len(errs) != 1 {
			t.Fatalf("%q: errs = %+v, want exactly one", src, errs)
		}
		if errs[0].Message != want {
			t.Errorf("%q: message = %q, want %q", src, errs[0].Message, want)
		}
	}
}

// Spec §3.61, §4.12 — every entry is validated against the one decided type and
// failures are re-raised with the type named and the children preserved.
func TestEntryProblemsAreNested(t *testing.T) {
	restore := cv.SetEntryValidatorForTest(func(
		node *yamldoc.Node, entryType entries.TypeName, location []string,
		source schemaerr.YamlSource, _ time.Time,
	) []schemaerr.ValidationError {
		if node.Kind != yamldoc.KindMapping {
			return nil
		}
		if _, ok := cv.MappingKey(node, "bullet"); ok {
			return nil
		}
		return []schemaerr.ValidationError{{
			Code: "missing", SchemaLocation: location, YamlSource: source, Message: "Field required",
		}}
	})
	defer restore()

	_, errs := cv.ValidateSection(
		section(t, "  - bullet: A point\n  - x: 1\n"),
		fixtureRegistry(), []string{"cv", "sections", "x"}, schemaerr.SourceMain, sectionReference,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	want := "There are problems with the entries. RenderCV detected the entry type of this" +
		" section to be BulletEntry. The problems are shown below."
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
	if len(errs[0].Children) != 1 {
		t.Fatalf("children = %+v, want exactly one nested failure", errs[0].Children)
	}
	if errs[0].Children[0].Message != "Field required" {
		t.Errorf("child message = %q, want it preserved structurally", errs[0].Children[0].Message)
	}
}

// Spec §5.8 — a section whose first entry does not resolve takes its type from
// the next one that does, and the bad entry is then reported against that type.
func TestTypeNamedIsTheResolvedOne(t *testing.T) {
	restore := cv.SetEntryValidatorForTest(func(
		node *yamldoc.Node, _ entries.TypeName, location []string,
		source schemaerr.YamlSource, _ time.Time,
	) []schemaerr.ValidationError {
		if _, ok := cv.MappingKey(node, "bullet"); ok {
			return nil
		}
		return []schemaerr.ValidationError{{
			Code: "missing", SchemaLocation: location, YamlSource: source, Message: "Field required",
		}}
	})
	defer restore()

	entryType, errs := cv.ValidateSection(
		section(t, "  - x: 1\n  - bullet: A point\n"),
		fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference,
	)
	if entryType != "BulletEntry" {
		t.Fatalf("entry type = %q, want BulletEntry", entryType)
	}
	if len(errs) != 1 || len(errs[0].Children) != 1 {
		t.Fatalf("errs = %+v, want one error with one child", errs)
	}
}

// Spec §5.7 — a mixed education/experience section resolves to the first
// entry's type and reports the other entries against it.
func TestMixedSectionNamesFirstResolvedType(t *testing.T) {
	restore := cv.SetEntryValidatorForTest(func(
		node *yamldoc.Node, entryType entries.TypeName, location []string,
		source schemaerr.YamlSource, _ time.Time,
	) []schemaerr.ValidationError {
		// Stand-in for the concrete types of iteration 3: an entry belongs to
		// the decided type only if it carries that type's characteristic field.
		field := map[entries.TypeName]string{
			"EducationEntry":  "institution",
			"ExperienceEntry": "company",
		}[entryType]
		if _, ok := cv.MappingKey(node, field); ok {
			return nil
		}
		return []schemaerr.ValidationError{{
			Code: "missing", SchemaLocation: location, YamlSource: source, Message: "Field required",
		}}
	})
	defer restore()

	entryType, errs := cv.ValidateSection(
		section(t, "  - institution: MIT\n    degree: BS\n  - company: Acme\n    position: Engineer\n"),
		fixtureRegistry(), []string{"cv", "sections", "mixed"}, schemaerr.SourceMain, sectionReference,
	)

	if entryType != "EducationEntry" {
		t.Fatalf("entry type = %q, want EducationEntry", entryType)
	}
	want := "There are problems with the entries. RenderCV detected the entry type of this" +
		" section to be EducationEntry. The problems are shown below."
	if len(errs) != 1 || errs[0].Message != want {
		t.Fatalf("errs = %+v, want one §4.12 naming EducationEntry", errs)
	}
	if len(errs[0].Children) != 1 {
		t.Errorf("children = %+v, want the experience entry's failure nested", errs[0].Children)
	}
}

// T19's guarantee: the validator reached through cv.Validate is the real
// dispatcher, not iteration 2's accept-everything stub. Asserted by giving a
// production path an entry that must fail and requiring that it does — a stub
// would return no errors and this test would catch it.
//
// The reference date defaults through a nil context to today, matching
// get_current_date (validation_context.py:36-58).
func TestProductionPathRejectsABadEntry(t *testing.T) {
	_, errs := cv.Validate(
		parse(t, "sections:\n  education:\n    - institution: MIT\n"),
		[]string{"cv"}, schemaerr.SourceMain, testOptions(),
	)
	if len(errs) == 0 {
		t.Fatal("errs is empty: an EducationEntry without `area` must fail, so the" +
			" registered validator is still accepting everything")
	}

	var sawMissingArea bool
	for _, err := range errs {
		for _, child := range err.Children {
			if child.Code == "missing" &&
				len(child.SchemaLocation) > 0 &&
				child.SchemaLocation[len(child.SchemaLocation)-1] == "area" {
				sawMissingArea = true
			}
		}
	}
	if !sawMissingArea {
		t.Errorf("errs = %+v, want a nested `missing` on `area`", errs)
	}
}
