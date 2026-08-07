package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// OneLineEntry mirrors OneLineEntry (entries/one_line.py:6-12). Its base is
// BaseEntry, which contributes no fields, so the type has no date fields at all
// (spec §5.19).
type OneLineEntry struct {
	bases.BaseEntry

	Label   *yamldoc.Node
	Details *yamldoc.Node
}

// oneLineOwnFields is the own-field order of spec §3.3: two required text
// fields, `label` before `details` (one_line.py:7, :10).
var oneLineOwnFields = []binder.Field{
	{Name: "label", Required: true, Value: binder.ValueString},
	{Name: "details", Required: true, Value: binder.ValueString},
}

// OneLineDescriptor is OneLineEntry's registration. Its field set is exactly its
// own fields, because BaseEntry declares none (one_line.py:6, spec §3.3).
func OneLineDescriptor() Descriptor {
	return Descriptor{Name: "OneLineEntry", Fields: bases.FieldNames(oneLineOwnFields)}
}

// ValidateOneLineEntry binds and validates one one-line entry
// (entries/one_line.py:6-12). The reference date is accepted for signature
// symmetry with the dated types and is unused: OneLineEntry has no date field
// (spec §5.19).
func ValidateOneLineEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	_ time.Time,
) (*OneLineEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntry(node, oneLineOwnFields, "OneLineEntry", location, source)

	entry := &OneLineEntry{BaseEntry: *base}
	if label, ok := base.Field("label"); ok && label != nil && label.Kind != yamldoc.KindNull {
		entry.Label = label
	}
	if details, ok := base.Field("details"); ok && details != nil && details.Kind != yamldoc.KindNull {
		entry.Details = details
	}
	return entry, errs
}
