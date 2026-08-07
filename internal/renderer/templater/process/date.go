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
	LastUpdated        string
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

// ComputeTimeSpan is `compute_time_span_string` (date.py:192-298).
//
// **Two branches, and the year-only one is not a special case of the other.**
// If either endpoint is year-only the span is whole years and the month half of
// the template is blanked; otherwise the arithmetic below runs.
//
// The arithmetic is upstream's own and none of it is calendar-correct:
//
//	years  = days / 365
//	months = (days % 365) / 30 + 1     // the +1 is unconditional
//	years += months / 12               // fold, so `1 year 12 months` cannot happen
//	months %= 12
//
// So a one-day span is **one month**, and the fold is what stops the twelve.
// Reproducing it means integer division on a day count, not `time.Time`
// arithmetic — a port using month deltas would differ on most inputs.
func ComputeTimeSpan(
	start, end string,
	startIsYearOnly, endIsYearOnly bool,
	catalog Catalog,
	current time.Time,
	timeSpanTemplate string,
) (string, error) {
	startDate, err := ParseDate(start, current)
	if err != nil {
		return "", err
	}
	endDate, err := ParseDate(end, current)
	if err != nil {
		return "", err
	}

	if startIsYearOnly || endIsYearOnly {
		// Neither endpoint can be more specific than a year, so the months half
		// is blanked and SubstitutePlaceholders' strip removes what is left.
		span := endDate.Year() - startDate.Year()
		count, word := "1", catalog.Year
		if span >= 2 {
			count, word = strconv.Itoa(span), catalog.Years
		}
		return SubstitutePlaceholders(timeSpanTemplate, map[string]string{
			"HOW_MANY_YEARS": count, "YEARS": word,
			"HOW_MANY_MONTHS": "", "MONTHS": "",
		}), nil
	}

	days := int(endDate.Sub(startDate).Hours() / 24)
	years := days / 365
	months := (days%365)/30 + 1
	years += months / 12
	months %= 12

	yearCount, yearWord := plural(years, catalog.Year, catalog.Years)
	monthCount, monthWord := plural(months, catalog.Month, catalog.Months)

	return SubstitutePlaceholders(timeSpanTemplate, map[string]string{
		"HOW_MANY_YEARS": yearCount, "YEARS": yearWord,
		"HOW_MANY_MONTHS": monthCount, "MONTHS": monthWord,
	}), nil
}

// plural blanks **both** the count and the word at zero, which is what makes the
// template collapse rather than print `0 years`, and uses the singular at one.
func plural(count int, singular, pluralWord string) (string, string) {
	switch count {
	case 0:
		return "", ""
	case 1:
		return "1", singular
	}
	return strconv.Itoa(count), pluralWord
}
