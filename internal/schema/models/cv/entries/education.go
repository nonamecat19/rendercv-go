package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// EducationEntry mirrors EducationEntry (entries/education.py:26-27), whose own
// fields come from BaseEducationEntry (education.py:7-22) and whose date,
// location, summary and highlights come from BaseEntryWithComplexFields.
type EducationEntry struct {
	bases.BaseEntryWithComplexFields

	Institution *yamldoc.Node
	Area        *yamldoc.Node

	// Degree is optional (`degree: str | None = None`, education.py:19-22) and is
	// nil when the key is absent or written null — absent, not empty text
	// (spec §5.18, §5.22).
	Degree *yamldoc.Node
}

// educationOwnFields is the own-field order of spec §3.9: `institution` and
// `area` required, `degree` optional (education.py:8, :11, :19).
var educationOwnFields = []binder.Field{
	{Name: "institution", Required: true, Value: binder.ValueString},
	{Name: "area", Required: true, Value: binder.ValueString},
	{Name: "degree", Value: binder.ValueString},
}

// EducationDescriptor is EducationEntry's registration. The own fields come
// first even though BaseEntryWithComplexFields is the first-listed base:
// pydantic emits the last-listed base's own fields first, which is exactly why
// upstream lists the bases in that order (education.py:25). So the field set is
// the three own fields, then `date`, then the five complex fields (spec §3.9).
func EducationDescriptor() Descriptor {
	fields := make([]string, 0, len(educationOwnFields)+1+5)
	for _, field := range educationOwnFields {
		fields = append(fields, field.Name)
	}
	fields = append(fields, bases.DateFieldNames()...)
	fields = append(fields, bases.ComplexFieldNames()...)
	return Descriptor{Name: "EducationEntry", Fields: fields}
}

// ValidateEducationEntry binds and validates one education entry
// (entries/education.py:7-27). The base owns `date`, `start_date`, `end_date`,
// `location`, `summary` and `highlights`, including the date precedence rules of
// spec 002 §3.77; the reference date is what `present` resolves to there.
func ValidateEducationEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) (*EducationEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntryWithComplexFields(
		node,
		educationOwnFields,
		location,
		source,
		reference,
	)

	entry := &EducationEntry{BaseEntryWithComplexFields: *base}
	for _, field := range []struct {
		name   string
		target **yamldoc.Node
	}{
		{name: "institution", target: &entry.Institution},
		{name: "area", target: &entry.Area},
		{name: "degree", target: &entry.Degree},
	} {
		value, ok := base.Field(field.name)
		if ok && value != nil && value.Kind != yamldoc.KindNull {
			*field.target = value
		}
	}
	return entry, errs
}
