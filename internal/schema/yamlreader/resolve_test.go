package yamlreader_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

type scalarRow struct {
	Raw   string `json:"raw"`
	Style string `json:"style"`
	Kind  string `json:"kind"`
}

// Scalar resolution, diffed against upstream's own reader.
//
// This table used to be written by hand on the Go side, which meant it stated
// what the port does and called it what upstream does — the same shape of
// mistake that let iteration 3 ship three wrong error codes behind a green
// suite. `tools/scalarprobe` now feeds each candidate through
// `rendercv.schema.yaml_reader.read_yaml` and records the Python type that
// comes back, so every expectation here is measured.
//
// The fixture is committed, so the test runs without the submodule; regenerate
// it with `go run ./tools/scalarprobe`.
func TestResolveScalar(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "scalars", "scalars.json"))
	if err != nil {
		t.Fatalf("reading the scalar fixture: %v", err)
	}
	var rows []scalarRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("parsing the scalar fixture: %v", err)
	}
	if len(rows) < 150 {
		t.Fatalf("the scalar corpus has %d rows, want the full set — has"+
			" tools/scalarprobe lost entries?", len(rows))
	}

	// Every kind the port can produce must appear, or the corpus could pass by
	// only ever exercising strings.
	seen := map[string]bool{}

	for _, row := range rows {
		style, ok := styleFromName(row.Style)
		if !ok {
			t.Errorf("fixture row %+v has an unknown style", row)
			continue
		}
		want, ok := kindFromName(row.Kind)
		if !ok {
			t.Errorf("fixture row %+v has an unknown kind", row)
			continue
		}
		seen[row.Kind] = true

		t.Run(row.Style+"/"+row.Raw, func(t *testing.T) {
			if got := yamlreader.ResolveScalar(row.Raw, style); got != want {
				t.Errorf("ResolveScalar(%q, %s) = %v, want %v",
					row.Raw, row.Style, got, want)
			}
		})
	}

	for _, kind := range []string{"null", "bool", "int", "float", "string"} {
		if !seen[kind] {
			t.Errorf("no fixture row resolves to %s", kind)
		}
	}
}

func styleFromName(name string) (yamldoc.ScalarStyle, bool) {
	switch name {
	case "plain":
		return yamldoc.StylePlain, true
	case "single":
		return yamldoc.StyleSingleQuoted, true
	case "double":
		return yamldoc.StyleDoubleQuoted, true
	default:
		return 0, false
	}
}

func kindFromName(name string) (yamldoc.Kind, bool) {
	switch name {
	case "null":
		return yamldoc.KindNull, true
	case "bool":
		return yamldoc.KindBool, true
	case "int":
		return yamldoc.KindInt, true
	case "float":
		return yamldoc.KindFloat, true
	case "string":
		return yamldoc.KindString, true
	default:
		return 0, false
	}
}
