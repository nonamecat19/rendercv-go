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

// **The candidate name is Python's `str()` of the value, not the raw YAML
// token.** A null used to skip the whole check by returning early; `true`
// stringified as its own raw text (`"true"`, lowercase, which happens to
// match the pattern) rather than Python's `"True"`; and a sequence or
// mapping had no text at all, so the message's backtick-quoted value was
// empty. All four fail the pattern and are rejected the way upstream
// rejects `str(None)`, `str(True)`, `str(['a', 'b'])` and `str({'a': 1})`.
// Found by a fresh-context verifier (iteration 14's twelfth
// re-verification).
func TestValidateThemeNonStringValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "null", value: "null", want: "None"},
		{name: "true", value: "true", want: "True"},
		{name: "false", value: "false", want: "False"},
		{name: "a sequence", value: "[a, b]", want: "['a', 'b']"},
		{name: "a mapping", value: "{a: 1}", want: "{'a': 1}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := design.ValidateTheme(
				themeNode(t, test.value), []string{"design"}, schemaerr.SourceMain,
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			want := "The custom theme name should only contain lowercase letters and" +
				" digits. The provided value is `" + test.want + "`."
			if errs[0].Message != want {
				t.Errorf("message =\n  %q\nwant\n  %q", errs[0].Message, want)
			}
			if errs[0].Input != test.want {
				t.Errorf("input = %q, want %q", errs[0].Input, test.want)
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

// **A YAML word-form boolean must reach the effective tree as a `bool`, not
// as the raw text a document's `no`/`yes` resolves to** — the same defect
// reaches a *built-in* theme, with no script involved at all: `mappingOf`
// (`internal/renderer/bridge/model.go`) keeps a design scalar's source text,
// so `show_footer: no` becomes the string `"no"` in the document map handed
// to `design.Effective`, and left uncoerced the Typst emitter would
// interpolate that literal text where a `false` token belongs — output that
// does not compile. Found by a fresh-context verifier (iteration 14's fifth
// re-verification).
func TestWordFormBooleanOnABuiltinTheme(t *testing.T) {
	// `classic`'s own default is `false` (`overrides_generated.go`), so a
	// coerced-to-zero-value bug would still read `false` here by accident;
	// the override has to flip it to `true` to be discriminating.
	values := design.Effective("classic", map[string]any{
		"page": map[string]any{"show_footer": "yes"},
	})
	if got := design.EffectiveBool(values, "page", "show_footer"); !got {
		t.Errorf("page.show_footer = %v, want true — the word-form override was not coerced to a bool", got)
	}
}

// **The font_family partial-mapping fix must hold with no script at all.**
// `opal`'s own font is Lato; a document overriding only `body` must still
// fall back to `FontFamily`'s own base default ("Source Sans 3") for the
// other four elements, not to `opal`'s Lato and not to the empty string a
// naive merge onto the theme's (by-then-scalar) value produces. Iteration
// 14's fix landed only for the scripted-custom-theme path
// (internal/renderer/bridge/luatheme_test.go); nothing pinned the identical
// defect on a built-in theme, which a fresh-context verifier flagged as an
// untested reach (iteration 14's sixth re-verification).
func TestFontFamilyPartialOverrideOnABuiltinTheme(t *testing.T) {
	values := design.Effective("opal", map[string]any{
		"typography": map[string]any{
			"font_family": map[string]any{"body": "Charter"},
		},
	})
	if got := design.EffectiveString(values, "typography", "font_family", "body"); got != "Charter" {
		t.Errorf("font_family.body = %q, want the document's override", got)
	}
	for _, sibling := range []string{"name", "headline", "connections", "section_titles"} {
		if got := design.EffectiveString(values, "typography", "font_family", sibling); got != "Source Sans 3" {
			t.Errorf("font_family.%s = %q, want the base FontFamily default, not opal's Lato or empty", sibling, got)
		}
	}
}

// **An explicit null on a design field is a type failure, not an absence.**
// Every design field has a default, so none is `Required` in the binder's
// sense — but a default is not the same as upstream's `X | None`: only
// `templates.education_entry.degree_column` (`KindOptionalString`) is
// actually typed nullable. `!Required` alone made a null anywhere else pass
// silently and fall back to the *base* tree's default rather than the
// theme's own, or rather than being rejected outright — measured on five
// field kinds across a **built-in** theme, no script involved. Found by a
// fresh-context verifier (iteration 14's eleventh re-verification).
func TestNullDesignFieldsAreRejectedExceptDegreeColumn(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "a color",
			yaml: "theme: sb2nov\ncolors:\n  name: null\n",
			want: "design.colors.name",
		},
		{
			name: "a typst dimension",
			yaml: "theme: sb2nov\npage:\n  top_margin: null\n",
			want: "design.page.top_margin",
		},
		{
			name: "a literal",
			yaml: "theme: sb2nov\npage:\n  size: null\n",
			want: "design.page.size",
		},
		{
			name: "a bool",
			yaml: "theme: sb2nov\nentries:\n  short_second_row: null\n",
			want: "design.entries.short_second_row",
		},
		{
			name: "a string list",
			yaml: "theme: sb2nov\nsections:\n  show_time_spans_in: null\n",
			want: "design.sections.show_time_spans_in",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString(test.yaml)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
			if len(errs) == 0 {
				t.Fatalf("errs = none, want a failure at %s", test.want)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != test.want {
				t.Errorf("location = %q, want %q", got, test.want)
			}
		})
	}
}

// The one field that genuinely admits null must keep passing.
func TestNullDegreeColumnIsAccepted(t *testing.T) {
	doc, err := yamlreader.ReadString(
		"theme: sb2nov\ntemplates:\n  education_entry:\n    degree_column: null\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — degree_column is str | None", errs)
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
