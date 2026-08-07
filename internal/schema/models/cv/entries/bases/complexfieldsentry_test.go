package bases_test

import (
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

func bind(t *testing.T, src string) (*bases.BaseEntryWithComplexFields, []schemaerr.ValidationError) {
	t.Helper()
	return bases.BindEntryWithComplexFields(
		parse(t, src), bases.ComplexSpec(nil),
		[]string{"cv", "sections", "x", "0"}, schemaerr.SourceMain, reference,
	)
}

// Spec §3.79 — the five fields, in declaration order after the inherited `date`.
func TestComplexFieldNames(t *testing.T) {
	want := []string{"start_date", "end_date", "location", "summary", "highlights"}
	got := bases.FieldNames(bases.ComplexSpec(nil))[1:]
	if len(got) != len(want) {
		t.Fatalf("field names = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("field names = %v, want %v", got, want)
		}
	}
}

func TestComplexFieldsBind(t *testing.T) {
	entry, errs := bind(t, ""+
		"start_date: 2020-09\n"+
		"end_date: 2021-09\n"+
		"location: Istanbul\n"+
		"summary: Did things\n"+
		"highlights:\n  - One\n  - Two\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if entry.Location == nil || entry.Summary == nil || entry.Highlights == nil {
		t.Errorf("entry = %+v, want location, summary and highlights bound", entry)
	}
}

// Spec §3.77 — the four-step precedence.
func TestDatePrecedence(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDate  string
		wantStart string
		wantEnd   string
	}{
		{
			name:     "date wins and clears the range",
			input:    "date: 2020-09-24\nstart_date: 2019-01-01\nend_date: 2020-01-01\n",
			wantDate: "2020-09-24",
		},
		{
			name:     "lone end_date migrates to date",
			input:    "end_date: 2021-02-03\n",
			wantDate: "2021-02-03",
		},
		{
			name:      "lone start_date implies present",
			input:     "start_date: 2020-09\n",
			wantStart: "2020-09",
			wantEnd:   "present",
		},
		{
			name:      "both kept when both given",
			input:     "start_date: 2020-09\nend_date: 2021-09\n",
			wantStart: "2020-09",
			wantEnd:   "2021-09",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, errs := bind(t, tc.input)
			if len(errs) != 0 {
				t.Fatalf("errs = %+v, want none", errs)
			}
			if entry.DateValue != tc.wantDate {
				t.Errorf("date = %q, want %q", entry.DateValue, tc.wantDate)
			}
			if entry.StartDate != tc.wantStart {
				t.Errorf("start_date = %q, want %q", entry.StartDate, tc.wantStart)
			}
			if entry.EndDate != tc.wantEnd {
				t.Errorf("end_date = %q, want %q", entry.EndDate, tc.wantEnd)
			}
		})
	}
}

// Spec §5.22 — discarded range values produce no diagnostic.
func TestDiscardedRangeIsSilent(t *testing.T) {
	_, errs := bind(t, "date: 2020\nstart_date: 2023-01-01\nend_date: 2021-01-01\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none — the rewrite is silent", errs)
	}
}

// Spec §4.16 — the ordering violation, with the user's own spellings interpolated.
func TestOrderingViolation(t *testing.T) {
	tests := []struct {
		input string
		start string
		end   string
	}{
		{input: "start_date: 2023-01-01\nend_date: 2021-01-01\n", start: "2023-01-01", end: "2021-01-01"},
		{input: "start_date: 2022\nend_date: 2021\n", start: "2022", end: "2021"},
		{input: "start_date: 2025\nend_date: 2021\n", start: "2025", end: "2021"},
	}

	for _, tc := range tests {
		t.Run(tc.start+" to "+tc.end, func(t *testing.T) {
			_, errs := bind(t, tc.input)
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			want := "`start_date` cannot be after `end_date`. The `start_date` is " +
				tc.start + " and the `end_date` is " + tc.end + "."
			if errs[0].Message != want {
				t.Errorf("message = %q, want %q", errs[0].Message, want)
			}
		})
	}
}

// A start date equal to the end date is accepted.
func TestEqualDatesAccepted(t *testing.T) {
	_, errs := bind(t, "start_date: 2021-01-01\nend_date: 2021-01-01\n")
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
}

// Spec §5.23 — the rejection table.
func TestRejectionTable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "start_date aaa", input: "start_date: aaa\n", want: "This is not a valid date! Please use"},
		{name: "end_date aaa", input: "end_date: aaa\n", want: "This is not a valid date! Please use"},
		{name: "end_date invalid_end_date", input: "end_date: invalid_end_date\n", want: "This is not a valid date! Please use"},
		{name: "start_date 20222", input: "start_date: 20222\n", want: "This is not a valid date! Please use"},
		{name: "start_date 202222-20200", input: "start_date: 202222-20200\n", want: "This is not a valid date! Please use"},
		{name: "start_date 202222-12-20", input: "start_date: 202222-12-20\n", want: "This is not a valid date! Please use"},
		{name: "start_date 2022-20-20", input: "start_date: 2022-20-20\n", want: "month must be in 1..12"},
		{name: "start_date 2020-99-99", input: "start_date: 2020-99-99\n", want: "month must be in 1..12"},
		{name: "end_date 2020-99-99", input: "end_date: 2020-99-99\n", want: "month must be in 1..12"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := bind(t, tc.input)
			if len(errs) == 0 {
				t.Fatalf("errs = none, want a failure")
			}
			if !strings.HasPrefix(errs[0].Message, tc.want) {
				t.Errorf("message = %q, want it to start with %q", errs[0].Message, tc.want)
			}
		})
	}
}

// Spec §5.23 — the accept cases.
func TestRejectionTableAcceptCases(t *testing.T) {
	for _, input := range []string{
		"start_date: 2020-09-24\nend_date: 2021-09-24\n",
		"start_date: 2020-09\nend_date: present\n",
		"start_date: 2020\n",
		"date: Fall 2023\n",
	} {
		t.Run(input, func(t *testing.T) {
			if _, errs := bind(t, input); len(errs) != 0 {
				t.Errorf("errs = %+v, want none", errs)
			}
		})
	}
}
