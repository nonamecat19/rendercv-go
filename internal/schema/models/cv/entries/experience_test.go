package entries_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var experienceReference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func experienceLocation() []string {
	return []string{"cv", "sections", "experience", "0"}
}

func validateExperience(
	t *testing.T,
	src string,
) (*entries.ExperienceEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateExperienceEntry(
		node, experienceLocation(), schemaerr.SourceMain, experienceReference,
	)
}

func assertExperienceStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}

// Spec §3.8 — the field order, positionally. The own fields come first even
// though BaseEntryWithComplexFields is the first-listed base, because pydantic
// emits the last-listed base's own fields first (experience.py:16).
func TestExperienceDescriptorFields(t *testing.T) {
	descriptor := entries.ExperienceDescriptor()
	if descriptor.Name != "ExperienceEntry" {
		t.Errorf("name = %q, want %q", descriptor.Name, "ExperienceEntry")
	}

	want := []string{
		"company", "position", "date", "start_date", "end_date",
		"location", "summary", "highlights",
	}
	assertExperienceStrings(t, "fields", descriptor.Fields, want)
}

// Spec §3.17 behavior 39, plan §7 hazard 1 — the templater-injected names are
// never declared fields, or they would leak into iteration 5's JSON schema.
func TestExperienceDeclaresNoInjectedFields(t *testing.T) {
	fields := entries.ExperienceDescriptor().Fields
	for _, name := range []string{"main_column", "date_and_location_column", "degree_column"} {
		for _, field := range fields {
			if field == name {
				t.Errorf("fields = %v, want %q not declared", fields, name)
			}
		}
	}
}

// Spec §5.17 — the conftest fixture (tests/schema/models/cv/conftest.py:19-32)
// validates with zero errors, with its exact values.
func TestExperienceFixtureValidates(t *testing.T) {
	entry, errs := validateExperience(t, `company: Some Company
location: TX, USA
position: Software Engineer
start_date: 2020-07
end_date: 2021-08-12
highlights:
  - Developed an [IOS application](https://example.com) that has received more than **100,000 downloads**.
  - Managed a team of **5** engineers.
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Company == nil || entry.Company.Raw != "Some Company" {
		t.Errorf("company = %+v, want %q", entry.Company, "Some Company")
	}
	if entry.Position == nil || entry.Position.Raw != "Software Engineer" {
		t.Errorf("position = %+v, want %q", entry.Position, "Software Engineer")
	}
	if entry.Location == nil || entry.Location.Raw != "TX, USA" {
		t.Errorf("location = %+v, want %q", entry.Location, "TX, USA")
	}
	if entry.StartDate != "2020-07" {
		t.Errorf("start_date = %q, want %q", entry.StartDate, "2020-07")
	}
	if entry.EndDate != "2021-08-12" {
		t.Errorf("end_date = %q, want %q", entry.EndDate, "2021-08-12")
	}
	if entry.Highlights == nil || len(entry.Highlights.Elems) != 2 {
		t.Fatalf("highlights = %+v, want two items", entry.Highlights)
	}
	wantHighlights := []string{
		"Developed an [IOS application](https://example.com) that has received more than" +
			" **100,000 downloads**.",
		"Managed a team of **5** engineers.",
	}
	got := make([]string, 0, len(entry.Highlights.Elems))
	for _, elem := range entry.Highlights.Elems {
		got = append(got, elem.Raw)
	}
	assertExperienceStrings(t, "highlights", got, wantHighlights)
}

// Spec §5.11 — an unknown key is retained and readable
// (tests/schema/models/cv/test_section.py:63-83).
func TestExperienceExtraAttributeRetained(t *testing.T) {
	entry, errs := validateExperience(t, `company: Some Company
position: Software Engineer
extra_attribute: extra value
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("extra_attribute")
	if !ok {
		t.Fatalf("extra keys = %v, want extra_attribute retained", entry.ExtraKeys())
	}
	if value.Raw != "extra value" {
		t.Errorf("extra_attribute = %q, want %q", value.Raw, "extra value")
	}
}

// Spec §4.3, §5.8 — each required own field missing, and both missing at once in
// declaration order: `company` then `position`.
func TestExperienceMissingFields(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		wants []string
	}{
		{name: "company", src: "position: Software Engineer\n", wants: []string{"company"}},
		{name: "position", src: "company: Some Company\n", wants: []string{"position"}},
		{name: "both", src: "other: value\n", wants: []string{"company", "position"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validateExperience(t, test.src)
			if len(errs) != len(test.wants) {
				t.Fatalf("errs = %+v, want %d", errs, len(test.wants))
			}
			for i, want := range test.wants {
				if errs[i].Code != binder.CodeMissing {
					t.Errorf("errs[%d].code = %q, want %q", i, errs[i].Code, binder.CodeMissing)
				}
				assertExperienceStrings(
					t, "schema location",
					errs[i].SchemaLocation, append(experienceLocation(), want),
				)
			}
		})
	}
}

