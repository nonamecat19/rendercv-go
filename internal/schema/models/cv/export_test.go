package cv

import "github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"

// SetElementValidatorForTest swaps a scalar-or-list field's element validator
// and returns a function restoring the previous one. It exists so the routing
// rule of spec §3.47 can be observed while the real validators are still
// iteration 4's pass-throughs.
func SetElementValidatorForTest(field string, validator ElementValidator) func() {
	previous := elementValidators[field]
	elementValidators[field] = validator
	return func() { elementValidators[field] = previous }
}

// SetEntryValidatorForTest swaps the entry validator and returns a function
// restoring the previous one. It exists so the section rules of spec
// §3.53–§3.61 can be observed while the concrete entry types are still
// iteration 3's.
func SetEntryValidatorForTest(validator EntryValidator) func() {
	previous := entryValidator
	entryValidator = validator
	return func() { entryValidator = previous }
}

// MappingKey reports a mapping node's value for a key, for tests that need to
// look inside an entry without importing the binder.
func MappingKey(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil, false
	}
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}
