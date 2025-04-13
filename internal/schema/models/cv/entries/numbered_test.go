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

var numberedReference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func validateNumbered(
	t *testing.T, src string,
) (*entries.NumberedEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateNumberedEntry(
		node, []string{"cv", "sections", "x", "0"}, schemaerr.SourceMain, numberedReference,
	)
}

// Spec 003 §3.5 — the field order is a parity surface, so it is asserted
// positionally. BaseEntry contributes no fields, so `number` is the whole set
// (numbered.py:6-9, entries/bases/entry.py:11-18).
func TestNumberedDescriptorFields(t *testing.T) {
	descriptor := entries.NumberedDescriptor()
	if descriptor.Name != "NumberedEntry" {
		t.Errorf("name = %q, want %q", descriptor.Name, "NumberedEntry")
	}

	want := []string{"number"}
	if len(descriptor.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", descriptor.Fields, want)
	}
	for i, name := range want {
		if descriptor.Fields[i] != name {
			t.Fatalf("fields = %v, want %v", descriptor.Fields, want)
		}
	}
}

// Spec 003 §5 edge case 19 — NumberedEntry descends directly from BaseEntry, so
// it has no date fields; and the templater's dynamic columns of spec §6.5 are
// not declared fields either.
func TestNumberedDescriptorDeclaresNoDynamicOrDateFields(t *testing.T) {
	forbidden := []string{
		"main_column", "date_and_location_column", "degree_column",
		"date", "start_date", "end_date", "location", "summary", "highlights",
	}
	for _, name := range entries.NumberedDescriptor().Fields {
		for _, bad := range forbidden {
			if name == bad {
				t.Errorf("field %q must not be declared", name)
			}
		}
	}
}

// Spec 003 §5 edge case 17 — the conftest fixture's exact bytes
// (tests/schema/models/cv/conftest.py:64-66) validate with zero errors.
func TestNumberedEntryConftestFixture(t *testing.T) {
	entry, errs := validateNumbered(t, "number: This is a numbered entry.\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Number == nil || entry.Number.Raw != "This is a numbered entry." {
		t.Errorf("number = %+v, want %q", entry.Number, "This is a numbered entry.")
	}
}

// Spec 003 §3.5 — the field's own examples (numbered.py:8) validate too.
func TestNumberedEntryUpstreamExamples(t *testing.T) {
	for _, example := range []string{
		"First publication about XYZ",
		"Patent for ABC technology",
	} {
		t.Run(example, func(t *testing.T) {
			entry, errs := validateNumbered(t, "number: "+example+"\n")
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if entry.Number.Raw != example {
				t.Errorf("number = %q, want %q", entry.Number.Raw, example)
			}
		})
	}
}

// Spec 003 §5 edge case 11 — extra keys are retained and readable on every
// entry model (tests/schema/models/cv/test_section.py:63-83).
func TestNumberedEntryRetainsExtraAttribute(t *testing.T) {
	entry, errs := validateNumbered(t, ""+
		"number: This is a numbered entry.\n"+
		"extra_attribute: extra value\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}

	extra, ok := entry.Extra("extra_attribute")
	if !ok {
		t.Fatalf("extra keys = %v, want extra_attribute", entry.ExtraKeys())
	}
	if extra.Raw != "extra value" {
		t.Errorf("extra_attribute = %q, want %q", extra.Raw, "extra value")
	}
}

// Spec 003 §4.3 and §4.4 — an absent required field is `missing`, while a
// required field written null is `string_type`: the key is present, only its
// value is wrong (spec §5 edge case 7).
func TestNumberedEntryRequiredFieldErrors(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantCode schemaerr.Code
		wantMsg  string
	}{
		{
			name:     "number absent",
			src:      "other: value\n",
			wantCode: binder.CodeMissing,
			wantMsg:  "Field required",
		},
		{
			name:     "number null",
			src:      "number: null\n",
			wantCode: binder.CodeStringType,
			wantMsg:  "Input should be a valid string",
		},
		{
			name:     "number is a mapping",
			src:      "number:\n  a: 1\n",
			wantCode: binder.CodeStringType,
			wantMsg:  "Input should be a valid string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := validateNumbered(t, test.src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != test.wantCode {
				t.Errorf("code = %q, want %q", errs[0].Code, test.wantCode)
			}
			if errs[0].Message != test.wantMsg {
				t.Errorf("message = %q, want %q", errs[0].Message, test.wantMsg)
			}
			want := "cv.sections.x.0.number"
			if got := strings.Join(errs[0].SchemaLocation, "."); got != want {
				t.Errorf("location = %q, want %q", got, want)
			}
		})
	}
}

// Spec 003 §5 edge case 19 — a `date` on a numbered entry is an unknown key and
// is not validated as a date.
func TestNumberedEntryDateIsAnUnknownKey(t *testing.T) {
	entry, errs := validateNumbered(t, ""+
		"number: This is a numbered entry.\n"+
		"date: not-a-date\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}

	extra, ok := entry.Extra("date")
	if !ok {
		t.Fatalf("extra keys = %v, want date", entry.ExtraKeys())
	}
	if extra.Kind != yamldoc.KindString || extra.Raw != "not-a-date" {
		t.Errorf("date = %+v, want the string %q", extra, "not-a-date")
	}
}
