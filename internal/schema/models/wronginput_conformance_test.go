//go:build conformance

package models_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

const upstreamTestdata = "../../../third_party/rendercv/tests/schema/testdata/test_pydantic_error_handling"

// expectedError is one row of upstream's own expected-errors fixture, with every
// member it records.
type expectedError struct {
	SchemaLocation []string `yaml:"schema_location"`
	Message        string   `yaml:"message"`
	Input          string   `yaml:"input"`
	YamlLocation   [][]int  `yaml:"yaml_location"`
	YamlSource     string   `yaml:"yaml_source"`
}

type expectedErrors struct {
	Errors []expectedError `yaml:"expected_errors"`
}

// The full differential: upstream's own fixture, member by member, in order,
// with an equal-length assertion.
//
// This is the strongest mechanical Axis-4 gate available and the only one — the
// CLI goldens pin the table's layout, not the messages. It is a port of
// `tests/schema/test_pydantic_error_handling.py:19-54`, which compares whole
// records with `zip(..., strict=True)`.
//
// **All five members.** Weakening it to locations, codes and messages would
// leave 50 of the fixture's 388 coordinate pairs unchecked, in the one file that
// is the contract (spec 004 §7.2).
func TestWrongInputDifferential(t *testing.T) {
	got := parseWrongInput(t, nil)
	want := loadExpectedErrors(t)

	if len(got) != len(want) {
		t.Fatalf("produced %d records, want %d:\n  got  %v\n  want %v",
			len(got), len(want), locationsOf(got), expectedLocationsOf(want))
	}

	for i := range want {
		assertRecord(t, i, got[i], want[i])
	}
}

// Spec 004 §3.12 behavior 40: with overlays supplied, a record rooted at an
// overlay key carries that overlay's source and resolves its coordinates against
// that document; everything else keeps the main file.
//
// The mixed case is the one worth running — a per-record assertion, not a
// blanket one, which is how upstream's own test at `:151-186` checks it.
func TestWrongInputOverlaySources(t *testing.T) {
	designDoc, err := yamlreader.ReadString("design:\n  theme: not_a_valid_theme\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	overlays := map[schemaerr.OverlayKey]*yamldoc.Node{schemaerr.OverlayDesign: designDoc}

	for _, record := range parseWrongInput(t, overlays) {
		wantSource := schemaerr.SourceMain
		if len(record.SchemaLocation) > 0 && record.SchemaLocation[0] == "design" {
			wantSource = schemaerr.SourceDesign
		}
		if record.YamlSource != wantSource {
			t.Errorf("%s: source = %q, want %q",
				strings.Join(record.SchemaLocation, "."), record.YamlSource, wantSource)
		}
	}
}

// parseWrongInput runs upstream's own bad input through the whole port —
// validation and then the error pipeline — and returns the final records.
func parseWrongInput(
	t *testing.T,
	overlays map[schemaerr.OverlayKey]*yamldoc.Node,
) []schemaerr.ValidationError {
	t.Helper()

	inputPath := filepath.Join(upstreamTestdata, "wrong_input.yaml")
	source, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("reading %s: %v — is the submodule initialized? `just setup`", inputPath, err)
	}
	document, err := yamlreader.ReadString(string(source))
	if err != nil {
		t.Fatalf("reading %s: %v", inputPath, err)
	}

	absolute, err := filepath.Abs(inputPath)
	if err != nil {
		t.Fatalf("resolving %s: %v", inputPath, err)
	}

	_, raw := models.Validate(
		document, &valctx.ValidationContext{InputFilePath: absolute}, schemaerr.SourceMain,
	)
	final, err := errorpipeline.Parse(raw, document, overlays)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return final
}

func loadExpectedErrors(t *testing.T) []expectedError {
	t.Helper()

	path := filepath.Join(upstreamTestdata, "expected_errors.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var fixture expectedErrors
	if err := yaml.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(fixture.Errors) != 25 {
		t.Fatalf("the fixture has %d records, want 25", len(fixture.Errors))
	}
	return fixture.Errors
}

func assertRecord(t *testing.T, index int, got schemaerr.ValidationError, want expectedError) {
	t.Helper()

	location := strings.Join(want.SchemaLocation, ".")
	t.Run(location, func(t *testing.T) {
		if strings.Join(got.SchemaLocation, ".") != location {
			t.Errorf("record %d schema_location = %v, want %v",
				index, got.SchemaLocation, want.SchemaLocation)
		}
		if got.Message != want.Message {
			t.Errorf("record %d message =\n  %q\nwant\n  %q", index, got.Message, want.Message)
		}
		if got.Input != want.Input {
			t.Errorf("record %d input = %q, want %q", index, got.Input, want.Input)
		}
		if string(got.YamlSource) != want.YamlSource {
			t.Errorf("record %d yaml_source = %q, want %q", index, got.YamlSource, want.YamlSource)
		}
		assertCoordinates(t, index, got, want)
	})
}

func assertCoordinates(t *testing.T, index int, got schemaerr.ValidationError, want expectedError) {
	t.Helper()

	if len(want.YamlLocation) != 2 || len(want.YamlLocation[0]) != 2 || len(want.YamlLocation[1]) != 2 {
		t.Errorf("record %d: the fixture's coordinates are malformed: %v", index, want.YamlLocation)
		return
	}
	if got.YamlLocation == nil {
		t.Errorf("record %d yaml_location is absent, want %v", index, want.YamlLocation)
		return
	}

	start, end := want.YamlLocation[0], want.YamlLocation[1]
	if got.YamlLocation.Start.Line != start[0] || got.YamlLocation.Start.Column != start[1] ||
		got.YamlLocation.End.Line != end[0] || got.YamlLocation.End.Column != end[1] {
		t.Errorf("record %d yaml_location =\n  [[%d %d] [%d %d]]\nwant\n  %v",
			index,
			got.YamlLocation.Start.Line, got.YamlLocation.Start.Column,
			got.YamlLocation.End.Line, got.YamlLocation.End.Column,
			want.YamlLocation)
	}
}

func locationsOf(records []schemaerr.ValidationError) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, strings.Join(record.SchemaLocation, "."))
	}
	return out
}

func expectedLocationsOf(records []expectedError) []string {
	out := make([]string, 0, len(records))
	for _, record := range records {
		out = append(out, strings.Join(record.SchemaLocation, "."))
	}
	return out
}
