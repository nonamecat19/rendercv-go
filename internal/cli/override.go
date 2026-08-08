package cli

import (
	"fmt"
	"strings"
)

// ParseOverrideArguments is `parse_override_arguments`
// (`render_command/parse_override_arguments.py:6-55`).
//
// **`render` declares `allow_extra_args` and `ignore_unknown_options`**
// (`render_command.py:26`), so click hands it every token it did not recognize,
// in order, and this reads that list as alternating keys and values. Nothing
// filters on whether a key looks like a schema path: `--nope value` is the
// override key `nope`, and the *model* is what rejects it, which is the
// distinction `err_bad_override_key` measures.
func ParseOverrideArguments(extras []string) (map[string]string, error) {
	// The count is checked before any key is looked at (`:36` precedes `:47`),
	// so an odd vector of malformed keys reports the count rather than the
	// first key. The join is `,` with no space.
	if len(extras)%2 != 0 {
		return nil, fmt.Errorf( //nolint:staticcheck // upstream's text
			"There is a problem with the extra arguments (%s)! Each key should have a corresponding value.",
			strings.Join(extras, ","),
		)
	}

	overrides := make(map[string]string, len(extras)/2)
	for i := 0; i < len(extras); i += 2 {
		key, value := extras[i], extras[i+1]
		if !strings.HasPrefix(key, "--") {
			return nil, fmt.Errorf( //nolint:staticcheck // upstream's text
				"The key (%s) should start with double dashes!", key)
		}
		// **Unanchored** (`:51`): `--cv--name` becomes `cvname`, not `cv--name`.
		overrides[strings.ReplaceAll(key, "--", "")] = value
	}
	return overrides, nil
}
