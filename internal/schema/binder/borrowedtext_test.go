package binder_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/errorpipeline"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// The four messages the binder borrows from pydantic, raw and final.
//
// Iteration 2 marked all four as unverified. They are measured now, and the
// pair of assertions is the point: what the binder emits is a **dictionary
// key** in three of the four cases, and what a user sees is the replacement.
// A validator that emitted the replacement would look right in isolation and
// take the message out of the dictionary's reach.
//
// `Input should be a valid string` is the one with no row, so its final text is
// itself plus a period — which is what `expected_errors.yaml:112` contains.
func TestBorrowedBinderText(t *testing.T) {
	spec := binder.Spec{
		Fields: []binder.Field{
			{Name: "name", Required: true, Value: binder.ValueString},
			{Name: "tags", Value: binder.ValueStringList},
		},
		Policy: binder.ForbidExtra,
		Model:  "Cv",
	}

	tests := []struct {
		name      string
		src       string
		wantRaw   string
		wantFinal string
	}{
		{
			name:      "a required key is absent",
			src:       "tags:\n  - a\n",
			wantRaw:   "Field required",
			wantFinal: "This field is required.",
		},
		{
			name:      "an unknown key",
			src:       "name: x\nnope: 1\n",
			wantRaw:   "Extra inputs are not permitted",
			wantFinal: "This field is unknown for this object. Please remove it.",
		},
		{
			name:    "a value that should be text",
			src:     "name:\n  a: 1\n",
			wantRaw: "Input should be a valid string",
			// No dictionary row matches, so only the period is added.
			wantFinal: "Input should be a valid string.",
		},
		{
			name:      "a value that should be a list",
			src:       "name: x\ntags: not a list\n",
			wantRaw:   "Input should be a valid list",
			wantFinal: "This field should contain a list of items but it doesn't.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := bindAll(parse(t, test.src), spec, []string{"cv"}, schemaerr.SourceMain)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.wantRaw {
				t.Errorf("raw message = %q, want %q", errs[0].Message, test.wantRaw)
			}

			final, err := errorpipeline.Parse(errs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if final[0].Message != test.wantFinal {
				t.Errorf("final message = %q, want %q", final[0].Message, test.wantFinal)
			}
		})
	}
}
