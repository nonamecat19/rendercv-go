package bases_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func parse(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return node
}

// Spec §3.67, §5.24 — an unknown key on an entry base is retained and readable.
func TestExtraKeysRetained(t *testing.T) {
	entry, errs := bases.BindEntryWithDate(
		parse(t, "date: 2020-09-24\ncustom_key: custom value\n"),
		bases.DateSpec(nil), []string{"cv", "sections", "x", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("custom_key")
	if !ok {
		t.Fatalf("extra keys = %v, want custom_key retained", entry.ExtraKeys())
	}
	if value.Raw != "custom value" {
		t.Errorf("custom_key = %q, want %q", value.Raw, "custom value")
	}
}

// Spec §3.70 — `date` defaults to absent.
func TestDateDefaultsToAbsent(t *testing.T) {
	entry, errs := bases.BindEntryWithDate(parse(t, "{}\n"), bases.DateSpec(nil), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Date != nil {
		t.Errorf("date = %+v, want absent", entry.Date)
	}
}

// Spec §3.69 — arbitrary dates: strict forms are range-checked, everything else passes.
func TestValidateArbitraryDate(t *testing.T) {
	tests := []struct {
		value   string
		wantErr string
	}{
		{value: "2020-09-24"},
		{value: "2020-09"},
		{value: "2020"},
		{value: "Fall 2023"},
		{value: "Summer 2020"},
		{value: "2024-02-29"},
		{value: "2022-20-20", wantErr: "month must be in 1..12"},
		{value: "2020-99-99", wantErr: "month must be in 1..12"},
		{value: "2020-01-99", wantErr: "day is out of range for month"},
		{value: "2020-09-99", wantErr: "day is out of range for month"},
		{value: "2020-20-20", wantErr: "month must be in 1..12"},
		{value: "0000-01-01", wantErr: "year 0 is out of range"},
		{value: "2023-02-29", wantErr: "day is out of range for month"},
		// Not a full match, so no range check runs at all.
		{value: "20222"},
		{value: "202222-20200"},
		{value: "202222-12-20"},
		{value: "invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			err := bases.ValidateArbitraryDate(tc.value)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateArbitraryDate(%q) = %v, want nil", tc.value, err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("ValidateArbitraryDate(%q) = %v, want %q", tc.value, err, tc.wantErr)
			}
		})
	}
}

// A rejected date is reported at its own schema location with the date library's text.
func TestDateErrorIsReported(t *testing.T) {
	_, errs := bases.BindEntryWithDate(
		parse(t, "date: 2020-01-99\n"),
		bases.DateSpec(nil), []string{"cv", "sections", "x", "0"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Message != "day is out of range for month" {
		t.Errorf("message = %q, want %q", errs[0].Message, "day is out of range for month")
	}
	got := errs[0].SchemaLocation
	if len(got) != 5 || got[4] != "date" {
		t.Errorf("schema location = %v, want it to end in `date`", got)
	}
}

// The integer form takes the pass-through branch, as `str(date)` does upstream.
func TestIntegerDateAccepted(t *testing.T) {
	entry, errs := bases.BindEntryWithDate(parse(t, "date: 2020\n"), bases.DateSpec(nil), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Date == nil || entry.Date.Kind != yamldoc.KindInt {
		t.Errorf("date = %+v, want the integer node kept as written", entry.Date)
	}
}

// Spec §3.69 — a null date validates nothing.
func TestNullDateIsAbsent(t *testing.T) {
	entry, errs := bases.BindEntryWithDate(parse(t, "date: null\n"), bases.DateSpec(nil), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Date != nil {
		t.Errorf("date = %+v, want absent", entry.Date)
	}
}

// Declared fields of a concrete entry type bind alongside `date`.
func TestExtraDeclaredFields(t *testing.T) {
	entry, errs := bases.BindEntryWithDate(
		parse(t, "date: 2020\ntitle: Some title\nunknown: 1\n"),
		bases.DateSpec([]binder.Field{{Name: "title"}}), nil, schemaerr.SourceMain,
	)
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if _, ok := entry.Field("title"); !ok {
		t.Error("title did not bind as a declared field")
	}
	if _, ok := entry.Extra("title"); ok {
		t.Error("title was retained as an extra key, want it declared")
	}
	if _, ok := entry.Extra("unknown"); !ok {
		t.Error("unknown was not retained")
	}
}

// Spec §3.70 — the base contributes exactly one field.
func TestDateSpecPutsOwnFieldsFirst(t *testing.T) {
	got := bases.FieldNames(bases.DateSpec(nil))
	if len(got) != 1 || got[0] != "date" {
		t.Errorf("date field names = %v, want [date]", got)
	}
}
