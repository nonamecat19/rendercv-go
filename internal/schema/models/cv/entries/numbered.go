package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// NumberedEntry mirrors NumberedEntry
// (third_party/rendercv/src/rendercv/schema/models/cv/entries/numbered.py:6-9).
// Its base is BaseEntry, which declares no fields, so the type has no date
// fields at all: a `date` written on one is an unknown key (spec 003 §5 edge
// case 19).
//
// Number is a *yamldoc.Node rather than a string because iteration 4 needs the
// value's span to place its error and iteration 8 needs the raw text with no
// normalization (plan §3.1).
type NumberedEntry struct {
	bases.BaseEntry

	Number *yamldoc.Node
}

// numberedOwnFields is the own-field order of spec 003 §3.5: one required text
// field, no description metadata (numbered.py:7-9).
var numberedOwnFields = []binder.Field{
	{Name: "number", Required: true, Value: binder.ValueString},
}

// NumberedDescriptor returns the registration of NumberedEntry. Its field set
// is the own fields alone, because BaseEntry contributes none
// (entries/bases/entry.py:11-18).
func NumberedDescriptor() Descriptor {
	return Descriptor{Name: "NumberedEntry", Fields: bases.FieldNames(numberedOwnFields)}
}

// ValidateNumberedEntry binds and validates one numbered entry. The reference
// date is part of every entry validator's signature so the dispatcher of plan
// §3.3 can call them uniformly; NumberedEntry has no date field to apply it to.
func ValidateNumberedEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	_ time.Time,
) (*NumberedEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntry(node, numberedOwnFields, location, source)

	entry := &NumberedEntry{BaseEntry: *base}
	entry.Number, _ = base.Field("number")
	return entry, errs
}
