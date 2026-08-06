package entries_test

import (
	"sort"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

// Spec §3.1 behavior 2. The `EntryModel` union order (section.py:24-33), which is
// the discrimination order — deliberately not the alphabetical import order at
// section.py:11-18. Asserted positionally, because a set assertion would pass on
// a wrongly ordered registry and discrimination would silently resolve ambiguous
// entries to the wrong type.
func TestDefaultRegistryIsInUnionOrder(t *testing.T) {
	want := []entries.TypeName{
		"OneLineEntry",
		"NormalEntry",
		"ExperienceEntry",
		"EducationEntry",
		"PublicationEntry",
		"BulletEntry",
		"NumberedEntry",
		"ReversedNumberedEntry",
	}

	got := entries.Default().Descriptors()
	if len(got) != len(want) {
		t.Fatalf("registry has %d descriptors, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("descriptor %d = %q, want %q", i, got[i].Name, name)
		}
	}
}

// Spec §3.16 behavior 34 — the characteristic-field table, verified at runtime
// against section.py:77.
func TestDefaultCharacteristicTable(t *testing.T) {
	want := map[entries.TypeName][]string{
		"OneLineEntry":          {"details", "label"},
		"NormalEntry":           {"name"},
		"ExperienceEntry":       {"company", "position"},
		"EducationEntry":        {"area", "degree", "institution"},
		"PublicationEntry":      {"authors", "doi", "journal", "title", "url"},
		"BulletEntry":           {"bullet"},
		"NumberedEntry":         {"number"},
		"ReversedNumberedEntry": {"reversed_number"},
	}

	got := entries.Default().Characteristic()
	if len(got) != len(want) {
		t.Fatalf("characteristic table has %d types, want %d", len(got), len(want))
	}

	for name, wantFields := range want {
		set, ok := got[name]
		if !ok {
			t.Errorf("no characteristic fields for %s", name)
			continue
		}
		names := make([]string, 0, len(set))
		for field := range set {
			names = append(names, field)
		}
		sort.Strings(names)

		if len(names) != len(wantFields) {
			t.Errorf("%s characteristic = %v, want %v", name, names, wantFields)
			continue
		}
		for i := range wantFields {
			if names[i] != wantFields[i] {
				t.Errorf("%s characteristic = %v, want %v", name, names, wantFields)
				break
			}
		}
	}
}

// Spec §3.16 behavior 34 — the common set is exactly these six. `summary` is in
// it because two unrelated bases declare it: BaseEntryWithComplexFields
// (entry_with_complex_fields.py:110) and BasePublicationEntry
// (publication.py:23). A field that is characteristic of nothing cannot
// discriminate, which is why a lone `summary` matches no type at all.
func TestDefaultCommonFields(t *testing.T) {
	want := []string{"date", "end_date", "highlights", "location", "start_date", "summary"}

	registry := entries.Default()
	characteristic := registry.Characteristic()

	declared := map[string]struct{}{}
	for _, descriptor := range registry.Descriptors() {
		for _, field := range descriptor.Fields {
			declared[field] = struct{}{}
		}
	}
	for _, set := range characteristic {
		for field := range set {
			delete(declared, field)
		}
	}

	got := make([]string, 0, len(declared))
	for field := range declared {
		got = append(got, field)
	}
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("common fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("common fields = %v, want %v", got, want)
		}
	}
}

// Spec §5.15 — upstream's own discrimination table covers seven types
// (tests/schema/models/cv/test_section.py:19-60). `NumberedEntry` and
// `ReversedNumberedEntry` are absent from it, so the two rows upstream never
// asserts are pinned here: their characteristic fields are unique, so each must
// resolve to itself.
func TestDefaultDiscriminates(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want entries.TypeName
	}{
		{name: "one line", keys: []string{"label", "details"}, want: "OneLineEntry"},
		{name: "normal", keys: []string{"name", "date"}, want: "NormalEntry"},
		{name: "experience", keys: []string{"company", "position"}, want: "ExperienceEntry"},
		{name: "education", keys: []string{"institution", "area"}, want: "EducationEntry"},
		{name: "publication", keys: []string{"title", "authors"}, want: "PublicationEntry"},
		{name: "bullet", keys: []string{"bullet"}, want: "BulletEntry"},
		{name: "numbered", keys: []string{"number"}, want: "NumberedEntry"},
		{
			name: "reversed numbered",
			keys: []string{"reversed_number"},
			want: "ReversedNumberedEntry",
		},
		{
			// `degree` alone is enough: it is characteristic of EducationEntry.
			name: "a lone characteristic field is enough",
			keys: []string{"degree"},
			want: "EducationEntry",
		},
	}

	registry := entries.Default()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := registry.Discriminate(test.keys)
			if !ok {
				t.Fatalf("Discriminate(%v) matched nothing, want %q", test.keys, test.want)
			}
			if got != test.want {
				t.Errorf("Discriminate(%v) = %q, want %q", test.keys, got, test.want)
			}
		})
	}
}

// Spec §5.20 — only common fields means no characteristic field matched, so no
// type does. The caller turns this into spec 002 §4.9.
func TestDefaultDiscriminatesNothingOnCommonFieldsAlone(t *testing.T) {
	registry := entries.Default()
	for _, keys := range [][]string{
		{"summary"},
		{"date"},
		{"start_date", "end_date", "location", "summary", "highlights", "date"},
		{},
	} {
		if got, ok := registry.Discriminate(keys); ok {
			t.Errorf("Discriminate(%v) = %q, want no match", keys, got)
		}
	}
}
