package bases

import (
	"strings"
	"unicode"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// The two codes a rejected date can carry. They differ because upstream raises
// them through different mechanisms, and iteration 4's rewrite table keys off the
// code, so collapsing them would build the message rewriter on a wrong key.
const (
	// CodeDateValue is the arbitrary `date` field's code. `validate_arbitrary_date`
	// lets `Date.fromisoformat` raise a bare Python ValueError
	// (entry_with_date.py:26-29), which pydantic reports as `value_error`.
	// Verified: `NormalEntry(name="n", date="2020-20-20")` → `value_error`.
	CodeDateValue schemaerr.Code = "value_error"

	// CodeDateOther is the code for `start_date`/`end_date`. Both
	// `validate_exact_date` (entry_with_complex_fields.py:31-36) and the
	// start-after-end check (:161-169) raise
	// `PydanticCustomError(CustomPydanticErrorTypes.other, …)` instead of letting a
	// ValueError through. Verified: `NormalEntry(name="n", start_date="notadate")`
	// → `rendercv_other_error`, and so does the start-after-end case.
	CodeDateOther schemaerr.Code = "rendercv_other_error"
)

// BaseEntry mirrors BaseEntry (entries/bases/entry.py:11-18). It inherits the
// extra-keys base (base.py:9), which is the arbitrary-keys feature of spec
// §3.67, not an oversight: any unknown key a user writes is retained and
// readable.
type BaseEntry struct {
	fields map[string]*yamldoc.Node
	extras []yamldoc.Item
}

// BindEntry binds an entry mapping with the given declared fields, keeping
// unknown keys (spec §3.67, §5.24).
func BindEntry(
	node *yamldoc.Node,
	fields []binder.Field,
	location []string,
	source schemaerr.YamlSource,
) (*BaseEntry, []schemaerr.ValidationError) {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: fields, Policy: binder.AllowExtra},
		location,
		source,
	)
	return &BaseEntry{fields: result.Values, extras: result.Extras}, errs
}

// Field reports a declared field's node and whether the key was present.
func (e *BaseEntry) Field(name string) (*yamldoc.Node, bool) {
	if e == nil {
		return nil, false
	}
	value, ok := e.fields[name]
	return value, ok
}

// Extra reports an unknown key's node and whether the user wrote it
// (spec §3.67).
func (e *BaseEntry) Extra(name string) (*yamldoc.Node, bool) {
	if e == nil {
		return nil, false
	}
	for _, item := range e.extras {
		if item.Key == name {
			return item.Value, true
		}
	}
	return nil, false
}

// ExtraKeys returns the unknown keys in input order.
func (e *BaseEntry) ExtraKeys() []string {
	if e == nil {
		return nil
	}
	keys := make([]string, 0, len(e.extras))
	for _, item := range e.extras {
		keys = append(keys, item.Key)
	}
	return keys
}

// EntryTypeInSnakeCase derives an entry type's snake-case name by inserting an
// underscore before each uppercase letter that is not first (spec §3.68).
func EntryTypeInSnakeCase(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
