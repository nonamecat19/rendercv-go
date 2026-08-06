package cv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// TextEntry has no model: it is the bare Python `str`, mapped to a name only in
// the discrimination logic (section.py:23-24, :162-165). These tests pin the
// ninth type's whole surface, which is why there is no textentry.go beside them
// (spec 003 §3.14, plan 003 §3.2).

// Spec 003 §3.14 behavior 27, §4 — a string entry is always valid: its
// validator is the identity, so a section of strings produces no errors.
func TestTextEntryValidatesWithZeroErrors(t *testing.T) {
	for _, src := range []string{
		"  - just text\n",
		"  - first\n  - second\n",
		"  - \"\"\n",
	} {
		entryType, errs := cv.ValidateSection(
			section(t, src),
			fixtureRegistry(),
			[]string{"cv", "sections", "x"},
			schemaerr.SourceMain,
			time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC),
		)
		if len(errs) != 0 {
			t.Errorf("ValidateSection(%q) errs = %+v, want none", src, errs)
		}
		if entryType != cv.TextEntry {
			t.Errorf("ValidateSection(%q) entry type = %q, want %q", src, entryType, cv.TextEntry)
		}
	}
}

// Spec 003 §3.14 behaviors 28-29 — a string never reaches characteristic-field
// discrimination; the string branch names it with the literal `TextEntry`
// (section.py:38, :164).
func TestTextEntryName(t *testing.T) {
	if cv.TextEntry != "TextEntry" {
		t.Errorf("cv.TextEntry = %q, want %q", cv.TextEntry, "TextEntry")
	}

	got, err := cv.InferEntryType(parseValue(t, "just text"), fixtureRegistry())
	if err != nil {
		t.Fatalf("InferEntryType: %v", err)
	}
	if got != "TextEntry" {
		t.Errorf("InferEntryType = %q, want %q", got, "TextEntry")
	}
}

// sectionModelName reproduces upstream's section model naming
// (section.py:106-110): `str` is the special case `SectionWithTextEntries`, and
// a model's name is `"SectionWith"` plus its class name with `Entry` replaced by
// `Entries`. Upstream's `str.replace` is unbounded, but each of the eight class
// names contains exactly one `Entry`, so replacing the first is equivalent.
//
// It lives in the test because the port does not reproduce the dynamic-model
// machinery these names come from; they are pinned, not computed at runtime
// (spec 003 §3.15 behavior 33).
func sectionModelName(entryTypeName string) string {
	if entryTypeName == "TextEntry" {
		return "SectionWithTextEntries"
	}
	return "SectionWith" + strings.Replace(entryTypeName, "Entry", "Entries", 1)
}

// Spec 003 §3.15 behaviors 30-31 — the nine section model names, as upstream's
// own test asserts them (tests/schema/models/cv/test_section.py:19-60).
func TestSectionModelNames(t *testing.T) {
	tests := []struct {
		entryType string
		want      string
	}{
		{"OneLineEntry", "SectionWithOneLineEntries"},
		{"NormalEntry", "SectionWithNormalEntries"},
		{"ExperienceEntry", "SectionWithExperienceEntries"},
		{"EducationEntry", "SectionWithEducationEntries"},
		{"PublicationEntry", "SectionWithPublicationEntries"},
		{"BulletEntry", "SectionWithBulletEntries"},
		{"NumberedEntry", "SectionWithNumberedEntries"},
		{"ReversedNumberedEntry", "SectionWithReversedNumberedEntries"},
		{"TextEntry", "SectionWithTextEntries"},
	}
	for _, tt := range tests {
		t.Run(tt.entryType, func(t *testing.T) {
			if got := sectionModelName(tt.entryType); got != tt.want {
				t.Errorf("sectionModelName(%q) = %q, want %q", tt.entryType, got, tt.want)
			}
		})
	}
}

// Spec 003 §3.17 behavior 40 — the nine snake-case names the theme's per-type
// template sub-model is selected by
// (renderer/templater/entry_templates_from_input.py:120, :125). The transform is
// `entry_type_to_snake_case_pattern` (entries/bases/entry.py:8).
func TestEntryTypeSnakeCaseNames(t *testing.T) {
	tests := []struct {
		entryType string
		want      string
	}{
		{"OneLineEntry", "one_line_entry"},
		{"NormalEntry", "normal_entry"},
		{"ExperienceEntry", "experience_entry"},
		{"EducationEntry", "education_entry"},
		{"PublicationEntry", "publication_entry"},
		{"BulletEntry", "bullet_entry"},
		{"NumberedEntry", "numbered_entry"},
		{"ReversedNumberedEntry", "reversed_numbered_entry"},
		{"TextEntry", "text_entry"},
	}
	for _, tt := range tests {
		t.Run(tt.entryType, func(t *testing.T) {
			if got := bases.EntryTypeInSnakeCase(tt.entryType); got != tt.want {
				t.Errorf("EntryTypeInSnakeCase(%q) = %q, want %q", tt.entryType, got, tt.want)
			}
		})
	}
}
