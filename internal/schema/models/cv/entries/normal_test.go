package entries_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var normalReference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func normalLocation() []string {
	return []string{"cv", "sections", "projects", "0"}
}

func validateNormal(t *testing.T, src string) (*entries.NormalEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateNormalEntry(node, normalLocation(), schemaerr.SourceMain, normalReference)
}

func assertNormalStrings(t *testing.T, label string, got, want []string) {
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

// Spec §3.7 — the field order, positionally. The own field comes first even
// though BaseEntryWithComplexFields is the first-listed base: pydantic emits the
// last-listed base's own fields first (normal.py:13-15, spec §3.2).
func TestNormalDescriptorFields(t *testing.T) {
	descriptor := entries.NormalDescriptor()
	if descriptor.Name != "NormalEntry" {
		t.Errorf("name = %q, want %q", descriptor.Name, "NormalEntry")
	}

	want := []string{"name", "date", "start_date", "end_date", "location", "summary", "highlights"}
	assertNormalStrings(t, "fields", descriptor.Fields, want)
}

// Spec §3.17 behavior 39, plan §7 hazard 1 — the templater-injected names are
// never declared fields, or they would leak into iteration 5's JSON schema.
func TestNormalDeclaresNoInjectedFields(t *testing.T) {
	fields := entries.NormalDescriptor().Fields
	for _, name := range []string{"main_column", "date_and_location_column", "degree_column"} {
		for _, field := range fields {
			if field == name {
				t.Errorf("fields = %v, want %q not declared", fields, name)
			}
		}
	}
}

// Spec §5.17 — the conftest fixture (tests/schema/models/cv/conftest.py:34-42)
// validates with zero errors and every value is retained as written.
func TestNormalFixtureValidates(t *testing.T) {
	entry, errs := validateNormal(t, `name: Some Project
location: Remote
date: 2021-09
highlights:
  - Developed a web application with **React** and **Django**.
  - Implemented a **RESTful API**
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Name == nil || entry.Name.Raw != "Some Project" {
		t.Errorf("name = %+v, want %q", entry.Name, "Some Project")
	}
	if entry.Location == nil || entry.Location.Raw != "Remote" {
		t.Errorf("location = %+v, want %q", entry.Location, "Remote")
	}
	if entry.DateValue != "2021-09" {
		t.Errorf("date = %q, want %q", entry.DateValue, "2021-09")
	}
	if entry.Highlights == nil || len(entry.Highlights.Elems) != 2 {
		t.Fatalf("highlights = %+v, want two elements", entry.Highlights)
	}
	wantHighlights := []string{
		"Developed a web application with **React** and **Django**.",
		"Implemented a **RESTful API**",
	}
	for i, elem := range entry.Highlights.Elems {
		if elem.Raw != wantHighlights[i] {
			t.Errorf("highlights[%d] = %q, want %q", i, elem.Raw, wantHighlights[i])
		}
	}
}

// Spec §5.11 — an unknown key is retained and readable
// (tests/schema/models/cv/test_section.py:63-83).
func TestNormalExtraAttributeRetained(t *testing.T) {
	entry, errs := validateNormal(t, "name: Some Project\nextra_attribute: extra value\n")
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

// Spec §4.3 — the required own field missing.
func TestNormalMissingField(t *testing.T) {
	_, errs := validateNormal(t, "location: Remote\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	// Upstream's literal rather than `binder.CodeMissing`, for the reason
	// TestNormalDateRejections spells out below.
	if errs[0].Code != "missing" {
		t.Errorf("code = %q, want %q", errs[0].Code, "missing")
	}
	assertNormalStrings(
		t, "schema location", errs[0].SchemaLocation, append(normalLocation(), "name"),
	)
}

// Spec §5.7 — a required field written null is a type error, not a missing
// field: the key is present and only its value is wrong.
func TestNormalNullNameIsStringType(t *testing.T) {
	entry, errs := validateNormal(t, "name:\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != "string_type" {
		t.Errorf("code = %q, want %q", errs[0].Code, "string_type")
	}
	if entry.Name != nil {
		t.Errorf("name = %+v, want nil for a null value", entry.Name)
	}
}

// Spec §5.21 — spec 002 §5.23's rejection table reached through this concrete
// type rather than through a bare BaseEntryWithComplexFields.
func TestNormalDateRejections(t *testing.T) {
	// The codes are written as upstream's literal strings rather than as the Go
	// constants, deliberately: asserting `bases.CodeDateOther` here would stay green
	// if that constant were changed to the wrong value, which is exactly how the
	// wrong codes survived the first round of this iteration.
	//
	// The codes differ per row and that is upstream's doing, not an accident:
	// `start_date`/`end_date` failures are raised as
	// PydanticCustomError(CustomPydanticErrorTypes.other)
	// (entry_with_complex_fields.py:31-36, :161-169), while the arbitrary `date`
	// lets a bare ValueError through (entry_with_date.py:26-29), which pydantic
	// reports as value_error. Measured on all three rows.
	tests := []struct {
		name     string
		input    string
		location []string
		code     schemaerr.Code
		message  string
	}{
		{
			name:     "start_date aaa",
			input:    "name: n\nstart_date: aaa\n",
			location: append(normalLocation(), "start_date"),
			code:     "rendercv_other_error",
			message: "This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM," +
				" or YYYY format.",
		},
		{
			name:     "start_date after end_date",
			input:    "name: n\nstart_date: 2023-01-01\nend_date: 2021-01-01\n",
			location: normalLocation(),
			code:     "rendercv_other_error",
			message: "`start_date` cannot be after `end_date`. The `start_date` is 2023-01-01" +
				" and the `end_date` is 2021-01-01.",
		},
		{
			name:     "date 2020-20-20",
			input:    "name: n\ndate: 2020-20-20\n",
			location: append(normalLocation(), "date"),
			code:     "value_error",
			message:  "month must be in 1..12",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validateNormal(t, test.input)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != test.code {
				t.Errorf("code = %q, want %q", errs[0].Code, test.code)
			}
			if errs[0].Message != test.message {
				t.Errorf("message = %q, want %q", errs[0].Message, test.message)
			}
			assertNormalStrings(t, "schema location", errs[0].SchemaLocation, test.location)
		})
	}
}

// Spec §5.21 — spec 002 §5.23's four accepting forms, including the integer
// `2020`, all reached through this concrete type and all retained as written.
func TestNormalDateAcceptances(t *testing.T) {
	tests := []struct {
		name  string
		start string
		kind  yamldoc.Kind
	}{
		{name: "full date", start: "2020-01-01", kind: yamldoc.KindString},
		{name: "year and month", start: "2020-01", kind: yamldoc.KindString},
		{name: "quoted year", start: "'2020'", kind: yamldoc.KindString},
		{name: "integer year", start: "2020", kind: yamldoc.KindInt},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, errs := validateNormal(
				t, "name: n\nstart_date: "+test.start+"\nend_date: 2021\n",
			)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			want := strings.Trim(test.start, "'")
			if entry.StartDate != want {
				t.Errorf("start_date = %q, want %q", entry.StartDate, want)
			}
			node, ok := entry.Field("start_date")
			if !ok || node == nil {
				t.Fatalf("start_date field = %+v, want it bound", node)
			}
			if node.Kind != test.kind {
				t.Errorf("start_date kind = %v, want %v", node.Kind, test.kind)
			}
			if entry.EndDate != "2021" {
				t.Errorf("end_date = %q, want %q", entry.EndDate, "2021")
			}
		})
	}
}

// Spec §5.25, §7.4 — a non-blank `summary` is retained verbatim. No golden case
// sets a non-blank `summary` on any type but NormalEntry (spec §5.1 item 25), so
// this expectation is differentially obtained: the vendored Python was run on
// this exact input and `NormalEntry.summary` came back
// `'A **non-blank** summary.'`, unchanged and untrimmed.
func TestNormalNonBlankSummaryRetained(t *testing.T) {
	entry, errs := validateNormal(t, `name: Some Project
summary: A **non-blank** summary.
start_date: 2020-01-01
end_date: 2021-01-01
`)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Summary == nil {
		t.Fatalf("summary = nil, want it bound")
	}
	if entry.Summary.Raw != "A **non-blank** summary." {
		t.Errorf("summary = %q, want %q", entry.Summary.Raw, "A **non-blank** summary.")
	}
	// Upstream leaves the range fields alone when no bare `date` is written
	// (spec 002 §3.77): the same run reported start_date '2020-01-01' and
	// end_date '2021-01-01'.
	if entry.StartDate != "2020-01-01" || entry.EndDate != "2021-01-01" {
		t.Errorf(
			"dates = (%q, %q), want (%q, %q)",
			entry.StartDate, entry.EndDate, "2020-01-01", "2021-01-01",
		)
	}
	if entry.DateValue != "" {
		t.Errorf("date = %q, want empty", entry.DateValue)
	}
}
