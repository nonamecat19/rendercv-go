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

// **A trailing `\n` is not a shape failure.** Python's `$` matches at the
// end of the string or just before one trailing newline, so
// `re.match(r"^[a-z0-9]+$", "mytheme\n")` is true — a name check reached
// only through a block scalar (`theme: |\n  mytheme\n`), but real. Found by
// a fresh-context verifier (iteration 14's non-colour-design-slice sweep).
func TestValidateThemeAllowsATrailingNewline(t *testing.T) {
	errs := design.ValidateTheme(
		themeNode(t, `"mytheme\n"`), []string{"design"}, schemaerr.SourceMain,
	)
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a trailing newline passes upstream's $", errs)
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
// Shape errors (binder.Bind) and value/enum errors (the per-field loop) must
// interleave by the model's field-declaration order, matching upstream's
// pydantic — not shape errors first. `Page`'s fields are declared `size` then
// `top_margin` (tree_generated.go), so an enum failure on `size` together with
// a type failure on `top_margin` must report in that order even though `size`
// is a value error and `top_margin` is a shape error.
func TestErrorsInterleaveByFieldDeclarationOrder(t *testing.T) {
	doc, err := yamlreader.ReadString(
		"theme: sb2nov\npage:\n  size: not-a-size\n  top_margin: {}\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)

	var locations []string
	for _, e := range errs {
		locations = append(locations, strings.Join(e.SchemaLocation, "."))
	}
	want := []string{"design.page.size", "design.page.top_margin"}
	if len(locations) < 2 || locations[0] != want[0] || locations[1] != want[1] {
		t.Fatalf("locations = %v, want %v first two in order", locations, want)
	}
}

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

// A colour tuple with an out-of-range element must fail validation, not just
// a wrong-length one — `validColorNode` used to check only length.
func TestOutOfRangeColorTupleIsRejected(t *testing.T) {
	doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: [300, 0, 0]\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
	if len(errs) == 0 {
		t.Fatal("errs = none, want a failure at design.colors.name")
	}
	if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.colors.name" {
		t.Errorf("location = %q, want design.colors.name", got)
	}
}

// **A colour-tuple element written in non-ASCII decimal digits is valid
// upstream.** Python's `float(str)` transliterates every Unicode `Nd`
// character to the ASCII digit of the same value before parsing, so
// `colors.name: [١٢٣, 2, 3]` is `rgb(123, 2, 3)` — measured against the
// vendored Python, which renders it at exit 0 with the same `.typ` bytes as
// `[123, 2, 3]`. `strconv.ParseFloat` is ASCII-only, so this exited 1 here:
// the port rejecting what upstream accepts. Note the element is a
// `KindString` to ruamel and to this port alike (both int resolvers are ASCII
// regexes), so this is the *non*-coercing path, the one
// `TestParseColorTupleAcceptsNonASCIIDigits` cannot reach. Found by a
// fresh-context verifier (iteration 14's colour-slice sweep, deferred there).
func TestNonASCIIDigitColorTupleIsAccepted(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool // true = must fail
	}{
		{name: "an Arabic-Indic channel", yaml: `[١٢٣, 2, 3]`, want: false},
		{name: "a Devanagari channel", yaml: `[१२३, 2, 3]`, want: false},
		{name: "a fullwidth channel", yaml: `[１２３, 2, 3]`, want: false},
		{name: "an Arabic-Indic alpha", yaml: `[1, 2, 3, ٠.٥]`, want: false},
		// Transliterated, then range-checked like any other number.
		{name: "an out-of-range Arabic-Indic channel", yaml: `[٣٠٠, 2, 3]`, want: true},
		// Not `Nd`, so `float()` raises on it too.
		{name: "a Han-numeral channel", yaml: `[一二三, 2, 3]`, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: " + test.yaml + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
			if test.want && len(errs) == 0 {
				t.Error("errs = none, want a failure")
			}
			if !test.want && len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}

// The same digits in the *string* colour forms, which reach the parser
// through a different door: upstream's `r_rgb`/`r_hsl` are `str` patterns
// compiled without `re.ASCII`, so their `\d` matches every Unicode decimal
// digit and `colors.name: "rgb(١٢٣, 2, 3)"` is a colour the vendored Python
// renders at exit 0 as `rgb(123, 2, 3)`. Go's `\d` is ASCII-only, so the
// whole document exited 1 here. Measured end to end against
// `just upstream render` for each accepted row.
func TestNonASCIIDigitColorStringIsAccepted(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool // true = must fail
	}{
		{name: "an Arabic-Indic channel", yaml: `"rgb(١٢٣, 2, 3)"`, want: false},
		{name: "a Devanagari channel", yaml: `"rgb(१२३, 2, 3)"`, want: false},
		{name: "a fullwidth channel", yaml: `"rgb(１２３, 2, 3)"`, want: false},
		{name: "a spaced-form channel", yaml: `"rgb(1 2 ٣)"`, want: false},
		{name: "a hue", yaml: `"hsl(١٢٠, 50%, 50%)"`, want: false},
		{name: "a percent alpha", yaml: `"rgba(1, 2, 3, ٥٠%)"`, want: false},
		// Widened only where upstream's `\d` is: hex is a literal
		// `[0-9a-f]` class in both, and the range check still runs.
		{name: "an Arabic-Indic hex", yaml: `"#١٢٣"`, want: true},
		{name: "an out-of-range channel", yaml: `"rgb(٣٠٠, 0, 0)"`, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: " + test.yaml + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
			if test.want && len(errs) == 0 {
				t.Error("errs = none, want a failure")
			}
			if !test.want && len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}

// **A quoted colour-tuple element must not get the bool-word/hex/octal/
// binary coercion an unquoted one gets.** `colors.name: ["0x10", 0, 0]`
// hands upstream's `float()` a `str`, which raises on that spelling exactly
// as it would on any other non-numeric text — only `colors.name: [0x10, 0,
// 0]`, which YAML itself resolves to an `int`, is legal. Found by a
// fresh-context verifier (iteration 14's sixteenth re-verification).
func TestQuotedColorTupleElementsAreNotCoerced(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool // true = must fail
	}{
		{name: "a quoted hex channel", yaml: `["0x10", 0, 0]`, want: true},
		{name: "a quoted bool-word channel", yaml: `["true", 0, 0]`, want: true},
		{name: "an unquoted hex channel", yaml: `[0x10, 0, 0]`, want: false},
		{name: "an unquoted bool-word channel", yaml: `[true, 0, 0]`, want: false},
		{name: "a quoted hex percent alpha", yaml: `[0, 0, 0, "0x10%"]`, want: true},
		// **Go's hex-float syntax is not a Python `float()` literal.**
		// `strconv.ParseFloat` alone accepts it even with the bool/hex/
		// octal/binary coercion turned off, which used to let a quoted
		// element validate and then leak a raw `[]string` into the
		// artifact. Found by a fresh-context verifier (iteration 14's
		// colour-slice sweep).
		{name: "a quoted Go hex-float channel", yaml: `["0x1p-2", 0, 0]`, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: " + test.yaml + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
			if test.want && len(errs) == 0 {
				t.Error("errs = none, want a failure")
			}
			if !test.want && len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}

// A null alpha is `parse_float_alpha(None)`, which returns `None` rather
// than raising — so a four-element tuple with a null fourth element is
// exactly the equivalent three-element tuple, not a validation failure.
func TestNullAlphaInColorTupleIsAccepted(t *testing.T) {
	doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: [1, 2, 3, null]\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none — a null alpha means no alpha", errs)
	}
}

// **A colour-tuple element that is not a scalar must be rejected.**
// `yamldoc.Node.Raw` is scalar-only, so a sequence or a mapping element
// arrives carrying `Raw: ""` — and an empty alpha is `parse_float_alpha`'s
// "no alpha at all", so `[1, 2, 3, [1]]` validated with three channels and
// rendered `rgb(1, 2, 3)` at exit 0: a colour the document never asked for.
// Upstream's `parse_float_alpha` catches only `ValueError`, so the same
// document is an unhandled `TypeError` there, exit 1; the port declines to
// reproduce a traceback and reports a `color_error` record instead, which
// the dictionary rewrites to the same text every other colour failure gets
// and which exits 1 the same way.
//
// The channel positions read the same empty `Raw`, but their answer was
// already right by accident: `parse_color_value` catches `TypeError` too and
// raises `color values must be a valid number`, which is exactly what an
// empty `Raw` produced. They are made explicit here so the two positions
// stop depending on that coincidence.
func TestNonScalarColorTupleElementIsRejected(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "a sequence alpha", yaml: `[1, 2, 3, [1]]`},
		{name: "a mapping alpha", yaml: `[1, 2, 3, {a: 1}]`},
		{name: "a sequence red", yaml: `[[1], 2, 3]`},
		{name: "a mapping red", yaml: `[{a: 1}, 2, 3]`},
		{name: "a sequence green", yaml: `[1, [2], 3]`},
		{name: "a sequence blue", yaml: `[1, 2, [3]]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: " + test.yaml + "\n")
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}
			node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
			errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
			if len(errs) == 0 {
				t.Fatal("errs = none, want a failure at design.colors.name")
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != "design.colors.name" {
				t.Errorf("location = %q, want design.colors.name", got)
			}
			if !strings.HasPrefix(errs[0].Message, "value is not a valid color") {
				t.Errorf("message = %q, want a color_error message", errs[0].Message)
			}
		})
	}
}

// A non-scalar element must not steal the length check: upstream reaches
// `parse_color_value` only after `parse_tuple` has already decided the tuple
// has three or four elements, so `[[1], 2]` is a length failure.
func TestNonScalarElementDoesNotPreemptTupleLength(t *testing.T) {
	doc, err := yamlreader.ReadString("theme: sb2nov\ncolors:\n  name: [[1], 2]\n")
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	node := &yamldoc.Node{Kind: yamldoc.KindMapping, Items: doc.Items}
	errs := design.Validate(node, []string{"design"}, schemaerr.SourceMain, nil)
	if len(errs) == 0 {
		t.Fatal("errs = none, want a failure at design.colors.name")
	}
	want := "value is not a valid color: tuples must have length 3 or 4"
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
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
