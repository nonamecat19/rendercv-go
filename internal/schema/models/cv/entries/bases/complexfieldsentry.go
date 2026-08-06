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
var complexFields = []binder.Field{
	{Name: "start_date"},
	{Name: "end_date"},
	{Name: "location"},
	{Name: "summary"},
	{Name: "highlights"},
}

// ComplexFieldNames returns the five field names in declaration order
// (spec §3.79).
func ComplexFieldNames() []string {
	names := make([]string, 0, len(complexFields))
	for _, field := range complexFields {
		names = append(names, field.Name)
	}
	return names
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
func BindEntryWithComplexFields(
	node *yamldoc.Node,
	extraFields []binder.Field,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) (*BaseEntryWithComplexFields, []schemaerr.ValidationError) {
	fields := append(append([]binder.Field(nil), complexFields...), extraFields...)
	withDate, errs := BindEntryWithDate(node, fields, location, source)

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

		if err := ValidateExactDate(node.Raw, *field.isInteger, reference); err != nil {
			errs = append(errs, schemaerr.ValidationError{
				Code:           CodeDateValue,
				SchemaLocation: fieldLocation(location, field.name),
				YamlLocation:   spanOf(node),
				YamlSource:     source,
				Message:        err.Error(),
				Input:          node.Raw,
			})
			// An unparseable date cannot take part in the ordering check.
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
		Code:           CodeDateValue,
		SchemaLocation: append([]string(nil), location...),
		YamlSource:     source,
		Message: fmt.Sprintf(
			"`start_date` cannot be after `end_date`. The `start_date` is %s and the"+
				" `end_date` is %s.",
			e.StartDate, e.EndDate,
		),
		Input: "...",
	}
}
