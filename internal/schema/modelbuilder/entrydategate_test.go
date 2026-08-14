package modelbuilder

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// row is one line of the validation panel: the location column and the
// explanation column. The tests below assert the **whole ordered list**, because
// the defect this file pins was an extra row appended after two correct ones —
// a presence check on any single row would have passed throughout.
type row struct {
	location string
	message  string
}

// gateCurrentDate is what `present` resolves to
// (entry_with_complex_fields.py:78-83). Pinned so the year-3000 start dates
// below stay in the future for as long as the port exists.
var gateCurrentDate = time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)

func panelRows(t *testing.T, src string) []row {
	t.Helper()

	_, err := BuildModel(
		buildResult(t, src),
		&valctx.ValidationContext{CurrentDate: gateCurrentDate},
	)
	if err == nil {
		return nil
	}

	var userErr *schemaerr.UserValidationError
	if !errors.As(err, &userErr) {
		t.Fatalf("err = %v (%T), want *schemaerr.UserValidationError", err, err)
	}

	rows := make([]row, 0, len(userErr.Errors))
	for _, record := range userErr.Errors {
		rows = append(rows, row{
			location: strings.Join(record.SchemaLocation, "."),
			message:  record.Message,
		})
	}
	return rows
}

const (
	sectionRowExperience = "There are problems with the entries. RenderCV detected the entry" +
		" type of this section to be ExperienceEntry. The problems are shown below."
	sectionRowEducation = "There are problems with the entries. RenderCV detected the entry" +
		" type of this section to be EducationEntry. The problems are shown below."
	sectionRowNormal = "There are problems with the entries. RenderCV detected the entry" +
		" type of this section to be NormalEntry. The problems are shown below."
	requiredRow = "This field is required."
)

func orderingRow(start, end string) string {
	return "`start_date` cannot be after `end_date`. The `start_date` is " + start +
		" and the `end_date` is " + end + "."
}

// Spec 002 §3.16 behavior 77a — `check_and_adjust_dates` is a `mode="after"`
// model validator (entry_with_complex_fields.py:134-135), so one field-level
// failure anywhere in the entry short-circuits it and the ordering row of step 4
// never appears.
//
// Every case here was measured against the vendored Python at `2eba248` with
// `NO_COLOR=1 TERM=dumb COLUMNS=80` and
// `render CV.yaml -nopdf -nopng -nomd -nohtml`; `want` is upstream's panel, in
// upstream's order.
func TestOrderingCheckIsGatedOnFieldErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []row
	}{
		{
			name: "missing required field alone",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        start_date: '2000-01'
        end_date: '2001-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0.position", requiredRow},
			},
		},
		{
			name: "bad date order alone",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0", orderingRow("3000-01", "present")},
			},
		},
		{
			// The reported defect: the port appended a third row here.
			name: "missing required field and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0.position", requiredRow},
			},
		},
		{
			name: "malformed start_date and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        start_date: nonsense
        end_date: '2000'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{
					"cv.sections.a.0.start_date",
					"This is not a valid date! Please use either YYYY-MM-DD, YYYY-MM," +
						" or YYYY format.",
				},
			},
		},
		{
			name: "malformed end_date and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        start_date: '3000-01'
        end_date: 'nonsense'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{
					"cv.sections.a.0.end_date",
					"This is not a valid `end_date`! Please use either YYYY-MM-DD," +
						" YYYY-MM, or YYYY format or \"present\"!.",
				},
			},
		},
		{
			// The suppression is per entry, not per section: entry 1 binds cleanly
			// and still reports.
			name: "one entry with a field error, a sibling with a date problem",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        start_date: '3000-01'
      - company: Y
        position: P
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0.position", requiredRow},
				{"cv.sections.a.1", orderingRow("3000-01", "present")},
			},
		},
		{
			// An *optional* field is enough to gate it — the failure need not be on
			// a required field or on a date.
			name: "mistyped optional highlights and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        highlights: 5
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{
					"cv.sections.a.0.highlights",
					"This field should contain a list of items but it doesn't.",
				},
			},
		},
		{
			name: "mistyped highlights item and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        highlights:
          - a: b
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0.highlights.0", "Input should be a valid string."},
			},
		},
		{
			name: "mistyped optional summary and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        summary:
          a: b
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0.summary", "Input should be a valid string."},
			},
		},
		{
			// An unknown key is not a field failure — entries allow extras
			// (models/base.py:9) — so the ordering row survives it.
			name: "unknown key and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        unknown_key: Z
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0", orderingRow("3000-01", "present")},
			},
		},
		{
			// Two cross-field problems at once, neither entry gated: both report.
			name: "two entries with date problems",
			input: `cv:
  name: A
  sections:
    a:
      - company: X
        position: P
        start_date: '3000-01'
      - company: Y
        position: Q
        start_date: '3000-02'
`,
			want: []row{
				{"cv.sections.a", sectionRowExperience},
				{"cv.sections.a.0", orderingRow("3000-01", "present")},
				{"cv.sections.a.1", orderingRow("3000-02", "present")},
			},
		},
		{
			// The gate is on the base, so every entry type built on
			// BaseEntryWithComplexFields inherits it.
			name: "education entry, missing required field and bad date order",
			input: `cv:
  name: A
  sections:
    a:
      - area: X
        start_date: '3000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowEducation},
				{"cv.sections.a.0.institution", requiredRow},
			},
		},
		{
			name: "normal entry, bad date order alone",
			input: `cv:
  name: A
  sections:
    a:
      - name: X
        start_date: '3000-01'
        end_date: '2000-01'
`,
			want: []row{
				{"cv.sections.a", sectionRowNormal},
				{"cv.sections.a.0", orderingRow("3000-01", "2000-01")},
			},
		},
		{
			// Enough missing fields and no type matches at all, so the entry-level
			// rows never get built.
			name: "no entry type matches",
			input: `cv:
  name: A
  sections:
    a:
      - start_date: '3000-01'
        location: L
`,
			want: []row{{
				"cv.sections.a",
				"RenderCV couldn't match this section with any entry types. Please" +
					" check the entries and make sure they are provided correctly.",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := panelRows(t, tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("row %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
