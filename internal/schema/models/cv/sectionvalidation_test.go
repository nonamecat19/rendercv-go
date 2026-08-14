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

	// Upstream gives exactly two children for this input, measured: the skipped
	// null fails as a non-mapping at index 0, and the MIT entry is missing `area`
	// at index 1. Asserted positionally with literal codes — a bare
	// "len(children) != 0" would pass with either child alone, or with both codes
	// wrong.
	var got []string
	for _, child := range errs[0].Children {
		got = append(got, strings.Join(child.SchemaLocation, ".")+" "+string(child.Code))
	}
	want := []string{"entries.0 model_type", "entries.1.area missing"}
	if len(got) != len(want) {
		t.Fatalf("children = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("children = %v, want %v", got, want)
		}
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
	// Relative to `entries`, which the pipeline's splice drops before prepending
	// the wrapper's location (spec 004 §3.7 behavior 22).
	if got := strings.Join(child.SchemaLocation, "."); got != "entries.0.bullet" {
		t.Errorf("child location = %q, want %q", got, "entries.0.bullet")
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
		"entries.1.institution missing",
		"entries.1.area missing",
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

// Spec §8, partially — the seven `(entry_type_name, section_model_name)` pairs,
// asserted from a **raw mapping**. Upstream asserts them twice, once from a dict
// and once from a constructed model instance
// (tests/schema/models/cv/test_section.py:19-60).
//
// **The constructed-entry half is deliberately not covered here, and is cut to
// iteration 4.** An earlier version of this test validated a node and then
// re-resolved the same node, which is a tautology: both calls take the identical
// KindMapping branch, so the second could only fail if Discriminate were
// nondeterministic. Upstream's already-a-model branch is a genuinely different
// path — `entry.__class__.__name__` at section.py:173-176 — and Go has no
// equivalent, because a non-mapping, non-string, non-null node falls through to
// the `messageNoType` branch under the TODO(iteration-12) at
// sectionvalidation.go. Reproducing it depends on iteration 4's §5.14
// already-a-model decision, so it is recorded in specs/STATE.md rather than
// approximated with a test that cannot fail.
func TestEntryTypeAndSectionModelPairs(t *testing.T) {
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

	// Upstream's own pairs, as literals rather than recomputed from the type name,
	// so this cannot agree with a wrong implementation of the name transform.
	wantModel := map[entries.TypeName]string{
		"PublicationEntry": "SectionWithPublicationEntries",
		"ExperienceEntry":  "SectionWithExperienceEntries",
		"EducationEntry":   "SectionWithEducationEntries",
		"NormalEntry":      "SectionWithNormalEntries",
		"OneLineEntry":     "SectionWithOneLineEntries",
		cv.TextEntry:       "SectionWithTextEntries",
		"BulletEntry":      "SectionWithBulletEntries",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entryType, errs := cv.ValidateSection(
				section(t, test.src), fixtureRegistry(), nil, schemaerr.SourceMain,
				sectionReference,
			)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if entryType != test.want {
				t.Fatalf("entry type = %q, want %q", entryType, test.want)
			}
			if got := sectionModelName(string(entryType)); got != wantModel[test.want] {
				t.Errorf("section model = %q, want %q", got, wantModel[test.want])
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

// Spec §3.59, §3.61 — a section that mixes a bare string with a mapping entry is
// refused **in both orders**, and the type it is refused against depends on the
// order.
//
// Found by a randomised differential fuzzer, reduced to
// `[Just text, {institution: Princeton}]`: upstream exits 1, the port exited 0.
// The mechanism is that a text section's `entries: list[str]`
// (`section.py:106-118`) type-checks every element, which the port's entry
// dispatcher was not doing — so a mapping in a string-first section was accepted
// by nobody in particular. The reverse order never had the defect, because there
// the decided type is a real model whose binder rejects a non-mapping.
//
// Every row's expectation is upstream's, measured whole-panel through the
// vendored Python at `render CV.yaml -nopdf -nopng -nomd -nohtml`.
func TestMixedTextAndMappingSectionsAreRefusedInBothOrders(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantType entries.TypeName
		want     []string
	}{
		{
			name:     "string first, then a mapping",
			src:      "  - Just text\n  - institution: Princeton\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "mapping first, then a string",
			src:      "  - institution: Princeton\n  - Just text\n",
			wantType: "EducationEntry",
			want:     []string{"entries.0.area missing", "entries.1 model_type"},
		},
		{
			name:     "string first, then two mappings of different types",
			src:      "  - Just text\n  - institution: P\n  - company: C\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type", "entries.2 string_type"},
		},
		{
			name:     "a mapping between two strings",
			src:      "  - one\n  - institution: P\n  - two\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "a string between two mappings",
			src:      "  - institution: P\n  - one\n  - institution: Q\n",
			wantType: "EducationEntry",
			want: []string{
				"entries.0.area missing",
				"entries.1 model_type",
				"entries.2.area missing",
			},
		},
		{
			name:     "null in a text section",
			src:      "  - Just text\n  - ~\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "an integer in a text section is not coerced",
			src:      "  - Just text\n  - 2020\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "a boolean in a text section",
			src:      "  - Just text\n  - true\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "a nested list in a text section",
			src:      "  - Just text\n  - [a, b]\n",
			wantType: cv.TextEntry,
			want:     []string{"entries.1 string_type"},
		},
		{
			name:     "a quoted number is a string and passes",
			src:      "  - Just text\n  - '2020'\n",
			wantType: cv.TextEntry,
			want:     nil,
		},
		{
			name:     "an all-strings section still renders",
			src:      "  - one\n  - two\n  - three\n",
			wantType: cv.TextEntry,
			want:     nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entryType, errs := cv.ValidateSection(
				section(t, test.src), fixtureRegistry(),
				[]string{"cv", "sections", "s"}, schemaerr.SourceMain, sectionReference,
			)
			if entryType != test.wantType {
				t.Errorf("entry type = %q, want %q", entryType, test.wantType)
			}

			if test.want == nil {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}

			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly the entry-problems wrapper", errs)
			}
			if errs[0].Code != cv.CodeEntryProblems {
				t.Errorf("code = %q, want %q", errs[0].Code, cv.CodeEntryProblems)
			}
			wantMessage := "There are problems with the entries. RenderCV detected the entry" +
				" type of this section to be " + string(test.wantType) +
				". The problems are shown below."
			if errs[0].Message != wantMessage {
				t.Errorf("message = %q, want %q", errs[0].Message, wantMessage)
			}

			var got []string
			for _, child := range errs[0].Children {
				got = append(got, strings.Join(child.SchemaLocation, ".")+" "+string(child.Code))
			}
			if len(got) != len(test.want) {
				t.Fatalf("children = %v, want %v", got, test.want)
			}
			for i := range test.want {
				if got[i] != test.want[i] {
					t.Fatalf("children = %v, want %v", got, test.want)
				}
			}
		})
	}
}
