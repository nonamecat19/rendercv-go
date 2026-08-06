package cv

// SetElementValidatorForTest swaps a scalar-or-list field's element validator
// and returns a function restoring the previous one. It exists so the routing
// rule of spec §3.47 can be observed while the real validators are still
// iteration 4's pass-throughs.
func SetElementValidatorForTest(field string, validator ElementValidator) func() {
	previous := elementValidators[field]
	elementValidators[field] = validator
	return func() { elementValidators[field] = previous }
}
