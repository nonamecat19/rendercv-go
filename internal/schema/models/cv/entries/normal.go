package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// NormalEntry mirrors NormalEntry (entries/normal.py:14-15) and its own-field
// base BaseNormalEntry (entries/normal.py:7-10). The complex-field base
// contributes the six inherited fields (spec §3.7).
type NormalEntry struct {
	bases.BaseEntryWithComplexFields

	Name *yamldoc.Node
}

// normalOwnFields is the own-field order of spec §3.7: one required text field.
// It precedes the base's fields because upstream declares
// `class NormalEntry(BaseEntryWithComplexFields, BaseNormalEntry)` and pydantic
// emits the last-listed base's own fields first — so field order does not follow
// the MRO (normal.py:13-15, spec §3.2).
var normalOwnFields = []binder.Field{
	{Name: "name", Required: true, Value: binder.ValueString},
}

// normalFields is the entry's whole declared field set, in upstream's order.
// Both the descriptor and the binder read this one value, so the order the
// registry advertises is by construction the order errors come out in
// (spec 004 §3.9a behavior 33c).
func normalFields() []binder.Field {
	return bases.ComplexSpec(normalOwnFields)
}

// NormalDescriptor is NormalEntry's registration. Its field set is its own field
// then the inherited `date` then the five complex fields, which is the verified
// runtime order `name, date, start_date, end_date, location, summary,
// highlights` (spec §3.7).
func NormalDescriptor() Descriptor {
	return Descriptor{Name: "NormalEntry", Fields: bases.FieldNames(normalFields())}
}

// ValidateNormalEntry binds and validates one normal entry (normal.py:14-15).
// The date fields, their exact-date checks and the precedence steps of
// spec 002 §3.77 belong to the complex-field base, which is reached unchanged
// through this concrete type (spec §5.21).
func ValidateNormalEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	reference time.Time,
) (*NormalEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntryWithComplexFields(node, normalFields(), "NormalEntry", location, source, reference)

	entry := &NormalEntry{BaseEntryWithComplexFields: *base}
	if name, ok := base.Field("name"); ok && name != nil && name.Kind != yamldoc.KindNull {
		entry.Name = name
	}
	return entry, errs
}
