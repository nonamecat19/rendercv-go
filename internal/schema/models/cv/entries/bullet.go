package entries

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries/bases"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// BulletEntry mirrors BulletEntry (entries/bullet.py:6-9). Its base is
// BaseEntry, which contributes no fields, so the type has no date fields at all
// (spec §5.19).
type BulletEntry struct {
	bases.BaseEntry

	Bullet *yamldoc.Node
}

// bulletOwnFields is the own-field order of spec §3.4: one required text field.
var bulletOwnFields = []binder.Field{
	{Name: "bullet", Required: true, Value: binder.ValueString},
}

// BulletDescriptor is BulletEntry's registration. Its field set is exactly its
// own fields, because BaseEntry declares none (bullet.py:6, spec §3.4).
func BulletDescriptor() Descriptor {
	return Descriptor{Name: "BulletEntry", Fields: bases.FieldNames(bulletOwnFields)}
}

// ValidateBulletEntry binds and validates one bullet entry
// (entries/bullet.py:6-9). The reference date is accepted for signature
// symmetry with the dated types and is unused: BulletEntry has no date field
// (spec §5.19).
func ValidateBulletEntry(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
	_ time.Time,
) (*BulletEntry, []schemaerr.ValidationError) {
	base, errs := bases.BindEntry(node, bulletOwnFields, "BulletEntry", location, source)

	entry := &BulletEntry{BaseEntry: *base}
	if bullet, ok := base.Field("bullet"); ok && bullet != nil && bullet.Kind != yamldoc.KindNull {
		entry.Bullet = bullet
	}
	return entry, errs
}
