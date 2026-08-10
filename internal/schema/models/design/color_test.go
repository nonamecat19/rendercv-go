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

// **A tuple's elements were never range- or type-checked**, only its
// length — `[300, 0, 0]` and `[a, b, c]` both "validated" before this,
// where `ParseColor`'s string form already rejected the equivalent
// `rgb(300,0,0)`. Found by a fresh-context verifier (iteration 14's
// twelfth re-verification).
func TestParseColorTuple(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		want     string // "" means success
	}{
		{name: "a valid triple", elements: []string{"1", "2", "3"}, want: ""},
		{name: "a valid quad", elements: []string{"0", "0", "0", "0.5"}, want: ""},
		{
			name: "a channel over 255", elements: []string{"300", "0", "0"},
			want: "value is not a valid color: color values must be in the range 0 to 255",
		},
		{
			name: "a negative channel", elements: []string{"-1", "0", "0"},
			want: "value is not a valid color: color values must be in the range 0 to 255",
		},
		{
			name: "a non-numeric channel", elements: []string{"a", "b", "c"},
			want: "value is not a valid color: color values must be a valid number",
		},
		{
			name: "an out-of-range alpha", elements: []string{"0", "0", "0", "5"},
			want: "value is not a valid color: alpha values must be in the range 0 to 1",
		},
		{
			name: "the wrong length", elements: []string{"1", "2"},
			want: "value is not a valid color: tuples must have length 3 or 4",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, err := design.ParseColorTuple(test.elements)
			if test.want == "" {
				if err != nil {
					t.Fatalf("ParseColorTuple(%v) = %v, want success", test.elements, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseColorTuple(%v) = %v, want %q", test.elements, color, test.want)
			}
			if err.Error() != test.want {
				t.Errorf("= %q, want %q", err.Error(), test.want)
			}
		})
	}
}

// A valid tuple renders through `Effective` as `rgb(...)`, not a Go slice
// printed by the emitter — the failure mode the tenth and ninth passes'
// font_family findings share: a shape that validates but never gets
// converted before it reaches the template.
func TestSequenceColorRendersAsRGB(t *testing.T) {
	values := design.Effective("sb2nov", map[string]any{
		"colors": map[string]any{"name": []string{"1", "2", "3"}},
	})
	if got := design.EffectiveString(values, "colors", "name"); got != "rgb(1, 2, 3)" {
		t.Errorf("colors.name = %q, want rgb(1, 2, 3)", got)
	}
}

// **A tuple element written as a bool word or a hex/octal/binary integer
// literal still resolves.** `float(True)` is `1.0` in Python and
// `float(0x10)` is `16.0` — `strconv.ParseFloat` alone rejects both tokens,
// which is what let a document write `colors.name: [true, 0, 0]` and get
// rejected where upstream accepts it. Found by a fresh-context verifier
// (iteration 14's thirteenth re-verification).
func TestParseColorTupleAcceptsNonDecimalChannels(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		want     string
	}{
		{name: "a true channel", elements: []string{"true", "0", "0"}, want: "rgb(1, 0, 0)"},
		{name: "a false channel", elements: []string{"false", "0", "0"}, want: "rgb(0, 0, 0)"},
		{name: "a hex channel", elements: []string{"0x10", "0", "0"}, want: "rgb(16, 0, 0)"},
		{name: "an octal channel", elements: []string{"0o17", "0", "0"}, want: "rgb(15, 0, 0)"},
		{name: "a binary channel", elements: []string{"0b101", "0", "0"}, want: "rgb(5, 0, 0)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, err := design.ParseColorTuple(test.elements)
			if err != nil {
				t.Fatalf("ParseColorTuple(%v) = %v, want success", test.elements, err)
			}
			if got := color.String(); got != test.want {
				t.Errorf("= %q, want %q", got, test.want)
			}
		})
	}
}

// `parse_float_alpha` is also `float(value)` — the same coercion `parseChannel`
// already got in pass 13. Only `parseAlpha`'s non-percent branch still called
// `strconv.ParseFloat` directly. Found by a fresh-context verifier (iteration
// 14's fourteenth re-verification).
func TestParseColorTupleAcceptsNonDecimalAlpha(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		want     string
	}{
		// alpha of 1 (true) becomes absent, the same as any other alpha of 1.
		{name: "a true alpha", elements: []string{"1", "2", "3", "true"}, want: "rgb(1, 2, 3)"},
		{name: "a false alpha", elements: []string{"1", "2", "3", "false"}, want: "rgba(1, 2, 3, 0.0)"},
		{name: "a hex alpha", elements: []string{"1", "2", "3", "0x0"}, want: "rgba(1, 2, 3, 0.0)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, err := design.ParseColorTuple(test.elements)
			if err != nil {
				t.Fatalf("ParseColorTuple(%v) = %v, want success", test.elements, err)
			}
			if got := color.String(); got != test.want {
				t.Errorf("= %q, want %q", got, test.want)
			}
		})
	}
}

// The percent branch had not been routed through the same numeric coercion
// or whitespace trim as the plain-number branch. Found by a fresh-context
// verifier (iteration 14's fifteenth re-verification).
func TestParseColorTupleAcceptsWhitespaceInAPercentAlpha(t *testing.T) {
	color, err := design.ParseColorTuple([]string{"10", "20", "30", " 50%"})
	if err != nil {
		t.Fatalf("ParseColorTuple = %v, want success", err)
	}
	if got := color.String(); got != "rgba(10, 20, 30, 0.5)" {
		t.Errorf("= %q, want rgba(10, 20, 30, 0.5)", got)
	}
}

// A NaN channel or alpha is a range failure, not a silent pass-through —
// `value < 0 || value > max` is false in both directions for NaN, which is
// the *accept* condition here (inverted from Python's chained comparison),
// so it used to reach `int(NaN)` and print undefined-overflow garbage into
// the artifact. And a token upstream's YAML resolver would not classify as
// an integer at all — an uppercase-prefixed `0X1F`, `0O17` — must still fail
// as "not a number", not be accepted the way `strconv.ParseInt`'s
// case-insensitive base-0 parsing would take it. Found by a fresh-context
// verifier (iteration 14's fifteenth re-verification).
func TestParseColorTupleRejectsNaNAndCaseSensitivePrefixes(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
	}{
		{name: "a NaN channel", elements: []string{"nan", "0", "0"}},
		{name: "a NaN alpha", elements: []string{"1", "2", "3", "nan"}},
		{name: "an uppercase hex channel", elements: []string{"0X1F", "0", "0"}},
		{name: "an uppercase octal channel", elements: []string{"0O17", "0", "0"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := design.ParseColorTuple(test.elements); err == nil {
				t.Errorf("ParseColorTuple(%v) succeeded, want a failure", test.elements)
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
