package bases

import (
	"fmt"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// complexFields is the declaration order of spec §3.79, after the inherited
// `date` (entry_with_complex_fields.py:93-132). All are optional and default to
// absent.
//
// The declared shapes are upstream's: `location: str | None` and
// `summary: str | None` (entry_with_complex_fields.py:106-119), and
// `highlights: list[str] | None` (:120-132). Iteration 2 bound all three as raw
// nodes with no check; spec 003 §3.13 behavior 25 closes that.
//
// `start_date` and `end_date` stay ValueAny because their own validators own
// them: they are `str | int` upstream, so an int is legal and the date checks of
// spec §3.71 report the failures.
var complexFields = []binder.Field{
	{Name: "start_date", Value: binder.ValueExactDate},
	{Name: "end_date", Value: binder.ValueExactDateOrPresent},
	{Name: "location", Value: binder.ValueString},
	{Name: "summary", Value: binder.ValueString},
	{Name: "highlights", Value: binder.ValueStringList},
}

// ComplexSpec is the full declared field set of an entry built on
// BaseEntryWithComplexFields: its own fields, then the inherited `date`, then
// the five complex fields.
//
// That is upstream's order for `class X(BaseEntryWithComplexFields, BaseX)` —
// own fields first, because pydantic emits the last-listed base's fields first
// (spec 003 §3.2, verified per type against `model_fields.keys()`).
//
// It is a shared builder rather than an inline `append` at each site because the
// descriptor and the binder must read the *same* order, and nothing but shared
// construction guarantees that. Iteration 3 composed the two separately and one
// of them came out backwards (spec 004 §3.9a behavior 33a); behavior 33c makes
// the order a prerequisite of dedup rather than a cosmetic property, so the
// drift is closed here by construction instead of by a test downstream.
func ComplexSpec(own []binder.Field) []binder.Field {
	fields := make([]binder.Field, 0, len(own)+len(dateFields)+len(complexFields))
	fields = append(fields, own...)
	fields = append(fields, dateFields...)
	return append(fields, complexFields...)
}

// withExactDateValidators binds the two exact-date checks to a reference date.
// They cannot live in the package-level declaration because `present` resolves
// against the reference (entry_with_complex_fields.py:78-83), which is
// per-validation.
func withExactDateValidators(spec []binder.Field, reference time.Time) []binder.Field {
	fields := append([]binder.Field(nil), spec...)
	for i := range fields {
		switch fields[i].Name {
		case "start_date", "end_date":
			fields[i].Scalar = func(raw string, isInteger bool) error {
				return ValidateExactDate(raw, isInteger, reference)
			}
		}
	}
	return fields
}

// BaseEntryWithComplexFields mirrors BaseEntryWithComplexFields
// (entry_with_complex_fields.py:89-171).
//
// Date, StartDate and EndDate hold the values as the user wrote them, because
// spec §4.16 interpolates the user's own spelling and steps 1-3 of spec §3.77
// move those spellings around rather than parsed dates.
type BaseEntryWithComplexFields struct {
	BaseEntryWithDate

	StartDate  string
	EndDate    string
	DateValue  string
	Location   *yamldoc.Node
	Summary    *yamldoc.Node
	Highlights *yamldoc.Node

	startDateIsInteger bool
	endDateIsInteger   bool
	dateIsInteger      bool
}

// BindEntryWithComplexFields binds an entry carrying the five complex fields
// plus the inherited `date`, validates each exact date (spec §3.71), then runs
// the four precedence steps of spec §3.77.
// spec is the entry's whole declared field set, as ComplexSpec builds it — the
// same value its descriptor is projected from, so the binder cannot be fed a
// different order than the one the descriptor advertises.
func BindEntryWithComplexFields(
	node *yamldoc.Node,
	spec []binder.Field,
	model string,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) (*BaseEntryWithComplexFields, []schemaerr.ValidationError) {
	withDate, errs := bindEntryWithDateFields(
		node, withExactDateValidators(spec, reference), model, location, source,
	)

	entry := &BaseEntryWithComplexFields{BaseEntryWithDate: *withDate}
	entry.Location, _ = withDate.Field("location")
	entry.Summary, _ = withDate.Field("summary")
	entry.Highlights, _ = withDate.Field("highlights")

	if withDate.Date != nil {
		entry.DateValue = withDate.Date.Raw
		entry.dateIsInteger = withDate.Date.Kind == yamldoc.KindInt
	}

	for _, field := range []struct {
		name      string
		value     *string
		isInteger *bool
	}{
		{name: "start_date", value: &entry.StartDate, isInteger: &entry.startDateIsInteger},
		{name: "end_date", value: &entry.EndDate, isInteger: &entry.endDateIsInteger},
	} {
		node, ok := withDate.Field(field.name)
		if !ok || node == nil || node.Kind == yamldoc.KindNull {
			continue
		}
		*field.value = node.Raw
		*field.isInteger = node.Kind == yamldoc.KindInt

		// The failure itself is emitted by the binder at the field's declared
		// position (spec 004 §3.9a behavior 33a). What remains here is the
		// consequence: a date that did not parse cannot take part in the
		// start-after-end comparison, which is a cross-field rule and stays after
		// the field pass.
		if ValidateExactDate(node.Raw, *field.isInteger, reference) != nil {
			*field.value = ""
		}
	}

	if err := entry.adjustDates(location, source, reference); err != nil {
		errs = append(errs, *err)
	}
	return entry, errs
}

// adjustDates mirrors check_and_adjust_dates
// (entry_with_complex_fields.py:133-171). Steps 1-3 are silent rewrites; only
// step 4 can fail (spec §3.78).
func (e *BaseEntryWithComplexFields) adjustDates(
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) *schemaerr.ValidationError {
	switch {
	case e.DateValue != "":
		// Step 1: `date` silently wins (spec §5.22).
		e.StartDate = ""
		e.EndDate = ""
	case e.StartDate == "" && e.EndDate != "":
		// Step 2: a lone `end_date` migrates to `date` (spec §5.21).
		e.DateValue = e.EndDate
		e.dateIsInteger = e.endDateIsInteger
		e.StartDate = ""
		e.EndDate = ""
	case e.StartDate != "" && e.EndDate == "":
		// Step 3: a lone `start_date` implies an ongoing event.
		e.EndDate = "present"
		e.endDateIsInteger = false
	}

	if e.StartDate == "" || e.EndDate == "" {
		return nil
	}

	// Step 4: the ordering check, the only step that can fail. A date that did
	// not parse has already been reported against its own field, so it takes no
	// part here — the ordering check is skipped rather than reported twice.
	start, startErr := GetDateObject(e.StartDate, e.startDateIsInteger, reference)
	end, endErr := GetDateObject(e.EndDate, e.endDateIsInteger, reference)
	if startErr != nil || endErr != nil {
		//nolint:nilerr // deliberate: the failure was already reported against
		// the field it came from, so the ordering check is skipped rather than
		// reporting the same date twice.
		return nil
	}
	if !start.After(end) {
		return nil
	}

	return &schemaerr.ValidationError{
		Code:           CodeDateOther,
		SchemaLocation: append([]string(nil), location...),
		YamlSource:     source,
		Message: fmt.Sprintf(
			"`start_date` cannot be after `end_date`. The `start_date` is %s and the"+
				" `end_date` is %s.",
			e.StartDate, e.EndDate,
		),
		Input: schemaerr.InputEllipsis,
	}
}
