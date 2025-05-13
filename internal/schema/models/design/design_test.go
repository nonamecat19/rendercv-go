package design_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func themeNode(t *testing.T, value string) *yamldoc.Node {
	t.Helper()
	doc, err := yamlreader.ReadString("theme: " + value + "\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return doc.Items[0].Value
}

// Spec 004 §3.17 behavior 64 and §4.27.
func TestValidateTheme(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "a built-in theme", value: "classic", want: ""},
		{name: "another built-in theme", value: "engineeringresumes", want: ""},
		// A custom theme is fine as long as the name is lowercase alphanumeric;
		// whether the folder exists is iteration 6's question.
		{name: "a well-formed custom name", value: "mytheme2", want: ""},
		{
			name:  "an underscore is not allowed",
			value: "not_a_valid_theme",
			want: "The custom theme name should only contain lowercase letters and" +
				" digits. The provided value is `not_a_valid_theme`.",
		},
		{
			name:  "nor is an uppercase letter",
			value: "MyTheme",
			want: "The custom theme name should only contain lowercase letters and" +
				" digits. The provided value is `MyTheme`.",
		},
		{
			name:  "nor a hyphen",
			value: "my-theme",
			want: "The custom theme name should only contain lowercase letters and" +
				" digits. The provided value is `my-theme`.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := design.ValidateTheme(
				themeNode(t, test.value), []string{"design"}, schemaerr.SourceMain,
			)
			if test.want == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.want {
				t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, test.want)
			}
			// The input is the theme name, not the whole design mapping.
			if errs[0].Input != test.value {
				t.Errorf("input = %q, want the theme name", errs[0].Input)
			}
		})
	}
}

// Spec 004 §3.17 behavior 65: the location is pinned, and the pin is what keeps
// it.
//
// `design` is a discriminated root, so without LocationIsFinal the pipeline's
// step 2 would drop `theme` as a branch value and the record would land at
// `("design",)`. The second half of this test is the one that fails if the flag
// is removed.
func TestThemeLocationIsPinnedThroughThePipeline(t *testing.T) {
	errs := design.ValidateTheme(
		themeNode(t, "not_a_valid_theme"), []string{"design"}, schemaerr.SourceMain,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want exactly one", errs)
	}
	if !errs[0].LocationIsFinal {
		t.Error("LocationIsFinal is false; step 2 will drop `theme`")
	}

	final, err := errorpipeline.Parse(errs, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := strings.Join(final[0].SchemaLocation, "."); got != "design.theme" {
		t.Errorf("final location = %q, want design.theme", got)
	}

	// Upstream's fixture text, period included — no dictionary row matches, so
	// the pipeline only appends one, and the message already ends in a period.
	const want = "The custom theme name should only contain lowercase letters and" +
		" digits. The provided value is `not_a_valid_theme`."
	if final[0].Message != want {
		t.Errorf("final message =\n  %q\nwant\n  %q", final[0].Message, want)
	}
}

// The nine built-in names, in the order the discriminated union discovers them.
func TestBuiltInThemes(t *testing.T) {
	want := []string{
		"classic", "ember", "engineeringclassic", "engineeringresumes",
		"harvard", "ink", "moderncv", "opal", "sb2nov",
	}
	if len(design.BuiltInThemes) != len(want) {
		t.Fatalf("themes = %v, want %v", design.BuiltInThemes, want)
	}
	for i := range want {
		if design.BuiltInThemes[i] != want[i] {
			t.Fatalf("themes = %v, want %v", design.BuiltInThemes, want)
		}
	}
}
