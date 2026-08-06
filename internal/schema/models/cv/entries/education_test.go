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

var educationReference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func educationLocation() []string {
	return []string{"cv", "sections", "education", "0"}
}

func validateEducation(
	t *testing.T,
	src string,
) (*entries.EducationEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateEducationEntry(
		node,
		educationLocation(),
		schemaerr.SourceMain,
		educationReference,
	)
}

func assertEducationStrings(t *testing.T, label string, got, want []string) {
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

// Spec §3.9 — the nine field names, positionally. The three own fields come
// first even though BaseEntryWithComplexFields is the first-listed base
// (education.py:25-27).
func TestEducationDescriptorFields(t *testing.T) {
	descriptor := entries.EducationDescriptor()
	if descriptor.Name != "EducationEntry" {
		t.Errorf("name = %q, want %q", descriptor.Name, "EducationEntry")
	}

	assertEducationStrings(t, "fields", descriptor.Fields, []string{
		"institution",
		"area",
		"degree",
		"date",
		"start_date",
		"end_date",
		"location",
		"summary",
		"highlights",
	})
}

// Spec §3.17 behavior 39, plan §7 hazard 1 — the templater-injected names are
// never declared fields. `degree_column` in particular is not the declared
// `degree` field: it is a name the templater injects, and conflating the two
// would leak it into iteration 5's JSON schema.
func TestEducationDeclaresNoInjectedFields(t *testing.T) {
	fields := entries.EducationDescriptor().Fields
	for _, name := range []string{"main_column", "date_and_location_column", "degree_column"} {
		for _, field := range fields {
			if field == name {
				t.Errorf("fields = %v, want %q not declared", fields, name)
			}
		}
	}
}

// Spec §5.17 — the conftest fixture
// (tests/schema/models/cv/conftest.py:6-17) validates with zero errors, with
// its exact values including the diacritics of "Boğaziçi University".
func TestEducationFixtureValidates(t *testing.T) {
	entry, errs := validateEducation(t, `institution: Boğaziçi University
location: Istanbul, Turkey
degree: BS
area: Mechanical Engineering
start_date: 2015-09
end_date: 2020-06
highlights:
  - "GPA: 3.24/4.00 ([Transcript](https://example.com))"
  - "Awards: Dean's Honor List, Sportsperson of the Year"
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}

	for _, want := range []struct {
		name  string
		node  *yamldoc.Node
		value string
	}{
		{name: "institution", node: entry.Institution, value: "Boğaziçi University"},
		{name: "area", node: entry.Area, value: "Mechanical Engineering"},
		{name: "degree", node: entry.Degree, value: "BS"},
		{name: "location", node: entry.Location, value: "Istanbul, Turkey"},
	} {
		if want.node == nil {
			t.Fatalf("%s = nil, want %q", want.name, want.value)
		}
		if want.node.Raw != want.value {
			t.Errorf("%s = %q, want %q", want.name, want.node.Raw, want.value)
		}
	}

	if entry.StartDate != "2015-09" {
		t.Errorf("start_date = %q, want %q", entry.StartDate, "2015-09")
	}
	if entry.EndDate != "2020-06" {
		t.Errorf("end_date = %q, want %q", entry.EndDate, "2020-06")
	}
	if entry.DateValue != "" {
		t.Errorf("date = %q, want it absent", entry.DateValue)
	}
	if entry.Highlights == nil || len(entry.Highlights.Elems) != 2 {
		t.Fatalf("highlights = %+v, want two elements", entry.Highlights)
	}
	assertEducationStrings(t, "highlights", []string{
		entry.Highlights.Elems[0].Raw,
		entry.Highlights.Elems[1].Raw,
	}, []string{
		"GPA: 3.24/4.00 ([Transcript](https://example.com))",
		"Awards: Dean's Honor List, Sportsperson of the Year",
	})
}

// Spec §5.11 — an unknown key is retained and readable
// (tests/schema/models/cv/test_section.py:63-83).
func TestEducationExtraAttributeRetained(t *testing.T) {
	entry, errs := validateEducation(t, `institution: Boğaziçi University
area: Mechanical Engineering
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

// Spec §5.8 — an empty mapping reports the two required own fields in
// declaration order, `institution` before `area`: not alphabetical, and not the
// input's order, which has none. Verified against the vendored Python, which
// reports `[(('institution',), 'missing'), (('area',), 'missing')]`.
func TestEducationEmptyMappingReportsInstitutionThenArea(t *testing.T) {
	_, errs := validateEducation(t, "{}\n")
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want exactly two", errs)
	}
	for i, want := range []string{"institution", "area"} {
		if errs[i].Code != binder.CodeMissing {
			t.Errorf("errs[%d].code = %q, want %q", i, errs[i].Code, binder.CodeMissing)
		}
		assertEducationStrings(
			t,
			"schema location",
			errs[i].SchemaLocation,
			append(educationLocation(), want),
		)
	}
}

// Spec §5.7 — a required field written null is a type error, not a missing
// field. `degree` written null is neither: it is optional, so it stays absent
// and reports nothing. Verified against the vendored Python, which reports
// `string_type` on `institution` and `area` only.
func TestEducationNullFieldsAreStringType(t *testing.T) {
	entry, errs := validateEducation(t, "institution:\narea:\ndegree:\n")
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want exactly two", errs)
	}
	for i, want := range []string{"institution", "area"} {
		if errs[i].Code != binder.CodeStringType {
			t.Errorf("errs[%d].code = %q, want %q", i, errs[i].Code, binder.CodeStringType)
		}
		assertEducationStrings(
			t,
			"schema location",
			errs[i].SchemaLocation,
			append(educationLocation(), want),
		)
	}
	if entry.Institution != nil || entry.Area != nil || entry.Degree != nil {
		t.Errorf("entry = %+v, want all three own fields nil for null values", entry)
	}
}

// Spec §5.18, §5.22 — an entry without `degree` is valid and `degree` is
// **absent**, not empty text. No golden case omits `degree` (spec §5.1
// item 22), so the expectation is differential per spec §7.4: the vendored
// Python validating
// `{"institution": "Boğaziçi University", "area": "Mechanical Engineering"}`
// accepts it and reports `entry.degree is None`.
func TestEducationWithoutDegreeIsValid(t *testing.T) {
	entry, errs := validateEducation(t, `institution: Boğaziçi University
area: Mechanical Engineering
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Degree != nil {
		t.Errorf("degree = %+v, want it absent", entry.Degree)
	}
	if _, present := entry.Field("degree"); present {
		t.Errorf("degree key present, want absent")
	}
}

// Spec §5.23 and spec 002 §3.77 step 1 — a real bare `date` alongside
// `start_date` and `end_date` silently clears both range fields. No golden case
// reaches this on a type carrying all three (spec §5.1 item 23), so the
// expectation is differential per spec §7.4: the vendored Python, given
// `date: 2020-06`, `start_date: 2015-09`, `end_date: 2020-06`, reports
// `date='2020-06'`, `start_date=None`, `end_date=None` with no error.
func TestEducationBareDateClearsTheRange(t *testing.T) {
	entry, errs := validateEducation(t, `institution: Boğaziçi University
area: Mechanical Engineering
degree: BS
date: 2020-06
start_date: 2015-09
end_date: 2020-06
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.DateValue != "2020-06" {
		t.Errorf("date = %q, want %q", entry.DateValue, "2020-06")
	}
	if entry.StartDate != "" {
		t.Errorf("start_date = %q, want it cleared", entry.StartDate)
	}
	if entry.EndDate != "" {
		t.Errorf("end_date = %q, want it cleared", entry.EndDate)
	}
}

// Spec §5.25 — a non-blank `summary` is retained verbatim. Every education
// entry in the submodule leaves `summary` blank or absent (spec §5.1 item 25),
// so the expectation is differential per spec §7.4: the vendored Python returns
// the string unchanged, markdown markers and all.
func TestEducationSummaryRetainedVerbatim(t *testing.T) {
	const summary = "Graduated with **honors** and a thesis on turbomachinery."
	entry, errs := validateEducation(t, `institution: Boğaziçi University
area: Mechanical Engineering
summary: Graduated with **honors** and a thesis on turbomachinery.
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Summary == nil {
		t.Fatalf("summary = nil, want %q", summary)
	}
	if entry.Summary.Raw != summary {
		t.Errorf("summary = %q, want %q", entry.Summary.Raw, summary)
	}
}
