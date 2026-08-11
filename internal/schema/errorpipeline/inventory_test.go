package errorpipeline

import "testing"

// TestDictionaryInventory is spec 013 §3.8 behavior 54's second `schema/` data
// file: `error_dictionary.yaml`, thirteen rows, shipped inside the wheel
// upstream and compiled into the binary here.
//
// It is the untagged half of the packaging inventory — `internal/cli`'s
// `packaging_test.go` cannot see this table, and
// `dictionary_conformance_test.go`, which diffs it against the submodule row by
// row, only runs under the conformance tag. This asserts the count alone, so a
// row deleted by accident fails an ordinary `go test`.
func TestDictionaryInventory(t *testing.T) {
	if len(dictionary) != 13 {
		t.Errorf("dictionary rows = %d, want 13", len(dictionary))
	}
}
