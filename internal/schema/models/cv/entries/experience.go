package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ExperienceEntry mirrors ExperienceEntry (entries/experience.py:17-18) and its
// own-field base BaseExperienceEntry (entries/experience.py:7-13).
type ExperienceEntry struct {
	bases.BaseEntryWithComplexFields

	Company  *yamldoc.Node
	Position *yamldoc.Node
}

// experienceOwnFields is the own-field order of spec §3.8. It precedes the
// base's fields because upstream declares
// `class ExperienceEntry(BaseEntryWithComplexFields, BaseExperienceEntry)` and
// pydantic emits the last-listed base first (experience.py:16-18, spec §3.2).
var experienceOwnFields = []binder.Field{
	{Name: "company", Required: true, Value: binder.ValueString},
	{Name: "position", Required: true, Value: binder.ValueString},
}

// experienceFields is the entry's whole declared field set, in upstream's order.
// Both the descriptor and the binder read this one value, so the order the
// registry advertises is by construction the order errors come out in
// (spec 004 §3.9a behavior 33c).
func experienceFields() []binder.Field {
	return bases.ComplexSpec(experienceOwnFields)
}

// ExperienceDescriptor is ExperienceEntry's registration. Its field set is its
// two own fields, then the inherited `date`, then the five complex fields —
// the verified runtime order of spec §3.8.
func ExperienceDescriptor() Descriptor {
	fields := make([]string, 0, len(experienceOwnFields)+1+len(bases.ComplexFieldNames()))
	for _, field := range experienceOwnFields {
		fields = append(fields, field.Name)
	}
	fields = append(fields, bases.DateFieldNames()...)
	fields = append(fields, bases.ComplexFieldNames()...)
	return Descriptor{Name: "ExperienceEntry", Fields: fields}
}

// ValidateExperienceEntry binds and validates one experience entry
// (entries/experience.py:7-18). The base owns `date`, `start_date`, `end_date`,
// `location`, `summary` and `highlights`, including the date precedence rules of
// spec 002 §3.77; the reference date is what `present` resolves to there.
func ValidateExperienceEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) (*ExperienceEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntryWithComplexFields(
		node, experienceFields(), location, source, reference,
	)

	entry := &ExperienceEntry{BaseEntryWithComplexFields: *base}
	for _, field := range []struct {
		name  string
		value **yamldoc.Node
	}{
		{name: "company", value: &entry.Company},
		{name: "position", value: &entry.Position},
	} {
		value, ok := base.Field(field.name)
		if ok && value != nil && value.Kind != yamldoc.KindNull {
			*field.value = value
		}
	}
	return entry, errs
}
