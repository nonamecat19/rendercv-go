package process_test

import (
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// english is `EnglishLocale`'s date slice. `June`, `July` and `Sept` are four
// letters because they come from Yale's cataloguing table, which spec 007 §2
// behavior 7 measured — so `2020-06` under `MONTH_ABBREVIATION YEAR` is
// `June 2020` and not `Jun 2020`.
var english = process.Catalog{
	MonthNames: []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	},
	MonthAbbreviations: []string{
		"Jan", "Feb", "Mar", "Apr", "May", "June",
		"July", "Aug", "Sept", "Oct", "Nov", "Dec",
	},
	Present: "present",
	Year:    "year", Years: "years",
	Month: "month", Months: "months",
}

var templates = process.DateTemplates{
	SingleDate: "MONTH_ABBREVIATION YEAR",
	DateRange:  "START_DATE – END_DATE",
	TimeSpan:   "HOW_MANY_YEARS YEARS HOW_MANY_MONTHS MONTHS",
}

func TestBuildDatePlaceholders(t *testing.T) {
	got := process.BuildDatePlaceholders(time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC), english)
	want := map[string]string{
		"MONTH_NAME": "March", "MONTH_ABBREVIATION": "Mar",
		"MONTH": "3", "MONTH_IN_TWO_DIGITS": "03",
		"DAY": "5", "DAY_IN_TWO_DIGITS": "05",
		"YEAR": "2025", "YEAR_IN_TWO_DIGITS": "25",
	}
	for key, wanted := range want {
		if got[key] != wanted {
			t.Errorf("%s = %q, want %q", key, got[key], wanted)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d placeholders, want %d", len(got), len(want))
	}
}

// `YEAR_IN_TWO_DIGITS` is a **slice** of the printed year, not a modulus, so
// year 7 gives `7` rather than `07`. Measured.
func TestYearInTwoDigitsIsASlice(t *testing.T) {
	got := process.BuildDatePlaceholders(time.Date(7, 1, 9, 0, 0, 0, 0, time.UTC), english)
	if got["YEAR"] != "7" || got["YEAR_IN_TWO_DIGITS"] != "7" {
		t.Errorf("YEAR = %q, YEAR_IN_TWO_DIGITS = %q, want %q and %q",
			got["YEAR"], got["YEAR_IN_TWO_DIGITS"], "7", "7")
	}
	// The day *is* zero-padded, which is what makes the year's not being so a
	// deliberate difference rather than a shared convention.
	if got["DAY_IN_TWO_DIGITS"] != "09" {
		t.Errorf("DAY_IN_TWO_DIGITS = %q, want %q", got["DAY_IN_TWO_DIGITS"], "09")
	}
}

func TestFormatDateRange(t *testing.T) {
	tests := []struct {
		name                   string
		start, end             string
		startYearly, endYearly bool
		want                   string
	}{
		{
			// June's abbreviation is four letters; see `english` above.
			name: "a month range", start: "2020-06", end: "2023-09",
			want: "June 2020 – Sept 2023",
		},
		{
			name: "present", start: "2020-06", end: "present",
			want: "June 2020 – present",
		},
		{
			// **Neither endpoint goes through `single_date`.** A port that
			// formatted them would emit `Jan 2020 – Jan 2023`.
			name: "two bare years", start: "2020", end: "2023",
			startYearly: true, endYearly: true,
			want: "2020 – 2023",
		},
		{
			name: "a bare year against a month", start: "2020", end: "2023-09",
			startYearly: true,
			want:        "2020 – Sept 2023",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := process.FormatDateRange(
				tc.start, tc.end, tc.startYearly, tc.endYearly, english, templates)
			if err != nil {
				t.Fatalf("FormatDateRange: %v", err)
			}
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// The asymmetry between the two formatters: a range **rejects** what a single
// date passes through.
func TestOnlyASingleDateFallsBack(t *testing.T) {
	if got := process.FormatSingleDate("Spring 2024", false, english, templates.SingleDate); got != "Spring 2024" {
		t.Errorf("FormatSingleDate = %q, want it unchanged", got)
	}
	if _, err := process.FormatDateRange("Spring 2024", "2025", false, true, english, templates); err == nil {
		t.Error("FormatDateRange accepted a custom string; upstream raises")
	}
}

func TestFormatSingleDate(t *testing.T) {
	tests := []struct {
		value    string
		yearOnly bool
		want     string
	}{
		{"2024-03", false, "Mar 2024"},
		{"2024-03-15", false, "Mar 2024"},
		{"2024", true, "2024"},
		{"present", false, "present"},
		{"Spring 2024", false, "Spring 2024"},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			got := process.FormatSingleDate(tc.value, tc.yearOnly, english, templates.SingleDate)
			if got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}
