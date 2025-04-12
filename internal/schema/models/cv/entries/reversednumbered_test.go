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

func parseReversedNumbered(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return node
}

func validateReversedNumbered(
	t *testing.T, src string,
) (*entries.ReversedNumberedEntry, []schemaerr.ValidationError) {
	t.Helper()
	return entries.ValidateReversedNumberedEntry(
		parseReversedNumbered(t, src),
		[]string{"cv", "sections", "x", "0"},
		schemaerr.SourceMain,
		time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
	)
}

// Spec §3.6 — the field order is a parity surface and is asserted positionally.
func TestReversedNumberedDescriptorFields(t *testing.T) {
	got := entries.ReversedNumberedDescriptor()
	if got.Name != "ReversedNumberedEntry" {
		t.Errorf("name = %q, want %q", got.Name, "ReversedNumberedEntry")
	}
	want := []string{"reversed_number"}
	if len(got.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", got.Fields, want)
	}
	for i, name := range want {
		if got.Fields[i] != name {
			t.Errorf("fields[%d] = %q, want %q", i, got.Fields[i], name)
		}
	}
}

// Spec §4.7 — the description metadata is emitted verbatim in iteration 5.
func TestReversedNumberDescription(t *testing.T) {
	want := "Reverse-numbered list item. Numbering goes in reverse (5, 4, 3, 2, 1)," +
		" making recent items have higher numbers."
	if entries.ReversedNumberDescription != want {
		t.Errorf("description = %q, want %q", entries.ReversedNumberDescription, want)
	}
}

// Spec §5.19 — none of the templater's dynamic columns is a declared field.
func TestReversedNumberedHasNoDynamicColumns(t *testing.T) {
	for _, name := range []string{"main_column", "date_and_location_column", "degree_column"} {
		for _, field := range entries.ReversedNumberedDescriptor().Fields {
			if field == name {
				t.Errorf("%q is declared as a field, want it set dynamically", name)
			}
		}
	}
}

// Spec §3.6 — the field's own examples validate with zero errors; upstream has
// no conftest fixture for this type (spec §5.15).
func TestReversedNumberedValid(t *testing.T) {
	for _, value := range []string{"Latest research paper", "Recent patent application"} {
		t.Run(value, func(t *testing.T) {
			entry, errs := validateReversedNumbered(t, "reversed_number: "+value+"\n")
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if entry.ReversedNumber == nil || entry.ReversedNumber.Raw != value {
				t.Errorf("reversed_number = %+v, want %q", entry.ReversedNumber, value)
			}
		})
	}
}

// Spec §5.11 — an unknown key is retained and readable (test_section.py:63-83).
func TestReversedNumberedExtraKeyRetained(t *testing.T) {
	entry, errs := validateReversedNumbered(
		t, "reversed_number: Latest research paper\nextra_attribute: extra value\n",
	)
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

// Spec §4.3 — the required field absent reports `missing` at its own location.
func TestReversedNumberedMissing(t *testing.T) {
	_, errs := validateReversedNumbered(t, "{}\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != binder.CodeMissing {
		t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeMissing)
	}
	if errs[0].Message != "Field required" {
		t.Errorf("message = %q, want %q", errs[0].Message, "Field required")
	}
	got := errs[0].SchemaLocation
	if len(got) == 0 || got[len(got)-1] != "reversed_number" {
		t.Errorf("schema location = %v, want it to end in `reversed_number`", got)
	}
}

// Spec §5.7 — written as null the key is present, so the failure is
// `string_type`, not `missing`.
func TestReversedNumberedNullIsTypeError(t *testing.T) {
	_, errs := validateReversedNumbered(t, "reversed_number: null\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != binder.CodeStringType {
		t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeStringType)
	}
	if errs[0].Message != "Input should be a valid string" {
		t.Errorf("message = %q, want %q", errs[0].Message, "Input should be a valid string")
	}
}
