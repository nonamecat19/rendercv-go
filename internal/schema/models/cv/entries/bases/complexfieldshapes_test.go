package bases_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 003 §3.13 behaviors 24-26. Iteration 2 bound `location`, `summary` and
// `highlights` as raw nodes, so a mapping in `summary` or a scalar in
// `highlights` passed silently. Upstream declares them `str | None`,
// `str | None` and `list[str] | None`
// (entry_with_complex_fields.py:106-132).
//
// Every row was measured against the vendored Python with EducationEntry, whose
// inherited fields these are.
func TestComplexFieldShapes(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantCode schemaerr.Code
		wantPath string
	}{
		{
			name:     "a mapping in summary is a string error",
			src:      "summary:\n  a: 1\n",
			wantCode: binder.CodeStringType,
			wantPath: "cv.sections.x.0.summary",
		},
		{
			name:     "a scalar in highlights is a list error",
			src:      "highlights: nope\n",
			wantCode: binder.CodeListType,
			wantPath: "cv.sections.x.0.highlights",
		},
		{
			name:     "a non-text highlight is a string error at its index",
			src:      "highlights:\n  - ok\n  - 5\n",
			wantCode: binder.CodeStringType,
			wantPath: "cv.sections.x.0.highlights.1",
		},
		{
			// The YAML reader resolves `2020` to KindInt, so a year written into
			// a text field must fail rather than coerce.
			name:     "an int in location is a string error",
			src:      "location: 2020\n",
			wantCode: binder.CodeStringType,
			wantPath: "cv.sections.x.0.location",
		},
		{
			name:     "a sequence in location is a string error",
			src:      "location:\n  - Istanbul\n",
			wantCode: binder.CodeStringType,
			wantPath: "cv.sections.x.0.location",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := bind(t, test.src)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Code != test.wantCode {
				t.Errorf("code = %q, want %q", errs[0].Code, test.wantCode)
			}
			if got := strings.Join(errs[0].SchemaLocation, "."); got != test.wantPath {
				t.Errorf("location = %q, want %q", got, test.wantPath)
			}
		})
	}
}

// All three are optional, so null is the declared default and passes — verified
// upstream for each of `location: None`, `summary: None` and `highlights: None`.
func TestComplexFieldsAcceptNull(t *testing.T) {
	for _, src := range []string{"location: null\n", "summary: null\n", "highlights: null\n"} {
		t.Run(strings.SplitN(src, ":", 2)[0], func(t *testing.T) {
			if _, errs := bind(t, src); len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}

// `start_date` and `end_date` are `str | int` upstream, so an integer year is
// legal and must not be caught by the string check — verified upstream with
// `{start_date: 2020, end_date: 2021}`, which validates.
func TestExactDateFieldsAcceptIntegers(t *testing.T) {
	entry, errs := bind(t, "start_date: 2020\nend_date: 2021\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.StartDate != "2020" || entry.EndDate != "2021" {
		t.Errorf("dates = %q/%q, want 2020/2021", entry.StartDate, entry.EndDate)
	}
}
