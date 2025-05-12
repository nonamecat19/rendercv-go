package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// ReversedNumberedEntry mirrors ReversedNumberedEntry
// (entries/reversed_numbered.py:6-13). Its base is BaseEntry, which contributes
// no fields, so the type has no date fields at all (spec §5.19).
type ReversedNumberedEntry struct {
	bases.BaseEntry

	ReversedNumber *yamldoc.Node
}

// ReversedNumberDescription is the `reversed_number` description metadata
// (entries/reversed_numbered.py:9-11, spec §4.7). It is the only own-field
// description among the four BaseEntry types, and iteration 5 emits it verbatim
// into the JSON Schema.
const ReversedNumberDescription = "Reverse-numbered list item. Numbering goes in reverse" +
	" (5, 4, 3, 2, 1), making recent items have higher numbers."

// reversedNumberedOwnFields is the own-field order of spec §3.6: one required
// text field.
var reversedNumberedOwnFields = []binder.Field{
	{Name: "reversed_number", Required: true, Value: binder.ValueString},
}

// ReversedNumberedDescriptor is ReversedNumberedEntry's registration. Its field
// set is exactly its own fields, because BaseEntry declares none
// (reversed_numbered.py:6, spec §3.6).
func ReversedNumberedDescriptor() Descriptor {
	return Descriptor{Name: "ReversedNumberedEntry", Fields: bases.FieldNames(reversedNumberedOwnFields)}
}

// ValidateReversedNumberedEntry binds and validates one reversed-numbered entry
// (entries/reversed_numbered.py:6-13). The reference date is accepted for
// signature symmetry with the dated types and is unused: ReversedNumberedEntry
// has no date field (spec §5.19).
func ValidateReversedNumberedEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	_ time.Time,
) (*ReversedNumberedEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntry(node, reversedNumberedOwnFields, "ReversedNumberedEntry", location, source)

	entry := &ReversedNumberedEntry{BaseEntry: *base}
	number, ok := base.Field("reversed_number")
	if ok && number != nil && number.Kind != yamldoc.KindNull {
		entry.ReversedNumber = number
	}
	return entry, errs
}
