package cv_test

import (
	"strings"
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
// iteration 3 T17 replaced it with entries.Default().
//
// Most tests below passed the swap untouched, but three did not and the earlier
// claim that all of them did was wrong: TestFirstResolvableEntryWins here, plus
// TestSectionRecordsInInputOrder and TestTypeComesFromFirstEntry in
// sectionlist_test.go, all asserted "no errors" over fixtures that the
// accept-everything stub let pass and that upstream genuinely rejects. Their
// expectations were corrected against the vendored Python, not relaxed.
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

// Spec 003 §5.13, §8 — a section whose first entry does not resolve takes its
// type from the next one that does, and the bad entry is reported against that
// type. Iteration 2 asserted this through an injected stub validator because the
// concrete types did not exist; T21 re-asserts it against the real ones, closing
// carried item 6 of iteration 2.
//
// Verified upstream: `[{x: 1}, {bullet: "A point"}]` reports one
// rendercv_entry_validation_error whose single child is `missing` at
// ('entries', 0, 'bullet').
func TestTypeNamedIsTheResolvedOne(t *testing.T) {
	entryType, errs := cv.ValidateSection(
		section(t, "  - x: 1\n  - bullet: A point\n"),
		fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference,
	)
	if entryType != "BulletEntry" {
		t.Fatalf("entry type = %q, want BulletEntry", entryType)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if len(errs[0].Children) != 1 {
		t.Fatalf("children = %+v, want exactly one", errs[0].Children)
	}

	child := errs[0].Children[0]
	if child.Code != "missing" {
		t.Errorf("child code = %q, want missing", child.Code)
	}
	if got := strings.Join(child.SchemaLocation, "."); got != "0.bullet" {
		t.Errorf("child location = %q, want %q", got, "0.bullet")
	}
}

// Spec 003 §5.12, §5.13, §8 — a mixed education/experience section resolves to
// the first entry's type and reports the others against it. The other half of
// carried item 6: iteration 2 used a stub here too.
//
// Verified upstream with the conftest fixture values: the wrapper names
// EducationEntry and its children are `missing` on `institution` then `area`, at
// entry index 1 — and nothing at all about `company` or `position`, because the
// experience entry is being judged as an EducationEntry.
func TestMixedSectionNamesFirstResolvedType(t *testing.T) {
	entryType, errs := cv.ValidateSection(
		section(t, ""+
			"  - institution: Boğaziçi University\n"+
			"    area: Mechanical Engineering\n"+
			"    degree: BS\n"+
			"  - company: Acme\n"+
			"    position: Engineer\n"),
		fixtureRegistry(), []string{"cv", "sections", "mixed"}, schemaerr.SourceMain,
		sectionReference,
	)
	if entryType != "EducationEntry" {
		t.Fatalf("entry type = %q, want EducationEntry — the first entry decides", entryType)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	want := "There are problems with the entries. RenderCV detected the entry type of this" +
		" section to be EducationEntry. The problems are shown below."
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}

	var got []string
	for _, child := range errs[0].Children {
		got = append(got, strings.Join(child.SchemaLocation, ".")+" "+string(child.Code))
	}
	expected := []string{
		"cv.sections.mixed.1.institution missing",
		"cv.sections.mixed.1.area missing",
	}
	if len(got) != len(expected) {
		t.Fatalf("children = %v, want %v", got, expected)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("children = %v, want %v", got, expected)
		}
	}

	// The experience entry is judged as an EducationEntry, so its own fields are
	// unknown keys there and must not be reported.
	for _, child := range errs[0].Children {
		last := child.SchemaLocation[len(child.SchemaLocation)-1]
		if last == "company" || last == "position" {
			t.Errorf("child reports %q, but the section is not an ExperienceEntry section", last)
		}
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

// Spec §8 — the seven `(entry_type_name, section_model_name)` pairs asserted from
// an **already-constructed entry**, not only from a raw mapping. Upstream tests
// both halves (tests/schema/models/cv/test_section.py:19-60) and its
// already-a-model branch is a distinct code path: `entry.__class__.__name__`
// rather than characteristic-field discrimination (section.py:173-176).
//
// Go has no single "constructed entry" interface to switch on, so the equivalent
// is to validate the entry first and then re-discriminate the same node: a
// successfully validated entry must resolve to the type it was validated as.
func TestConstructedEntryResolvesToItsOwnType(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want entries.TypeName
	}{
		{
			name: "publication",
			src:  "  - title: A Paper\n    authors:\n      - J. Doe\n",
			want: "PublicationEntry",
		},
		{
			name: "experience",
			src:  "  - company: Acme\n    position: Engineer\n",
			want: "ExperienceEntry",
		},
		{
			name: "education",
			src:  "  - institution: MIT\n    area: Computer Science\n",
			want: "EducationEntry",
		},
		{name: "normal", src: "  - name: Some Project\n", want: "NormalEntry"},
		{
			name: "one line",
			src:  "  - label: Languages\n    details: English\n",
			want: "OneLineEntry",
		},
		{name: "text", src: "  - just text\n", want: cv.TextEntry},
		{name: "bullet", src: "  - bullet: A point\n", want: "BulletEntry"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := section(t, test.src)

			// First pass: the section validates cleanly as the expected type.
			entryType, errs := cv.ValidateSection(
				node, fixtureRegistry(), nil, schemaerr.SourceMain, sectionReference,
			)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if entryType != test.want {
				t.Fatalf("entry type = %q, want %q", entryType, test.want)
			}

			// Second pass over the same, now-validated node: the decision is stable.
			// Upstream's already-a-model branch returns the instance's own class name,
			// so re-resolving must not drift to a different type.
			again, err := cv.InferEntryType(node.Elems[0], fixtureRegistry())
			if err != nil {
				t.Fatalf("re-resolving a validated entry: %v", err)
			}
			if again != test.want {
				t.Errorf("re-resolved to %q, want %q", again, test.want)
			}

			if got := sectionModelName(string(test.want)); got != "SectionWith"+
				strings.Replace(string(test.want), "Entry", "Entries", 1) {
				t.Errorf("section model name = %q, unexpected", got)
			}
		})
	}
}

// Spec §8, spec 002 §4.9 — `{summary: "only a summary"}` end to end. `summary` is
// characteristic of nothing (both BaseEntryWithComplexFields and
// BasePublicationEntry declare it), so it cannot discriminate and the section
// resolves to no type at all. Asserted here through ValidateSection rather than
// only at registry level.
func TestSummaryOnlyEntryMatchesNoType(t *testing.T) {
	entryType, errs := cv.ValidateSection(
		section(t, "  - summary: only a summary\n"),
		fixtureRegistry(), []string{"cv", "sections", "x"}, schemaerr.SourceMain,
		sectionReference,
	)
	if entryType != "" {
		t.Errorf("entry type = %q, want none inferred", entryType)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	want := "RenderCV couldn't match this section with any entry types." +
		" Please check the entries and make sure they are provided correctly."
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
	if errs[0].Code != cv.CodeSection {
		t.Errorf("code = %q, want %q", errs[0].Code, cv.CodeSection)
	}
}
