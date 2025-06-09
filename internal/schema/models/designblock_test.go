package models_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The `design` block end to end, every row measured against the vendored
// Python.
//
// The block reaches the option tree for the first time here: before this, only
// `theme` was checked, so a bad dimension five levels down went unreported —
// the same gap iteration 7's verifier found in `locale`.
func TestDesignBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		code     string
		message  string
		location string
	}{
		{
			name:  "a built-in theme with good options",
			input: "design:\n  theme: sb2nov\n  page:\n    top_margin: 1cm\n",
		},
		{
			// Spec 006 §3 behavior 6: the option, not "unknown theme".
			name:     "a bad dimension reports the option",
			input:    "design:\n  theme: classic\n  page:\n    top_margin: x\n",
			code:     "rendercv_other_error",
			message:  "The value must be a number followed by a unit (cm, in, pt, mm, em). For example, 0.1cm.",
			location: "design.page.top_margin",
		},
		{
			// And in a variant, where the option tree is the same tree.
			name:     "a bad dimension in a variant theme",
			input:    "design:\n  theme: sb2nov\n  page:\n    top_margin: x\n",
			code:     "rendercv_other_error",
			location: "design.page.top_margin",
		},
		{
			name:     "a bad colour",
			input:    "design:\n  theme: classic\n  colors:\n    body: notacolor\n",
			code:     "color_error",
			message:  "value is not a valid color: string not recognised as a valid color",
			location: "design.colors.body",
		},
		{
			name:     "a bad page size",
			input:    "design:\n  theme: classic\n  page:\n    size: a3\n",
			code:     "literal_error",
			message:  "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
			location: "design.page.size",
		},
		{
			// Non-ASCII members reach the message literally.
			name:     "a bad bullet, three levels down",
			input:    "design:\n  theme: classic\n  entries:\n    highlights:\n      bullet: x\n",
			code:     "literal_error",
			message:  "Input should be '●', '•', '◦', '-', '◆', '★', '■', '—' or '○'",
			location: "design.entries.highlights.bullet",
		},
		{
			name:     "an unknown key four levels down",
			input:    "design:\n  theme: classic\n  typography:\n    font_size:\n      nope: 1\n",
			code:     "extra_forbidden",
			message:  "Extra inputs are not permitted",
			location: "design.typography.font_size.nope",
		},
		{
			name:     "an unknown key inside a font-family mapping",
			input:    "design:\n  theme: classic\n  typography:\n    font_family:\n      nope: x\n",
			code:     "extra_forbidden",
			location: "design.typography.font_family.nope",
		},
		{
			// Any font name validates (spec 006 §3.1 behavior 12).
			name:  "an unlisted font name",
			input: "design:\n  theme: classic\n  typography:\n    font_family: AnyFont\n",
		},
		{
			// The coercion of §3.2 behavior 15 accepts anything; nothing rejects.
			name:  "a spaced section title",
			input: "design:\n  theme: classic\n  sections:\n    show_time_spans_in:\n      - Work Experience\n",
		},
		{
			name:     "a non-mapping design",
			input:    "design: null\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "design",
		},
		{
			// The custom-theme name check iteration 4 ported, still first.
			name:     "a badly named custom theme",
			input:    "design:\n  theme: No Such\n",
			code:     "rendercv_other_error",
			message:  "The custom theme name should only contain lowercase letters and digits.",
			location: "design.theme",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := models.Validate(parse(t, tc.input), nil, schemaerr.SourceMain)

			if tc.code == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if string(errs[0].Code) != tc.code {
				t.Errorf("code = %q, want %q", errs[0].Code, tc.code)
			}
			if tc.message != "" && !strings.Contains(errs[0].Message, tc.message) {
				t.Errorf("message = %q, want it to contain %q", errs[0].Message, tc.message)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != tc.location {
				t.Errorf("location = %q, want %q", got, tc.location)
			}
		})
	}
}

// A `design` block with no `theme` key **crashes upstream**: validate_design
// does `str(design["theme"])` unguarded before the union resolves it
// (design.py:57), so the shape that gives `locale` a union_tag_not_found gives
// `design` a KeyError.
//
// The port stays silent rather than inventing a message, which would report
// where upstream crashes. Iteration 12's unhandled-failure handling owns it, and
// this test says so rather than leaving the silence unexplained.
func TestDesignWithoutAThemeIsSilent(t *testing.T) {
	_, errs := models.Validate(
		parse(t, "design:\n  page:\n    top_margin: x\n"), nil, schemaerr.SourceMain)
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none until iteration 12 decides what a crash prints", errs)
	}
}
