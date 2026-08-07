package models_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The `locale` block end to end, from a whole document.
//
// Every row is measured against the vendored Python, and the table exists
// because the catalog model iteration 7 landed had no edge reaching it — the
// rules were written, tested in isolation, and unreachable from a real file.
// These are the documents that reach them.
//
// Two rows would pass under the reading that looks right and is wrong:
// `language: null` is a *failure* rather than an absence, and a variant's
// eleven-element month list is *accepted* because the twelve-element bound is
// EnglishLocale's alone.
func TestLocaleBlock(t *testing.T) {
	elevenMonths := "  month_names:\n" + strings.Repeat("    - x\n", 11)

	tests := []struct {
		name     string
		input    string
		code     string
		message  string
		location string
	}{
		{
			name:  "a known language validates",
			input: "locale:\n  language: danish\n",
		},
		{
			name:     "an unknown language is a tag failure at the block",
			input:    "locale:\n  language: klingon\n",
			code:     "union_tag_invalid",
			location: "locale",
		},
		{
			name:     "a null language is a tag failure reading 'None'",
			input:    "locale:\n  language: null\n",
			code:     "union_tag_invalid",
			message:  "Input tag 'None' found using 'language' does not match",
			location: "locale",
		},
		{
			name:     "no language at all is a different code",
			input:    "locale:\n  month: x\n",
			code:     "union_tag_not_found",
			message:  "Unable to extract tag using discriminator 'language'",
			location: "locale",
		},
		{
			name:     "a non-mapping locale",
			input:    "locale: null\n",
			code:     "model_attributes_type",
			message:  "Input should be a valid dictionary or object to extract fields from",
			location: "locale",
		},
		{
			name:     "an unknown key reaches the member",
			input:    "locale:\n  language: danish\n  bogus: 1\n",
			code:     "extra_forbidden",
			message:  "Extra inputs are not permitted",
			location: "locale.bogus",
		},
		{
			name:     "eleven months is too short for English",
			input:    "locale:\n  language: english\n" + elevenMonths,
			code:     "too_short",
			message:  "List should have at least 12 items after validation, not 11",
			location: "locale.month_names",
		},
		{
			name:  "eleven months is fine for a variant",
			input: "locale:\n  language: danish\n" + elevenMonths,
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
