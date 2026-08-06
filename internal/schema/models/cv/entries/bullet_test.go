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

var bulletReference = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func bulletLocation() []string {
	return []string{"cv", "sections", "bullets", "0"}
}

func validateBullet(t *testing.T, src string) (*entries.BulletEntry, []schemaerr.ValidationError) {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return entries.ValidateBulletEntry(node, bulletLocation(), schemaerr.SourceMain, bulletReference)
}

// Spec §3.4 — the field order, positionally: `bullet` and nothing else. BaseEntry
// contributes no fields, so there is no `date` (spec §5.19).
func TestBulletDescriptorFields(t *testing.T) {
	descriptor := entries.BulletDescriptor()
	if descriptor.Name != "BulletEntry" {
		t.Errorf("name = %q, want %q", descriptor.Name, "BulletEntry")
	}

	want := []string{"bullet"}
	got := descriptor.Fields
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fields = %v, want %v", got, want)
		}
	}
}

// Spec §3.17 behavior 39, plan §7 hazard 1 — the templater-injected names are
// never declared fields, or they would leak into iteration 5's JSON schema.
func TestBulletDeclaresNoInjectedFields(t *testing.T) {
	for _, name := range []string{"main_column", "date_and_location_column", "degree_column"} {
		for _, field := range entries.BulletDescriptor().Fields {
			if field == name {
				t.Errorf("fields = %v, want %q not declared", entries.BulletDescriptor().Fields, name)
			}
		}
	}
}

// Spec §5.17 — the conftest fixture (tests/schema/models/cv/conftest.py:60-62)
// validates with zero errors and is retained as written.
func TestBulletFixtureValidates(t *testing.T) {
	entry, errs := validateBullet(t, "bullet: This is a bullet entry.\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Bullet == nil {
		t.Fatalf("bullet = nil, want it bound")
	}
	if entry.Bullet.Raw != "This is a bullet entry." {
		t.Errorf("bullet = %q, want %q", entry.Bullet.Raw, "This is a bullet entry.")
	}
}

// Spec §5.11 — an unknown key is retained and readable
// (tests/schema/models/cv/test_section.py:63-83).
func TestBulletExtraAttributeRetained(t *testing.T) {
	entry, errs := validateBullet(t, "bullet: This is a bullet entry.\nextra_attribute: extra value\n")
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

// Spec §4.3 — the required field missing.
func TestBulletMissingField(t *testing.T) {
	_, errs := validateBullet(t, "other: value\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != binder.CodeMissing {
		t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeMissing)
	}
	want := append(bulletLocation(), "bullet")
	got := errs[0].SchemaLocation
	if len(got) != len(want) {
		t.Fatalf("schema location = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("schema location = %v, want %v", got, want)
		}
	}
}

// Spec §5.7 — a required field written null is a type error, not a missing
// field: the key is present and only its value is wrong.
func TestBulletNullFieldIsStringType(t *testing.T) {
	entry, errs := validateBullet(t, "bullet:\n")
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if errs[0].Code != binder.CodeStringType {
		t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeStringType)
	}
	if entry.Bullet != nil {
		t.Errorf("bullet = %+v, want nil for a null value", entry.Bullet)
	}
}

// Spec §5.19 — a `date` on a BulletEntry is an unknown key, not a date, and is
// not validated as one.
func TestBulletDateIsAnUnknownKey(t *testing.T) {
	entry, errs := validateBullet(t, "bullet: A bullet.\ndate: not-a-date\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	value, ok := entry.Extra("date")
	if !ok {
		t.Fatalf("extra keys = %v, want date retained as an unknown key", entry.ExtraKeys())
	}
	if value.Kind != yamldoc.KindString || value.Raw != "not-a-date" {
		t.Errorf("date = %+v, want the string %q", value, "not-a-date")
	}
}
