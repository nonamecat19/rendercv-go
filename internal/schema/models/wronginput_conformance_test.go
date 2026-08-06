//go:build conformance

package models_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

const upstreamTestdata = "../../../third_party/rendercv/tests/schema/testdata/test_pydantic_error_handling"

// expectedError is one row of upstream's own expected-errors fixture.
type expectedError struct {
	SchemaLocation []string `yaml:"schema_location"`
	Message        string   `yaml:"message"`
}

type expectedErrors struct {
	Errors []expectedError `yaml:"expected_errors"`
}

// messageToCode maps upstream's rendered message to the code that produced it.
// The fixture records messages rather than codes, and iteration 4 owns the
// message rewriting (spec §9) — "This field is required." against our "Field
// required" is the same `missing`. So this test compares locations and codes and
// uses the message only to identify the code.
var messageToCode = map[string]schemaerr.Code{
	"RenderCV couldn't match this section with any entry types. Please check the entries and make sure they are provided correctly.": "rendercv_other_error",
	"This field is required.":                                   "missing",
	"The month must be between 1 and 12.":                       "value_error",
	"The day is out of range for the month.":                    "value_error",
	"Input should be a valid string.":                           "string_type",
	"This field should contain a list of items but it doesn't.": "list_type",
}

func codeFor(message string) (schemaerr.Code, bool) {
	if code, ok := messageToCode[message]; ok {
		return code, true
	}
	// The two interpolated messages cannot be table keys.
	switch {
	case strings.HasPrefix(message, "There are problems with the entries."):
		return "rendercv_other_error", true
	case strings.HasPrefix(message, "`start_date` cannot be after `end_date`."):
		return "value_error", true
	}
	return "", false
}

// flatten walks the error tree depth-first, which is the order upstream's flat
// fixture lists them in: each section wrapper immediately followed by its own
// children.
func flatten(errs []schemaerr.ValidationError, out *[]schemaerr.ValidationError) {
	for _, err := range errs {
		*out = append(*out, err)
		flatten(err.Children, out)
	}
}

// Spec 003 §5.10, §5.13, §5.14, §9 — the entry-level half of upstream's own
// error fixture, reproduced from its own input file.
//
// Locations and codes only. The messages in `expected_errors.yaml` are RenderCV's
// rewrites of pydantic's text, which is iteration 4's work (spec §9); comparing
// them here would fail for a reason this iteration is not responsible for.
//
// TODO(iteration-4): spec §9 — compare `message` too, and drop messageToCode.
func TestWrongInputEntryErrors(t *testing.T) {
	inputPath := filepath.Join(upstreamTestdata, "wrong_input.yaml")
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("reading %s: %v — is the submodule initialized? `just setup`", inputPath, err)
	}

	node, err := yamlreader.ReadString(string(raw))
	if err != nil {
		t.Fatalf("reading %s: %v", inputPath, err)
	}

	_, errs := models.Validate(node, &valctx.ValidationContext{}, schemaerr.SourceMain)

	var flat []schemaerr.ValidationError
	flatten(errs, &flat)

	// Only the sections subtree is iteration 3's. `design`, `locale`, `settings`
	// and the social-network rows belong to later iterations.
	var got []string
	for _, err := range flat {
		path := strings.Join(err.SchemaLocation, ".")
		if !strings.HasPrefix(path, "cv.sections.") {
			continue
		}
		got = append(got, path+" | "+string(err.Code))
	}

	expectedPath := filepath.Join(upstreamTestdata, "expected_errors.yaml")
	expectedRaw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("reading %s: %v", expectedPath, err)
	}
	var expected expectedErrors
	if err := yaml.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatalf("parsing %s: %v", expectedPath, err)
	}

	var want []string
	for _, row := range expected.Errors {
		path := strings.Join(row.SchemaLocation, ".")
		if !strings.HasPrefix(path, "cv.sections.") {
			continue
		}
		code, ok := codeFor(row.Message)
		if !ok {
			t.Fatalf("no code mapped for upstream message %q; the fixture changed", row.Message)
		}
		want = append(want, path+" | "+string(code))
	}

	if len(want) == 0 {
		t.Fatal("no section rows found in the upstream fixture; the parse is wrong")
	}

	if len(got) != len(want) {
		t.Fatalf("produced %d section errors, want %d\n got: %s\nwant: %s",
			len(got), len(want), strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("error %d = %q, want %q", i, got[i], want[i])
		}
	}

	// `welcome_to_rendercv_tests_5` supplies `date: "Fall 2025"`, a valid
	// arbitrary date, so upstream reports nothing for it. A regression that
	// started rejecting free-text dates would add a row the fixture does not have,
	// and the length check above would catch it — but name it explicitly, because
	// it is the one section in this file that is meant to pass.
	for _, line := range got {
		if strings.Contains(line, "welcome_to_rendercv_tests_5") {
			t.Errorf("reported %q, but `Fall 2025` is a valid arbitrary date", line)
		}
	}
}
