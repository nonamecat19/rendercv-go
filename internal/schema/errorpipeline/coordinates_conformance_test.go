//go:build conformance

package errorpipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

const upstreamTestdata = "../../../third_party/rendercv/tests/schema/testdata/test_pydantic_error_handling"

type expectedRecord struct {
	SchemaLocation []string `yaml:"schema_location"`
	YamlLocation   [][]int  `yaml:"yaml_location"`
	Message        string   `yaml:"message"`
}

// The coordinate resolver, diffed against upstream's own 25-record fixture.
//
// `expected_errors.yaml` records what upstream reports for `wrong_input.yaml`,
// coordinates included, so it is a ready-made oracle for step 10: walk the same
// document with the same location and the numbers must match. That makes this
// the only test in the iteration that checks the two coordinate formulas on real
// paths rather than on shaped fixtures.
//
// It exercises the `missing` truncation too — the fixture's two missing
// `EducationEntry` fields both report the enclosing mapping — because the
// truncation is applied here from the recorded code rather than assumed.
func TestCoordinatesMatchTheUpstreamFixture(t *testing.T) {
	document, err := yamlreader.ReadFile(filepath.Join(upstreamTestdata, "wrong_input.yaml"))
	if err != nil {
		t.Fatalf("reading wrong_input.yaml: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(upstreamTestdata, "expected_errors.yaml"))
	if err != nil {
		t.Fatalf("reading expected_errors.yaml: %v", err)
	}
	var fixture struct {
		ExpectedErrors []expectedRecord `yaml:"expected_errors"`
	}
	if err := yaml.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parsing expected_errors.yaml: %v", err)
	}
	if len(fixture.ExpectedErrors) != 25 {
		t.Fatalf("the fixture has %d records, want 25", len(fixture.ExpectedErrors))
	}

	for i, want := range fixture.ExpectedErrors {
		// The fixture records the final location and the final message, not the
		// code, so the truncation is inferred from the one message that means an
		// absent key. That is the same literal comparison step 10 makes, applied
		// to the only signal the fixture carries.
		code := schemaerr.Code("other")
		if want.Message == "This field is required." {
			code = "missing"
		}

		got, err := resolveCoordinates(document, coordinatePath(want.SchemaLocation, code))
		if err != nil {
			t.Errorf("record %d %v: %v", i, want.SchemaLocation, err)
			continue
		}
		if got == nil {
			t.Errorf("record %d %v: no coordinates", i, want.SchemaLocation)
			continue
		}

		if len(want.YamlLocation) != 2 || len(want.YamlLocation[0]) != 2 ||
			len(want.YamlLocation[1]) != 2 {
			t.Errorf("record %d: fixture coordinates are malformed: %v", i, want.YamlLocation)
			continue
		}
		wantStart, wantEnd := want.YamlLocation[0], want.YamlLocation[1]

		if got.Start.Line != wantStart[0] || got.Start.Column != wantStart[1] ||
			got.End.Line != wantEnd[0] || got.End.Column != wantEnd[1] {
			t.Errorf("record %d %v:\n  got  [[%d %d] [%d %d]]\n  want %v",
				i, want.SchemaLocation,
				got.Start.Line, got.Start.Column, got.End.Line, got.End.Column,
				want.YamlLocation)
		}
	}
}