// Spec §5.7 — a required field written null is a type error, not a missing
// field: the key is present and only its value is wrong.
func TestExperienceNullFieldsAreStringType(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "company", src: "company:\nposition: Software Engineer\n", want: "company"},
		{name: "position", src: "company: Some Company\nposition:\n", want: "position"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, errs := validateExperience(t, test.src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != binder.CodeStringType {
				t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeStringType)
			}
			assertExperienceStrings(
				t, "schema location",
				errs[0].SchemaLocation, append(experienceLocation(), test.want),
			)
			if test.want == "company" && entry.Company != nil {
				t.Errorf("company = %+v, want nil for a null value", entry.Company)
			}
			if test.want == "position" && entry.Position != nil {
				t.Errorf("position = %+v, want nil for a null value", entry.Position)
			}
		})
	}
}

// Spec §5.14 — `date: "No."` is a legal arbitrary date, so the only error is the
// missing `position` (wrong_input.yaml:25-28 → expected_errors.yaml:69-79).
func TestExperienceArbitraryDatePassesAndOnlyPositionIsMissing(t *testing.T) {
	entry, errs := validateExperience(t, `company: Company C
location: Location C
date: "No."
`)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != binder.CodeMissing {
		t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeMissing)
	}
	assertExperienceStrings(
		t, "schema location",
		errs[0].SchemaLocation, append(experienceLocation(), "position"),
	)
	if entry.DateValue != "No." {
		t.Errorf("date = %q, want %q", entry.DateValue, "No.")
	}
}

// Spec §5.23, spec 002 §3.77 step 1 — a real bare `date` alongside
// `start_date`/`end_date` silently clears both range fields, with no diagnostic.
//
// No golden case exercises this on an experience entry (spec §5.1 item 23), so
// per spec §7.4 the expectation was obtained differentially by running the
// vendored Python on the same input:
//
//	ExperienceEntry(company='Some Company', position='Software Engineer',
//	    date='2021', start_date='2020-07', end_date='2021-08-12',
//	    location='TX, USA')
//	→ date='2021', start_date=None, end_date=None
func TestExperienceBareDateClearsRangeFields(t *testing.T) {
	entry, errs := validateExperience(t, `company: Some Company
position: Software Engineer
date: "2021"
start_date: 2020-07
end_date: 2021-08-12
location: TX, USA
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.DateValue != "2021" {
		t.Errorf("date = %q, want %q", entry.DateValue, "2021")
	}
	if entry.StartDate != "" {
		t.Errorf("start_date = %q, want it cleared", entry.StartDate)
	}
	if entry.EndDate != "" {
		t.Errorf("end_date = %q, want it cleared", entry.EndDate)
	}
}

// Spec §5.25 — a non-blank `summary` validates and is retained verbatim. Every
// golden case leaves an experience `summary` blank or absent (spec §5.1 item
// 25), so per spec §7.4 the expectation was obtained differentially:
//
//	ExperienceEntry(company='C', position='P', summary='A **real** summary.')
//	→ summary='A **real** summary.'
func TestExperienceSummaryRetainedVerbatim(t *testing.T) {
	entry, errs := validateExperience(t, `company: C
position: P
summary: A **real** summary.
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Summary == nil {
		t.Fatalf("summary = nil, want it bound")
	}
	if entry.Summary.Kind != yamldoc.KindString || entry.Summary.Raw != "A **real** summary." {
		t.Errorf("summary = %+v, want the string %q", entry.Summary, "A **real** summary.")
	}
}
