package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.9c behavior 33h's code table, re-measured against the vendored
// Python.
//
// Codes are the error pipeline's dispatch key — §3.7 unpacks on
// `rendercv_entry_validation_error` and §3.10 truncates the coordinate path on
// `missing` — so a wrong one is a silent routing bug, not a cosmetic slip.
// Iteration 3 shipped three of them wrong behind a green suite, which is why
// behavior 33h says every code assertion is suspect until measured and why this
// table exists as a single place to look.
//
// Every `wantCode` below is upstream's, taken from `e.errors()[i]['type']` on
// the vendored Python and written as a literal. Asserting a Go constant here
// would be mutation-proof: change the constant and the test still passes.
func TestErrorCodes(t *testing.T) {
	// The four `section.py` sites that raise `CustomPydanticErrorTypes.other`
	// (`:158`, `:169`, `:214`, `:240`) and the one that raises
	// `entry_validation` (`:230`). The split is behavior 33i's point: only the
	// wrapper's code triggers §3.7's unpacking, and it is the single site raised
	// with a different type than its four neighbours.
	sections := []struct {
		name     string
		src      string
		wantCode schemaerr.Code
	}{
		{
			name:     "the section is not a list (section.py:240)",
			src:      "  a: 1\n",
			wantCode: "rendercv_other_error",
		},
		{
			name:     "no entry resolves to a type (section.py:214)",
			src:      "  - zzz: 1\n",
			wantCode: "rendercv_other_error",
		},
		{
			name:     "an entry is null (section.py:169)",
			src:      "  - ~\n",
			wantCode: "rendercv_other_error",
		},
		{
			name:     "the entries have problems (section.py:230) — the wrapper",
			src:      "  - company: c\n",
			wantCode: "rendercv_entry_validation_error",
		},
	}

	for _, test := range sections {
		t.Run(test.name, func(t *testing.T) {
			_, errs := cv.ValidateSection(
				section(t, test.src), fixtureRegistry(),
				[]string{"cv", "sections", "x"}, schemaerr.SourceMain, sectionReference,
			)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != test.wantCode {
				t.Errorf("code = %q, want %q", errs[0].Code, test.wantCode)
			}
		})
	}

	// The two date codes and `missing`, asserted on the wrapper's children so
	// the whole path from entry to record is exercised. The date split is
	// upstream's mechanism showing through: `validate_exact_date`
	// (`entry_with_complex_fields.py:31-36`) raises `PydanticCustomError`, while
	// `validate_arbitrary_date` (`entry_with_date.py:26-29`) lets a bare Python
	// `ValueError` escape and pydantic labels that `value_error`.
	children := []struct {
		name     string
		src      string
		wantCode schemaerr.Code
	}{
		{
			name:     "a required key is absent",
			src:      "  - institution: MIT\n",
			wantCode: "missing",
		},
		{
			name:     "an exact date does not parse (PydanticCustomError)",
			src:      "  - institution: MIT\n    area: CS\n    start_date: aaa\n",
			wantCode: "rendercv_other_error",
		},
		{
			name:     "the arbitrary date is out of range (a bare ValueError)",
			src:      "  - institution: MIT\n    area: CS\n    date: 2020-20-20\n",
			wantCode: "value_error",
		},
		{
			name:     "start is after end (PydanticCustomError)",
			src:      "  - institution: MIT\n    area: CS\n    start_date: 2021\n    end_date: 2020\n",
			wantCode: "rendercv_other_error",
		},
	}

	for _, test := range children {
		t.Run(test.name, func(t *testing.T) {
			_, errs := cv.ValidateSection(
				section(t, test.src), fixtureRegistry(),
				[]string{"cv", "sections", "x"}, schemaerr.SourceMain, sectionReference,
			)
			if len(errs) != 1 || len(errs[0].Children) != 1 {
				t.Fatalf("errs = %+v, want one wrapper with one child", errs)
			}
			if errs[0].Children[0].Code != test.wantCode {
				t.Errorf("code = %q, want %q", errs[0].Children[0].Code, test.wantCode)
			}
		})
	}
}
