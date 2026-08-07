//go:build conformance

package entries_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// dumps is testdata/dumps.json, generated from the vendored Python by
// tools/dumpprobe (tasks 009 T1). It is the authority for what
// `model_dump(exclude_none=True)` produces, which is the dictionary
// `render_entry_templates` turns into placeholders.
type dumps struct {
	Dumps []struct {
		Type  string         `json:"type"`
		Case  string         `json:"case"`
		Input map[string]any `json:"input"`
		Dump  map[string]any `json:"dump"`
	} `json:"dumps"`
}

func loadDumps(t *testing.T) dumps {
	t.Helper()
	path := filepath.Join("testdata", "dumps.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v — regenerate it with `just dumpprobe`", path, err)
	}
	var out dumps
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(out.Dumps) == 0 {
		t.Fatalf("%s carries no dumps; the fixture is empty", path)
	}
	return out
}

// Spec 009 §0 and plan §2. The dump is compared **through JSON**, because that
// is the only common ground: the fixture came out of Python's `json.dump` and an
// integer there is a `float64` once Go's decoder has seen it. Round-tripping the
// Go side the same way compares like with like, and still separates `2023` from
// `"2023"` — which is the distinction the whole `YearOnly` mechanism exists for.
func TestDumpMatchesUpstream(t *testing.T) {
	fixture := loadDumps(t)

	for _, row := range fixture.Dumps {
		t.Run(row.Type+"/"+row.Case, func(t *testing.T) {
			node, err := yamlreader.ReadString(toYAML(t, row.Input))
			if err != nil {
				t.Fatalf("reading the case's YAML: %v", err)
			}

			got, _ := entries.Dump(node, entries.TypeName(row.Type))
			if !reflect.DeepEqual(throughJSON(t, got), throughJSON(t, row.Dump)) {
				t.Errorf("dump =\n%s\nwant\n%s", encode(t, got), encode(t, row.Dump))
			}
		})
	}
}

// The three fields whose dumped type is not a string, asserted separately from
// the value comparison above: JSON cannot tell `2000` from `2000.0`, and the
// port's `YearOnly` set is what carries the distinction into the renderer.
func TestDumpReportsIntegerDates(t *testing.T) {
	fixture := loadDumps(t)

	for _, row := range fixture.Dumps {
		t.Run(row.Type+"/"+row.Case, func(t *testing.T) {
			node, err := yamlreader.ReadString(toYAML(t, row.Input))
			if err != nil {
				t.Fatalf("reading the case's YAML: %v", err)
			}

			_, yearOnly := entries.Dump(node, entries.TypeName(row.Type))
			for _, field := range []string{"date", "start_date", "end_date"} {
				value, present := row.Dump[field]
				_, isNumber := value.(float64)
				want := present && isNumber
				if yearOnly[field] != want {
					t.Errorf("yearOnly[%q] = %v, want %v (dumped %#v)",
						field, yearOnly[field], want, value)
				}
			}
		})
	}
}

// toYAML writes the case's input back out as the YAML a user would have typed.
//
// **The whole-number floats have to become integers first.** The fixture's
// `start_date: 2000` arrived through Go's JSON decoder as `float64(2000)`, and
// marshalling that emits `2000.0` — a float to the reader, so the very
// distinction this fixture exists to pin would be destroyed by the harness that
// checks it.
func toYAML(t *testing.T, value map[string]any) string {
	t.Helper()
	narrowed := make(map[string]any, len(value))
	for key, item := range value {
		number, isNumber := item.(float64)
		if isNumber && number == float64(int(number)) {
			narrowed[key] = int(number)
			continue
		}
		narrowed[key] = item
	}

	raw, err := yaml.Marshal(narrowed)
	if err != nil {
		t.Fatalf("marshalling the case: %v", err)
	}
	return string(raw)
}

// throughJSON is the normalization the comparison rests on: it makes the Go
// side's numbers the same `float64` the fixture's are.
func throughJSON(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(encode(t, value)), &out); err != nil {
		t.Fatalf("re-decoding: %v", err)
	}
	return out
}

func encode(t *testing.T, value map[string]any) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	return string(raw)
}
