package design_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// Every row measured against the vendored Python's `str(Color(x))`, which is
// `as_rgb()` — the string the Typst templates receive and the string the JSON
// Schema shows as a colour field's default.
//
// The table is the parser's whole contract, so the rows that look redundant are
// the ones that are not: `fff` and `red` take different branches, `rgba(…,1)`
// drops its alpha, and `transparent` and `#ff00` print the same zero
// differently.
func TestParseColor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Named, case-insensitively.
		{"red", "rgb(255, 0, 0)"},
		{"RED", "rgb(255, 0, 0)"},
		{"Red", "rgb(255, 0, 0)"},
		{"Black", "rgb(0, 0, 0)"},

		// Hex, with every prefix and both lengths. The short form **doubles**
		// its digits rather than scaling them.
		{"#fff", "rgb(255, 255, 255)"},
		{"fff", "rgb(255, 255, 255)"},
		{"0x00ff00", "rgb(0, 255, 0)"},
		{"#ff0000", "rgb(255, 0, 0)"},
		{"ffffff", "rgb(255, 255, 255)"},
		{"7fffd4", "rgb(127, 255, 212)"},
		{"#ffff", "rgb(255, 255, 255)"},
		{"#ff000080", "rgba(255, 0, 0, 0.5)"},
		{"#0004", "rgba(0, 0, 0, 0.27)"},

		// rgb(), both the comma and the space-slash form.
		{"rgb(1,2,3)", "rgb(1, 2, 3)"},
		{"rgb(1, 2, 3)", "rgb(1, 2, 3)"},
		{"rgba(1,2,3,0.5)", "rgba(1, 2, 3, 0.5)"},
		{"rgba(1,2,3,50%)", "rgba(1, 2, 3, 0.5)"},
		{"rgb(1 2 3)", "rgb(1, 2, 3)"},
		{"rgb(1 2 3 / 0.5)", "rgba(1, 2, 3, 0.5)"},
		// An alpha of exactly 1 becomes absent, so the `a` does not survive.
		{"rgba(1,2,3,1)", "rgb(1, 2, 3)"},

		// hsl(), each hue unit, including a negative that Python's `%` wraps
		// forward and Go's `math.Mod` would not.
		{"hsl(120,50%,50%)", "rgb(64, 191, 64)"},
		{"hsl(270, 60%, 70%)", "rgb(178, 133, 224)"},
		{"hsl(-90, 50%, 50%)", "rgb(127, 64, 191)"},
		{"hsl(0.5turn, 50%, 50%)", "rgb(64, 191, 191)"},
		{"hsl(3.14rad, 50%, 50%)", "rgb(64, 191, 191)"},
		{"hsla(120,50%,50%,0.25)", "rgba(64, 191, 64, 0.25)"},

		// The two zero alphas, which print differently.
		{"transparent", "rgba(0, 0, 0, 0)"},
		{"#ff00", "rgba(255, 255, 0, 0.0)"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			color, err := design.ParseColor(tc.input)
			if err != nil {
				t.Fatalf("ParseColor(%q): %v", tc.input, err)
			}
			if got := color.String(); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

// The failures, with their exact texts. The first is the only one an ordinary
// design file reaches, and it is dictionary row 13's key.
func TestParseColorFailures(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"notacolor", design.MessageBadColor},
		{"#gggggg", design.MessageBadColor},
		// A percentage channel is a CSS form the library does not accept.
		{"rgb(50%,50%,50%)", design.MessageBadColor},
		// A bare tuple is not a colour string.
		{"(1,2,3)", design.MessageBadColor},
		// Surrounding space is **not** stripped for a name, though the patterns
		// allow it around a hex or a function form.
		{"  red  ", design.MessageBadColor},
		// hsl() requires its percent signs.
		{"hsl(120,50,50)", design.MessageBadColor},

		{"rgb(300,0,0)", "value is not a valid color: color values must be in the range 0 to 255"},
		{"rgba(0,0,0,5)", "value is not a valid color: alpha values must be in the range 0 to 1"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			_, err := design.ParseColor(tc.input)
			if err == nil {
				t.Fatalf("ParseColor(%q) succeeded, want %q", tc.input, tc.want)
			}
			if err.Error() != tc.want {
				t.Errorf("= %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

// The code is the library's, asserted as upstream's literal rather than as the
// Go constant.
func TestColorErrorCode(t *testing.T) {
	if design.CodeColor != "color_error" {
		t.Errorf("= %q, want %q", design.CodeColor, "color_error")
	}
}
