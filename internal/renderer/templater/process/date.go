package process

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Catalog is the slice of the locale a date needs. It is an interface-free
// struct rather than a dependency on `models/locale` because `process` is
// downstream of the schema and the renderer should not reach back into it —
// `model.go` fills this in from the validated catalog.
type Catalog struct {
	MonthNames         []string
	MonthAbbreviations []string
	Present            string
	Year, Years        string
	Month, Months      string
}

// DateTemplates is the three `design.templates` strings a date reads.
type DateTemplates struct {
	SingleDate string
	DateRange  string
	TimeSpan   string
}

// ErrNotADate is `get_date_object`'s failure. Only FormatSingleDate catches it;
// see its doc comment.
var ErrNotADate = errors.New("not a date")

var (
	fullDatePattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthDatePattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	yearDatePattern  = regexp.MustCompile(`^\d{4}$`)
)

// ParseDate is `get_date_object` (entry_with_complex_fields.py:43-90): four
// accepted shapes plus `present`.
//
// A year and a `YYYY-MM` both become the **first** of their period, which is
// what makes the time-span arithmetic of ComputeTimeSpan reproducible.
func ParseDate(value string, current time.Time) (time.Time, error) {
	switch {
	case fullDatePattern.MatchString(value):
		return time.Parse("2006-01-02", value)
	case monthDatePattern.MatchString(value):
		return time.Parse("2006-01-02", value+"-01")
	case yearDatePattern.MatchString(value):
		return time.Parse("2006-01-02", value+"-01-01")
	case value == "present":
		return current, nil
	}
	return time.Time{}, fmt.Errorf("%w: %q", ErrNotADate, value)
}

// BuildDatePlaceholders is `build_date_placeholders` (date.py:12-39).
//
// **The month lookups are the only consumer of spec 007's twelve-element
// lists**, which is why their order is contractual there.
//
// `YEAR_IN_TWO_DIGITS` is `str(year)[-2:]` — a slice, not a modulus — so it is
// the last two characters of the printed year and a year below 10 gives one
// digit rather than a zero-padded two.
func BuildDatePlaceholders(date time.Time, catalog Catalog) map[string]string {
	year := strconv.Itoa(date.Year())
	twoDigits := year
	if len(twoDigits) > 2 {
		twoDigits = twoDigits[len(twoDigits)-2:]
	}

	month := int(date.Month())
	return map[string]string{
		"MONTH_NAME":          catalog.MonthNames[month-1],
		"MONTH_ABBREVIATION":  catalog.MonthAbbreviations[month-1],
		"MONTH":               strconv.Itoa(month),
		"MONTH_IN_TWO_DIGITS": fmt.Sprintf("%02d", month),
		"DAY":                 strconv.Itoa(date.Day()),
		"DAY_IN_TWO_DIGITS":   fmt.Sprintf("%02d", date.Day()),
		"YEAR":                year,
		"YEAR_IN_TWO_DIGITS":  twoDigits,
	}
}

// FormatDate is `date_object_to_string` (date.py:42-71): the eight placeholders
// through the theme's `single_date` template, which means
// SubstitutePlaceholders' strip applies.
func FormatDate(date time.Time, catalog Catalog, singleDateTemplate string) string {
	return SubstitutePlaceholders(singleDateTemplate, BuildDatePlaceholders(date, catalog))
}

// FormatDateRange is `format_date_range` (date.py:74-140).
//
// **A bare year is never run through `single_date`** (`:110-112`, `:125-126`):
// `2020` stays `2020` rather than becoming `Jan 2020`, because upstream tests
// `isinstance(x, int)` and a year-only YAML scalar arrives as an int. The port
// carries `yearOnly` flags rather than re-deriving that from the string, since
// `"2020"` quoted in YAML is a *string* to upstream and does go through the
// template.
//
// **There is no custom-string fallback here.** An unparseable endpoint returns
// an error, where FormatSingleDate would pass it through — the asymmetry that
// makes `"Spring 2024"` legal in a publication date and illegal in a range.
func FormatDateRange(
	start, end string,
	startIsYearOnly, endIsYearOnly bool,
	catalog Catalog,
	templates DateTemplates,
) (string, error) {
	startText := start
	if !startIsYearOnly {
		date, err := ParseDate(start, time.Time{})
		if err != nil {
			return "", err
		}
		startText = FormatDate(date, catalog, templates.SingleDate)
	}

	endText := end
	switch {
	case end == "present":
		endText = catalog.Present
	case endIsYearOnly:
	default:
		date, err := ParseDate(end, time.Time{})
		if err != nil {
			return "", err
		}
		endText = FormatDate(date, catalog, templates.SingleDate)
	}

	return SubstitutePlaceholders(templates.DateRange, map[string]string{
		"START_DATE": startText,
		"END_DATE":   endText,
	}), nil
}

// FormatSingleDate is `format_single_date` (date.py:143-189), and it is the one
// formatter with a fallback: a value `get_date_object` rejects **passes through
// unchanged**, which is what makes `Spring 2024` a legal publication date.
//
// The order of the three tests is upstream's: year-only first, then `present`,
// then parse-or-pass-through.
func FormatSingleDate(
	value string,
	isYearOnly bool,
	catalog Catalog,
	singleDateTemplate string,
) string {
	if isYearOnly {
		return value
	}
	if value == "present" {
		return catalog.Present
	}
	date, err := ParseDate(value, time.Time{})
	if err != nil {
		return value
	}
	return FormatDate(date, catalog, singleDateTemplate)
}
