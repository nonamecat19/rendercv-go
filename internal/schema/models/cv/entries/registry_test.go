package entries_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

func fixtureRegistry() *entries.Registry {
	return entries.NewRegistry(
		entries.Descriptor{
			Name: "OneLineEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"details", "label",
			},
		},
		entries.Descriptor{
			Name: "NormalEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"name",
			},
		},
		entries.Descriptor{
			Name: "ExperienceEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"company", "position",
			},
		},
		entries.Descriptor{
			Name: "EducationEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"area", "degree", "institution",
			},
		},
		entries.Descriptor{
			Name: "PublicationEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"authors", "doi", "journal", "title", "url",
			},
		},
		entries.Descriptor{
			Name: "BulletEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"bullet",
			},
		},
		entries.Descriptor{
			Name: "NumberedEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"number",
			},
		},
		entries.Descriptor{
			Name: "ReversedNumberedEntry",
			Fields: []string{
				"date", "start_date", "end_date", "location", "summary", "highlights",
				"reversed_number",
			},
		},
	)
}

func TestCharacteristicFields(t *testing.T) {
	r := fixtureRegistry()
	chars := r.Characteristic()

	expected := map[entries.TypeName]map[string]struct{}{
		"OneLineEntry":          {"details": {}, "label": {}},
		"NormalEntry":           {"name": {}},
		"ExperienceEntry":       {"company": {}, "position": {}},
		"EducationEntry":        {"area": {}, "degree": {}, "institution": {}},
		"PublicationEntry":      {"authors": {}, "doi": {}, "journal": {}, "title": {}, "url": {}},
		"BulletEntry":           {"bullet": {}},
		"NumberedEntry":         {"number": {}},
		"ReversedNumberedEntry": {"reversed_number": {}},
	}

	for name, want := range expected {
		got, ok := chars[name]
		if !ok {
			t.Errorf("missing characteristic set for %s", name)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%s: got %d fields, want %d", name, len(got), len(want))
		}
		for f := range want {
			if _, ok := got[f]; !ok {
				t.Errorf("%s: missing field %q", name, f)
			}
		}
	}
}

func TestNames(t *testing.T) {
	r := fixtureRegistry()
	names := r.Names()
	if len(names) != 9 {
		t.Errorf("Names() = %d entries, want 9", len(names))
	}
	if names[len(names)-1] != "TextEntry" {
		t.Errorf("last name = %q, want TextEntry", names[len(names)-1])
	}
}

func TestDiscriminate(t *testing.T) {
	r := fixtureRegistry()

	tests := []struct {
		keys []string
		want entries.TypeName
		ok   bool
	}{
		{[]string{"details"}, "OneLineEntry", true},
		{[]string{"name"}, "NormalEntry", true},
		{[]string{"company"}, "ExperienceEntry", true},
		{[]string{"degree"}, "EducationEntry", true},
		{[]string{"authors"}, "PublicationEntry", true},
		{[]string{"bullet"}, "BulletEntry", true},
		{[]string{"number"}, "NumberedEntry", true},
		{[]string{"reversed_number"}, "ReversedNumberedEntry", true},
		{[]string{"unknown_field"}, "", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			got, ok := r.Discriminate(tt.keys)
			if ok != tt.ok || got != tt.want {
				t.Errorf("Discriminate(%v) = (%q, %v), want (%q, %v)", tt.keys, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDiscriminatePriority(t *testing.T) {
	r := fixtureRegistry()
	got, ok := r.Discriminate([]string{"details", "name"})
	if !ok || got != "OneLineEntry" {
		t.Errorf("Discriminate with details+name = (%q, %v), want (OneLineEntry, true)", got, ok)
	}
}
