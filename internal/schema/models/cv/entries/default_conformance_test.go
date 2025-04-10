//go:build conformance

package entries_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

// fieldOrders is testdata/field_orders.json, generated from the vendored Python
// by tools/entryprobe (tasks 003 T5). It is the authority for three orderings
// this package cannot derive on its own: pydantic's field order per model, the
// `EntryModel` union order, and the characteristic-field table computed at
// section.py:77.
type fieldOrders struct {
	AvailableEntryTypeNames []string `json:"available_entry_type_names"`
	FieldOrders             []struct {
		Type   string   `json:"type"`
		Fields []string `json:"fields"`
	} `json:"field_orders"`
	CharacteristicEntryFields []struct {
		Type   string   `json:"type"`
		Fields []string `json:"fields"`
	} `json:"characteristic_entry_fields"`
}

func loadFieldOrders(t *testing.T) fieldOrders {
	t.Helper()
	path := filepath.Join("testdata", "field_orders.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — regenerate it with `just entryprobe`", path, err)
	}
	var out fieldOrders
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(out.FieldOrders) == 0 {
		t.Fatalf("%s carries no field orders; the fixture is empty", path)
	}
	return out
}

// Spec 003 §3.1 behaviors 2-3, §3.2, §6.1. The registry's order is the
// discrimination order, so it is asserted positionally, never as a set.
func TestDefaultRegistryMatchesUpstreamFieldOrders(t *testing.T) {
	fixture := loadFieldOrders(t)
	got := entries.Default().Descriptors()

	if len(got) != len(fixture.FieldOrders) {
		t.Fatalf("registry has %d descriptors, want %d", len(got), len(fixture.FieldOrders))
	}

	for i, want := range fixture.FieldOrders {
		if string(got[i].Name) != want.Type {
			t.Errorf("descriptor %d is %q, want %q — the registry is out of union order", i, got[i].Name, want.Type)
			continue
		}
		if len(got[i].Fields) != len(want.Fields) {
			t.Errorf("%s fields = %v, want %v", want.Type, got[i].Fields, want.Fields)
			continue
		}
		for j := range want.Fields {
			if got[i].Fields[j] != want.Fields[j] {
				t.Errorf("%s fields = %v, want %v", want.Type, got[i].Fields, want.Fields)
				break
			}
		}
	}
}

// Spec 003 §3.16 behavior 34. Names carries the eight model names in union order
// followed by the literal TextEntry, which is a type without a model
// (section.py:37-39).
func TestDefaultRegistryNames(t *testing.T) {
	fixture := loadFieldOrders(t)
	got := entries.Default().Names()

	if len(got) != len(fixture.AvailableEntryTypeNames) {
		t.Fatalf("names = %v, want %v", got, fixture.AvailableEntryTypeNames)
	}
	for i, want := range fixture.AvailableEntryTypeNames {
		if string(got[i]) != want {
			t.Fatalf("names = %v, want %v", got, fixture.AvailableEntryTypeNames)
		}
	}
}

// Spec 003 §3.16 behavior 34, §6.2. A field declared by more than one type is
// common and cannot discriminate; what is left per type is its characteristic
// set. The fixture sorts each set, so this compares sorted.
func TestDefaultRegistryCharacteristicFields(t *testing.T) {
	fixture := loadFieldOrders(t)
	got := entries.Default().Characteristic()

	if len(got) != len(fixture.CharacteristicEntryFields) {
		t.Fatalf("characteristic table has %d types, want %d", len(got), len(fixture.CharacteristicEntryFields))
	}

	for _, want := range fixture.CharacteristicEntryFields {
		set, ok := got[entries.TypeName(want.Type)]
		if !ok {
			t.Errorf("no characteristic fields for %s", want.Type)
			continue
		}
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		sort.Strings(names)

		if len(names) != len(want.Fields) {
			t.Errorf("%s characteristic = %v, want %v", want.Type, names, want.Fields)
			continue
		}
		for i := range want.Fields {
			if names[i] != want.Fields[i] {
				t.Errorf("%s characteristic = %v, want %v", want.Type, names, want.Fields)
				break
			}
		}
	}
}
