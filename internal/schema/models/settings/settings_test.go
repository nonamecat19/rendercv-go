package settings_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/settings"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func currentDateNode(t *testing.T, value string) *yamldoc.Node {
	t.Helper()
	doc, err := yamlreader.ReadString("current_date: " + value + "\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc.Items[0].Value
}

// Spec 004 §3.5 behaviors 18-19 and §4.13.
//
// The record's own message is irrelevant and the test says so: what matters is
// the location, because the pipeline's override fires on that alone. The final
// text is asserted through the pipeline, which is where §4.13 comes from.
func TestValidateCurrentDate(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		rejected bool
	}{
		{name: "the literal today", value: "today", rejected: false},
		{name: "an ISO date", value: "2025-01-01", rejected: false},
		{name: "upstream's typo", value: "todady", rejected: true},
		{name: "a partial date", value: "2025-01", rejected: true},
		{name: "prose", value: "the first of January", rejected: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := settings.ValidateCurrentDate(
				currentDateNode(t, test.value), []string{"settings"}, schemaerr.SourceMain,
			)
			if !test.rejected {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "settings.current_date" {
				t.Errorf("location = %q, want settings.current_date", got)
			}
		})
	}
}

// The override is what produces §4.13, and it fires on the location regardless
// of the raw message. Asserted end to end, on upstream's own typo, whose final
// text is `expected_errors.yaml:148`.
func TestCurrentDateMessageComesFromTheOverride(t *testing.T) {
	errs := settings.ValidateCurrentDate(
		currentDateNode(t, "todady"), []string{"settings"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	const want = "This is not a valid `current_date`! Please use YYYY-MM-DD format" +
		` or "today".`
	if final[0].Message != want {
		t.Errorf("final message =\n  %q\nwant\n  %q", final[0].Message, want)
	}
	if final[0].Input != "todady" {
		t.Errorf("input = %q, want the value as written", final[0].Input)
	}
}
