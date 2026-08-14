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
			// bool_parsing, because a string was tried and could not be read.
			name:     "an uninterpretable boolean",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: yes please\n",
			code:     "bool_parsing",
			message:  "Input should be a valid boolean, unable to interpret input",
			location: "design.page.show_footer",
		},
		{
			// bool_type, because a fractional float is a shape pydantic will
			// not narrow — the code differs from the row above and the message
			// does too.
			name:     "a float where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: 1.5\n",
			code:     "bool_type",
			message:  "Input should be a valid boolean",
			location: "design.page.show_footer",
		},
		{
			// A tiny exponent is fractional too, so it takes the same code as
			// `1.5` rather than the whole-number one below.
			name:     "a fractional exponent where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: 1e-7\n",
			code:     "bool_type",
			message:  "Input should be a valid boolean",
			location: "design.page.show_footer",
		},
		{
			// **A whole number that is not 0 or 1 is bool_parsing**: pydantic
			// narrows it to an int and then fails *reading* it as a bool. The
			// port used to report bool_type here, printing a message two lines
			// shorter than upstream's in the validation panel. Measured on `2`,
			// `-1`, `2.0` and `1_0`; found by a fresh-context verifier
			// (iteration 14's twenty-second re-verification).
			name:     "an integer where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: 2\n",
			code:     "bool_parsing",
			message:  "Input should be a valid boolean, unable to interpret input",
			location: "design.page.show_footer",
		},
		{
			name:     "a negative integer where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: -1\n",
			code:     "bool_parsing",
			message:  "Input should be a valid boolean, unable to interpret input",
			location: "design.page.show_footer",
		},
		{
			name:     "a whole float where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: 2.0\n",
			code:     "bool_parsing",
			message:  "Input should be a valid boolean, unable to interpret input",
			location: "design.page.show_footer",
		},
		{
			name:     "a digit-separated integer where a boolean belongs",
			input:    "design:\n  theme: classic\n  page:\n    show_footer: 1_0\n",
			code:     "bool_parsing",
			message:  "Input should be a valid boolean, unable to interpret input",
			location: "design.page.show_footer",
		},
		{
			// `yes` and `1` are both accepted; the lax direction matters as much
			// as the strict one.
			name:  "a truthy word and a one",
			input: "design:\n  theme: classic\n  page:\n    show_footer: yes\n    show_top_note: 1\n",
		},
		{
			// A non-member of any shape is literal_error, not string_type.
			name:     "an integer where a literal belongs",
			input:    "design:\n  theme: classic\n  page:\n    size: 5\n",
			code:     "literal_error",
			message:  "Input should be 'a4', 'a5', 'us-letter' or 'us-executive'",
			location: "design.page.size",
		},
		{
			// The colour type owns the message even for a non-string, so this is
			// color_error and not string_type — and dictionary row 13 rewrites
			// it like any other colour failure.
			name:     "an integer where a colour belongs",
			input:    "design:\n  theme: classic\n  colors:\n    body: 5\n",
			code:     "color_error",
			message:  "value is not a valid color: value must be a tuple, list or string",
			location: "design.colors.body",
		},
		{
			// A three- or four-element sequence is a colour tuple.
			name:  "a colour tuple",
			input: "design:\n  theme: classic\n  colors:\n    body: [1, 2, 3]\n",
		},
		{
			name:     "a two-element colour tuple",
			input:    "design:\n  theme: classic\n  colors:\n    body: [1, 2]\n",
			code:     "color_error",
			message:  "value is not a valid color: tuples must have length 3 or 4",
			location: "design.colors.body",
		},
		{
			// The model is **named** in the message.
			name:     "a sequence where a font family belongs",
			input:    "design:\n  theme: classic\n  typography:\n    font_family:\n      - a\n      - b\n",
			code:     "model_type",
			message:  "Input should be a valid dictionary or instance of FontFamily",
			location: "design.typography.font_family",
		},
		{
			name:     "a string where a nested model belongs",
			input:    "design:\n  theme: classic\n  page: x\n",
			code:     "model_type",
			message:  "Input should be a valid dictionary or instance of Page",
			location: "design.page",
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

// A `design` **mapping** with no `theme` key crashes upstream: `validate_design`
// catches the discriminated union's own failure, reads the discriminator out of
// it (`design.py:38-47`), concludes the block names a custom theme, and then does
// `str(design["theme"])` unguarded (`design.py:57`) — `KeyError: 'theme'`, a Rich
// traceback on stderr at **exit 1**.
//
// Every row below is measured against the vendored Python with `NO_COLOR=1
// TERM=dumb COLUMNS=80`, non-tty:
//
//	document                        upstream                       code
//	------------------------------  -----------------------------  ----
//	(no design key)                 renders                        exit 0
//	design: {theme: classic}        renders                        exit 0
//	design: {page: {size: …}}       KeyError, 522 B out / 9611 err  exit 1
//	design: {}                      KeyError, 522 B out / 9611 err  exit 1
//	design: {bogus: 1}              KeyError, 522 B out / 9611 err  exit 1
//	design: {page: …, zzz: 1}       KeyError, 522 B out / 9611 err  exit 1
//	design: null                    panel, model_attributes_type   exit 1
//	design: (no value)              panel, model_attributes_type   exit 1
//	design: hello                   panel, model_attributes_type   exit 1
//	design: [1]                     TypeError traceback, 10600 B   exit 1
//
// The port reports `union_tag_not_found` for the four crashing mapping shapes and
// exits 1 — D-011/D-014's class, following D-012's stated precedent (a forced
// kind plus an ordinary record at exit 1). The message is not invented: it is
// pydantic's own for the union `design.py:36` tries and then discards, and is
// what `locale` — the same union without a wrap validator in front of it — shows
// a user for exactly this shape.
func TestDesignWithoutAThemeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		code     string
		message  string
		location string
	}{
		{
			name:  "no design block at all is untouched",
			input: "cv:\n  name: A\n",
		},
		{
			name:  "a theme key renders",
			input: "design:\n  theme: classic\n",
		},
		{
			name:     "a page block and no theme",
			input:    "design:\n  page:\n    size: us-letter\n",
			code:     "union_tag_not_found",
			message:  "Unable to extract tag using discriminator 'theme'",
			location: "design",
		},
		{
			name:     "an empty mapping",
			input:    "design: {}\n",
			code:     "union_tag_not_found",
			message:  "Unable to extract tag using discriminator 'theme'",
			location: "design",
		},
		{
			name:     "an unknown key alone",
			input:    "design:\n  bogus: 1\n",
			code:     "union_tag_not_found",
			message:  "Unable to extract tag using discriminator 'theme'",
			location: "design",
		},
		{
			// The union fails before any option is judged, so the bad `size`
			// and the unknown key are never reported: one record, not three.
			name:     "a bad option and an unknown key, still one record",
			input:    "design:\n  page:\n    size: bogus\n  zzz: 1\n",
			code:     "union_tag_not_found",
			message:  "Unable to extract tag using discriminator 'theme'",
			location: "design",
		},
		{
			// Not a mapping, so it never reaches the missing-key branch —
			// upstream prints this exact panel rather than crashing.
			name:     "an explicit null is the mapping shape failure",
			input:    "design: null\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "design",
		},
		{
			name:     "a key with no value is the same",
			input:    "design:\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "design",
		},
		{
			name:     "a scalar is the same",
			input:    "design: hello\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "design",
		},
		{
			// Upstream's `design["theme"]` raises `TypeError` here rather than
			// `KeyError`; the port keeps the shape record it already had, and the
			// exit code is 1 on both sides either way.
			name:     "a sequence is the same",
			input:    "design: [1]\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "design",
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
			if !strings.Contains(errs[0].Message, tc.message) {
				t.Errorf("message = %q, want it to contain %q", errs[0].Message, tc.message)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != tc.location {
				t.Errorf("location = %q, want %q", got, tc.location)
			}
		})
	}
}
