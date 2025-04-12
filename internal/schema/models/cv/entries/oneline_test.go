package entries_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var oneLineReference = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func validateOneLine(t *testing.T, src string) (*entries.OneLineEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateOneLineEntry(
		node, []string{"cv", "sections", "x", "0"}, schemaerr.SourceMain, oneLineReference,
	)
}

// Spec §3.3 — two fields in declaration order. Asserted positionally because the
// order is a parity surface: `details` sorts before `label` alphabetically, so a
// set-based assertion would pass while being wrong. Runtime order verified
// against the vendored Python: `['label', 'details']`.
func TestOneLineDescriptorFields(t *testing.T) {
	got := entries.OneLineDescriptor()
	if got.Name != "OneLineEntry" {
		t.Errorf("name = %q, want %q", got.Name, "OneLineEntry")
	}

	want := []string{"label", "details"}
	if len(got.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", got.Fields, want)
	}
	for i, name := range want {
		if got.Fields[i] != name {
			t.Fatalf("fields = %v, want %v", got.Fields, want)
		}
	}
}

// BaseEntry contributes no fields, and the columns the templater sets
// dynamically in iteration 8 are not declared fields (spec §3.3, §5.19).
func TestOneLineDescriptorDeclaresNoOtherFields(t *testing.T) {
	declared := make(map[string]bool)
	for _, name := range entries.OneLineDescriptor().Fields {
		declared[name] = true
	}
	for _, name := range []string{
		"date", "start_date", "end_date", "location", "summary", "highlights",
		"main_column", "date_and_location_column", "degree_column",
	} {
		if declared[name] {
			t.Errorf("field %q is declared, want it absent", name)
		}
	}
}

// Spec §5.17 — the conftest fixture, with its exact values
// (tests/schema/models/cv/conftest.py:55-58).
func TestOneLineFixtureValidates(t *testing.T) {
	entry, errs := validateOneLine(t, ""+
		"label: Programming\n"+
		"details: Python, C++, JavaScript, MATLAB\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Label == nil || entry.Label.Raw != "Programming" {
		t.Errorf("label = %+v, want %q", entry.Label, "Programming")
	}
	if entry.Details == nil || entry.Details.Raw != "Python, C++, JavaScript, MATLAB" {
		t.Errorf("details = %+v, want %q", entry.Details, "Python, C++, JavaScript, MATLAB")
	}
}

// Spec §5.11 — upstream parametrizes `extra_attribute` over every entry model and
// asserts it is readable (tests/schema/models/cv/test_section.py:63-83).
func TestOneLineExtraAttributeRetained(t *testing.T) {
	entry, errs := validateOneLine(t, ""+
		"label: Programming\n"+
		"details: Python, C++, JavaScript, MATLAB\n"+
		"extra_attribute: extra value\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("extra_attribute")
	if !ok {
		t.Fatalf("extra_attribute absent, want it retained")
	}
	if value.Raw != "extra value" {
		t.Errorf("extra_attribute = %q, want %q", value.Raw, "extra value")
	}
}

// Spec §4.3, §5.8 — required fields absent report `missing`, in declaration
// order, so `label` precedes `details` when both are gone.
func TestOneLineMissingFields(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		want  []string
		codes []schemaerr.Code
	}{
		{
			name:  "both absent report label then details",
			src:   "extra_attribute: extra value\n",
			want:  []string{"cv.sections.x.0.label", "cv.sections.x.0.details"},
			codes: []schemaerr.Code{binder.CodeMissing, binder.CodeMissing},
		},
		{
			name:  "label absent",
			src:   "details: Python, C++, JavaScript, MATLAB\n",
			want:  []string{"cv.sections.x.0.label"},
			codes: []schemaerr.Code{binder.CodeMissing},
		},
		{
			name:  "details absent",
			src:   "label: Programming\n",
			want:  []string{"cv.sections.x.0.details"},
			codes: []schemaerr.Code{binder.CodeMissing},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validateOneLine(t, test.src)
			if len(errs) != len(test.want) {
				t.Fatalf("errs = %+v, want %d", errs, len(test.want))
			}
			for i, want := range test.want {
				if got := strings.Join(errs[i].SchemaLocation, "."); got != want {
					t.Errorf("errs[%d] location = %q, want %q", i, got, want)
				}
				if errs[i].Code != test.codes[i] {
					t.Errorf("errs[%d] code = %q, want %q", i, errs[i].Code, test.codes[i])
				}
			}
		})
	}
}

// Spec §5.7 — a required field written null is a type error, not a missing field:
// the key is present, only its value is wrong.
func TestOneLineNullFieldIsStringType(t *testing.T) {
	tests := []struct {
		name string
		src  string
		path string
	}{
		{
			name: "label null",
			src:  "label: null\ndetails: Python\n",
			path: "cv.sections.x.0.label",
		},
		{
			name: "details null",
			src:  "label: Programming\ndetails: null\n",
			path: "cv.sections.x.0.details",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validateOneLine(t, test.src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != binder.CodeStringType {
				t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeStringType)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != test.path {
				t.Errorf("location = %q, want %q", got, test.path)
			}
		})
	}
}

// Spec §4.4 — both fields are text, so a non-text value in either is rejected
// rather than coerced.
func TestOneLineNonTextFields(t *testing.T) {
	for _, src := range []string{
		"label: 2020\ndetails: Python\n",
		"label: Programming\ndetails:\n  - Python\n",
	} {
		t.Run(src, func(t *testing.T) {
			_, errs := validateOneLine(t, src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != binder.CodeStringType {
				t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeStringType)
			}
		})
	}
}

// Spec §5.19 — a `date` written on a OneLineEntry is an unknown key and is not
// validated as a date.
func TestOneLineDateIsAnUnknownKey(t *testing.T) {
	entry, errs := validateOneLine(t, ""+
		"label: Programming\n"+
		"details: Python\n"+
		"date: not-a-date\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("date")
	if !ok {
		t.Fatalf("date absent from extras, want it retained")
	}
	if value.Kind != yamldoc.KindString || value.Raw != "not-a-date" {
		t.Errorf("date = %+v, want the string %q", value, "not-a-date")
	}
}
