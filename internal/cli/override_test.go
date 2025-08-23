package cli

import (
	"maps"
	"testing"
)

// TestParseOverrideArguments is spec 012 §2 behaviors 9, 11a and 11b.
//
// **The collector is not a dotted-key filter.** `render` is declared with
// `allow_extra_args` and `ignore_unknown_options` (`render_command.py:26`), so
// every token click does not recognize — unknown flags, stray positionals,
// single-dash words — lands in `ctx.args` in order and is read as alternating
// keys and values. The port collected only tokens containing a dot and left the
// rest to cobra, which rejected them with exit 70 and no output at all.
//
// Every expectation here was measured against the vendored CLI.
func TestParseOverrideArguments(t *testing.T) {
	cases := []struct {
		name      string
		extras    []string
		overrides map[string]string
		err       string
	}{
		{
			name:      "nothing extra",
			extras:    nil,
			overrides: map[string]string{},
		},
		{
			// `render_override_scalar`.
			name:      "dotted pair",
			extras:    []string{"--cv.phone", "+1-555-555-5555"},
			overrides: map[string]string{"cv.phone": "+1-555-555-5555"},
		},
		{
			// **A key with no dot is still a key.** Upstream accepts it and the
			// model then rejects `nope` as an unknown field; the port made it a
			// parser error.
			name:      "undotted key is collected",
			extras:    []string{"--nope", "value"},
			overrides: map[string]string{"nope": "value"},
		},
		{
			name:      "several pairs",
			extras:    []string{"--cv.name", "Jane", "--design.theme", "moderncv"},
			overrides: map[string]string{"cv.name": "Jane", "design.theme": "moderncv"},
		},
		{
			// `key.replace("--", "")` is unanchored (`:51`), so every occurrence
			// goes, not just the prefix. Measured: upstream accepts the argument
			// and the model rejects the field `cvname`.
			name:      "dashes are stripped everywhere",
			extras:    []string{"--cv--name", "Jane"},
			overrides: map[string]string{"cvname": "Jane"},
		},
		{
			// The join is `,` with no space (`:39`).
			name:   "odd count",
			extras: []string{"a", "b", "c"},
			err:    "There is a problem with the extra arguments (a,b,c)! Each key should have a corresponding value.",
		},
		{
			name:   "one stray positional",
			extras: []string{"extra.yaml"},
			err:    "There is a problem with the extra arguments (extra.yaml)! Each key should have a corresponding value.",
		},
		{
			// **The count is checked before the prefix**, so an odd vector of
			// bad keys reports the count rather than the first key (`:36`
			// precedes `:47`).
			name:   "single dash key",
			extras: []string{"-x", "value"},
			err:    "The key (-x) should start with double dashes!",
		},
		{
			name:   "bare word key",
			extras: []string{"cv.phone", "123"},
			err:    "The key (cv.phone) should start with double dashes!",
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			overrides, err := ParseOverrideArguments(row.extras)

			if row.err != "" {
				if err == nil {
					t.Fatalf("no error, want %q", row.err)
				}
				if err.Error() != row.err {
					t.Errorf("error = %q, want %q", err, row.err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !maps.Equal(overrides, row.overrides) {
				t.Errorf("overrides = %v, want %v", overrides, row.overrides)
			}
		})
	}
}
