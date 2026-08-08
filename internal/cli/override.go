package cli

// ParseOverrideArguments is `parse_override_arguments`
// (`render_command/parse_override_arguments.py:6-55`).
//
// **The body is deliberately empty**, so the tests that state upstream's three
// rules are red until the next unit implements them.
func ParseOverrideArguments(extras []string) (map[string]string, error) {
	_ = extras
	return map[string]string{}, nil
}
