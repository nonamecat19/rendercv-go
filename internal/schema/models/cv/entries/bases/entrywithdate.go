package bases

import (
	"regexp"
	"strconv"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

var (
	fullDatePattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthDatePattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
)

// dateFields is the one field BaseEntryWithDate adds
// (entry_with_date.py:42-50). It defaults to absent (spec §3.70).
var dateFields = []binder.Field{{
	Name:   "date",
	Value:  binder.ValueArbitraryDate,
	Scalar: func(raw string, _ bool) error { return ValidateArbitraryDate(raw) },
}}

// BaseEntryWithDate mirrors BaseEntryWithDate
// (schema/models/cv/entries/bases/entry_with_date.py:39-50).
type BaseEntryWithDate struct {
	BaseEntry

	// Date is the arbitrary date of spec §3.69, held as written: upstream
	// returns the original value and coerces nothing (entry_with_date.py:31).
	Date *yamldoc.Node
}

// DateSpec is the full declared field set of an entry built on
// BaseEntryWithDate: its own fields, then the inherited `date`. See ComplexSpec
// for why the composition is a shared builder rather than an inline append.
func DateSpec(own []binder.Field) []binder.Field {
	fields := make([]binder.Field, 0, len(own)+len(dateFields))
	fields = append(fields, own...)
	return append(fields, dateFields...)
}

// DateFieldNames returns the field names this base contributes.
//
// TODO(spec 004 A2): superseded by FieldNames once the descriptors read the
// shared field set.
func DateFieldNames() []string {
	return FieldNames(dateFields)
}

// FieldNames projects a field set onto its declared name order. It is how a
// Descriptor is derived from the very field set the binder is given, so the two
// cannot disagree.
func FieldNames(fields []binder.Field) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

// ValidateArbitraryDate mirrors validate_arbitrary_date
// (entry_with_date.py:10-31): a full `YYYY-MM-DD` must parse as a real date, a
// `YYYY-MM` must parse once `-01` is appended, and anything else — `Fall 2023`,
// `Summer 2020`, `2020` — is accepted unchanged.
//
// The value reaches this function as text; upstream stringifies an integer
// first (`str(date)`), so the integer `2020` takes the pass-through branch.
func ValidateArbitraryDate(value string) error {
	switch {
	case fullDatePattern.MatchString(value):
		return parseISODate(value)
	case monthDatePattern.MatchString(value):
		return parseISODate(value + "-01")
	default:
		return nil
	}
}

// parseISODate reproduces the range checks CPython's `date.fromisoformat`
// performs, in its order: year, then month, then day.
//
// TODO(iteration-4): spec §7.3, §5.11 — these three strings are CPython's, not
// RenderCV's, and are user-visible (spec §4.13). They are pinned here so a
// later decision to diverge is visible in a diff; the decision itself, and any
// divergence entry, is iteration 4's.
func parseISODate(value string) error {
	year, _ := strconv.Atoi(value[0:4])
	month, _ := strconv.Atoi(value[5:7])
	day, _ := strconv.Atoi(value[8:10])

	if year == 0 {
		return &DateRangeError{Message: "year 0 is out of range"}
	}
	if month < 1 || month > 12 {
		return &DateRangeError{Message: "month must be in 1..12"}
	}
	if day < 1 || day > daysInMonth(year, month) {
		return &DateRangeError{Message: "day is out of range for month"}
	}
	return nil
}

// DateRangeError is a structurally well-formed date whose components are out of
// range (spec §4.13). It is surfaced unwrapped by upstream, so its text is the
// date library's own.
type DateRangeError struct {
	Message string
}

func (e *DateRangeError) Error() string {
	return e.Message
}

func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// BindEntryWithDate binds an entry mapping that carries a `date`, validating it
// per spec §3.69 and retaining unknown keys per spec §3.67.
//
// spec is the entry's whole declared field set, as DateSpec builds it — the same
// value its descriptor is projected from. Iteration 3 composed the two
// separately and put `date` ahead of the own fields, which is item 1 of its cut
// scope; DateSpec is why that can no longer happen at one site and not the
// other.
func BindEntryWithDate(
	node *yamldoc.Node,
	spec []binder.Field,
	location []string,
	source schemaerr.YamlSource,
) (*BaseEntryWithDate, []schemaerr.ValidationError) {
	return bindEntryWithDateFields(node, spec, location, source)
}

// bindEntryWithDateFields binds an already-ordered field list. It exists so that
// BaseEntryWithComplexFields can place `date` between the own fields and the five
// complex ones — the order upstream produces — instead of having it forced to one
// end (spec 004 §3.9a behavior 33a).
func bindEntryWithDateFields(
	node *yamldoc.Node,
	fields []binder.Field,
	location []string,
	source schemaerr.YamlSource,
) (*BaseEntryWithDate, []schemaerr.ValidationError) {
	base, errs := BindEntry(node, fields, location, source)

	entry := &BaseEntryWithDate{BaseEntry: *base}
	date, ok := base.Field("date")
	if !ok || date == nil || date.Kind == yamldoc.KindNull {
		return entry, errs
	}

	// The `date` failure itself is emitted by the binder at the field's declared
	// position (spec 004 §3.9a behavior 33a). Appending it here, as iteration 3
	// did, put it after every other field's error.
	entry.Date = date
	return entry, errs
}
