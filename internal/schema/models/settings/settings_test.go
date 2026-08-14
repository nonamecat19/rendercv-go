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

func settingsNode(t *testing.T, document string) *yamldoc.Node {
	t.Helper()
	doc, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc
}

// Spec 004 §4.15 and its acceptance criterion, for the unknown-key record.
//
// **The Input Value column was empty on every settings unknown key.** The
// record left `Input` at its zero value, so the column rendered blank where
// upstream prints the offending value. Pydantic's `input` for `extra_forbidden`
// is the *value* of the unknown key — not the containing model — and
// `pydantic_error_handling.py:122-126` renders it `str(value)` unless it is a
// `dict` or a `list`, in which case `...`. Measured through upstream's own CLI
// on all nine value shapes below; eight of the nine disagreed.
func TestUnknownKeyEchoesItsValue(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{name: "a string", document: "bogus: hello\n", want: "hello"},
		{name: "an integer", document: "bogus: 7\n", want: "7"},
		{name: "a float", document: "bogus: 1.50\n", want: "1.5"},
		{name: "a bool", document: "bogus: true\n", want: "True"},
		{name: "a null", document: "bogus: null\n", want: "None"},
		{name: "an absent value", document: "bogus:\n", want: "None"},
		{name: "a sequence", document: "bogus:\n  - 1\n  - 2\n", want: "..."},
		{name: "a mapping", document: "bogus:\n  a: 1\n", want: "..."},
		{name: "an empty string", document: "bogus: ''\n", want: ""},
		{
			name:     "a long string",
			document: "bogus: '" + strings.Repeat("y", 90) + "'\n",
			want:     strings.Repeat("y", 90),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := settings.ValidateUnknownKeys(
				settingsNode(t, test.document), []string{"settings"}, schemaerr.SourceMain,
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "settings.bogus" {
				t.Errorf("location = %q, want settings.bogus", got)
			}
			if errs[0].Input != test.want {
				t.Errorf("input = %q, want %q", errs[0].Input, test.want)
			}
		})
	}
}

// The nested `render_command` mapping reaches the same record through a second
// call site, so the value echo is asserted there too.
func TestUnknownRenderCommandKeyEchoesItsValue(t *testing.T) {
	document := "render_command:\n  output_folder_name: out\n"
	errs := settings.ValidateUnknownKeys(
		settingsNode(t, document), []string{"settings"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}

	const wantLocation = "settings.render_command.output_folder_name"
	if got := strings.Join(errs[0].SchemaLocation, "."); got != wantLocation {
		t.Errorf("location = %q, want %s", got, wantLocation)
	}
	if errs[0].Input != "out" {
		t.Errorf("input = %q, want the value as written", errs[0].Input)
	}
}
