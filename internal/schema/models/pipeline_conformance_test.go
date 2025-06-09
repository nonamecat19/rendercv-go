//go:build conformance

package models_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// `design` and `locale` errors through the **whole** pipeline, which is where
// the user's location and message come from.
//
// The block-level tests stop at `models.Validate`, and that is exactly what hid
// the bug this test exists for: the pipeline reproduced upstream's step 2 — the
// deletion of a discriminated union's branch element — on locations that never
// carried one, so `design.colors.body` became `design.body` and then failed to
// resolve against the document at all. Every row below was measured against the
// vendored Python's `parse_validation_errors`.
func TestDesignAndLocaleErrorsSurviveTheWholePipeline(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		location string
		message  string
	}{
		{
			// Dictionary row 13's first live producer (spec 006 §3.1
			// behavior 11): the raw `color_error` text is a substring of the
			// row's key, so what the user sees is the rewritten sentence — and
			// the period the pipeline appends lands after the closing quote.
			name:     "a colour reaches dictionary row 13",
			input:    "design:\n  theme: classic\n  colors:\n    body: notacolor\n",
			location: "design.colors.body",
			message: `This is not a valid color. Here are some examples of valid colors:` +
				` "red", "#ff0000", "rgb(255, 0, 0)", "hsl(0, 100%, 50%)".`,
		},
		{
			name:     "a dimension four levels deep",
			input:    "design:\n  theme: classic\n  page:\n    top_margin: x\n",
			location: "design.page.top_margin",
			message: "The value must be a number followed by a unit (cm, in, pt, mm, em)." +
				" For example, 0.1cm.",
		},
		{
			name:     "an unknown key at the top of design",
			input:    "design:\n  theme: classic\n  nope: 1\n",
			location: "design.nope",
			message:  "This field is unknown for this object. Please remove it.",
		},
		{
			name:     "an unknown key five levels deep",
			input:    "design:\n  theme: classic\n  typography:\n    font_size:\n      nope: 1\n",
			location: "design.typography.font_size.nope",
			message:  "This field is unknown for this object. Please remove it.",
		},
		{
			name:     "a locale key",
			input:    "locale:\n  language: danish\n  nope: 1\n",
			location: "locale.nope",
			message:  "This field is unknown for this object. Please remove it.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			document, err := yamlreader.ReadString(tc.input)
			if err != nil {
				t.Fatalf("ReadString: %v", err)
			}

			_, raw := models.Validate(document, nil, schemaerr.SourceMain)
			if len(raw) != 1 {
				t.Fatalf("errs = %+v, want exactly one", raw)
			}

			final, err := errorpipeline.Parse(raw, document, nil)
			if err != nil {
				t.Fatalf("Parse: %v — the location did not resolve against the document", err)
			}
			if got := strings.Join(final[0].SchemaLocation, "."); got != tc.location {
				t.Errorf("location = %q, want %q", got, tc.location)
			}
			if final[0].Message != tc.message {
				t.Errorf("message = %q, want %q", final[0].Message, tc.message)
			}
		})
	}
}
