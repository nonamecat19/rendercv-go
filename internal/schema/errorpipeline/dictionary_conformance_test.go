//go:build conformance

package errorpipeline

import (
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

const dictionaryPath = "../../../third_party/rendercv/src/rendercv/schema/error_dictionary.yaml"

// The dictionary is compiled-in Go data, so nothing but this test stands between
// the port and a silently drifted message.
//
// It is Go source rather than an embedded copy of the YAML because the file
// lives in the submodule, which is not present at runtime — embedding would mean
// transcribing it into the Go tree, which is the same risk with an extra step.
// The check is therefore a diff against the submodule itself, read with the
// project's own reader, key for key and value for value **in file order**.
//
// Two rows are traps rather than data (spec 004 §3.4 behavior 13, plan §3):
//
//   - Rows 3 and 4's keys contain **doubled** backslashes, and the scalars are
//     plain, so YAML performs no escape processing and the keys literally carry
//     two. Pydantic's messages carry one, which is why both rows are dead.
//     A porter who "fixes" the escaping produces a *live* row and breaks parity
//     in the opposite direction from the obvious mistake.
//   - Row 13's key and value are the only quoted scalars in the file, so the
//     reader must carry them through without changing the bytes.
//
// TestDictionaryTrapsAreExercised below fails if either trap stops being
// covered, so this test cannot quietly degrade into a length check.
func TestDictionaryMatchesTheSubmodule(t *testing.T) {
	want := readDictionary(t)

	if len(dictionary) != len(want) {
		t.Fatalf("dictionary has %d rows, the submodule has %d", len(dictionary), len(want))
	}

	for i := range want {
		if dictionary[i].Old != want[i].Old {
			t.Errorf("row %d key:\n  got  %q\n  want %q", i+1, dictionary[i].Old, want[i].Old)
		}
		if dictionary[i].New != want[i].New {
			t.Errorf("row %d value:\n  got  %q\n  want %q", i+1, dictionary[i].New, want[i].New)
		}
	}
}

// The two traps, stated as properties of the submodule data rather than of the
// Go slice. If upstream ever single-escapes rows 3 and 4, or unquotes row 13,
// the test above would still pass while the reasoning behind five dead rows
// silently changed — so the traps are asserted where they live.
func TestDictionaryTrapsAreExercised(t *testing.T) {
	rows := readDictionary(t)
	if len(rows) < 13 {
		t.Fatalf("the submodule dictionary has %d rows, want at least 13", len(rows))
	}

	for _, row := range []int{3, 4} {
		if !containsDoubleBackslash(rows[row-1].Old) {
			t.Errorf("row %d key %q no longer carries a doubled backslash — it may now"+
				" be reachable, and spec 004 §3.4 behavior 13 needs re-measuring",
				row, rows[row-1].Old)
		}
	}

	const colorKey = "value is not a valid color"
	if rows[12].Old != colorKey {
		t.Errorf("row 13 key = %q, want %q — the quoted scalar did not survive the reader",
			rows[12].Old, colorKey)
	}
}

// readDictionary parses the submodule file with the project's reader and returns
// its pairs in file order.
func readDictionary(t *testing.T) []dictionaryRow {
	t.Helper()

	path, err := filepath.Abs(dictionaryPath)
	if err != nil {
		t.Fatalf("resolving %s: %v", dictionaryPath, err)
	}
	node, err := yamlreader.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if node == nil || node.Kind != yamldoc.KindMapping {
		t.Fatalf("%s is not a mapping", path)
	}

	rows := make([]dictionaryRow, 0, len(node.Items))
	for _, item := range node.Items {
		if item.Value == nil {
			t.Fatalf("row %q has no value", item.Key)
		}
		rows = append(rows, dictionaryRow{Old: item.Key, New: item.Value.Raw})
	}
	return rows
}

func containsDoubleBackslash(value string) bool {
	for i := 0; i+1 < len(value); i++ {
		if value[i] == '\\' && value[i+1] == '\\' {
			return true
		}
	}
	return false
}
