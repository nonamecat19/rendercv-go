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

// **`round(x, 2)` rounds the exact binary value, not `x*100` rounded then
// divided.** `0.075` is not exactly `0.075` in float64 — its true value sits
// fractionally below — so Python rounds it down to `0.07`; multiplying by
// 100 first lands on exactly `7.5` and rounds up to `0.08`, the wrong
// answer. And an alpha that only *rounds* to 1 (not one that's already
// close to it, which never reaches this code at all) is still a Python
// `float`, so it prints `1.0`, not `1`. Found by a fresh-context verifier
// (iteration 14's colour-slice sweep).
func TestAlphaRoundsOnTheExactBinaryValue(t *testing.T) {
	tests := []struct {
		alpha string
		want  string
	}{
		{"0.075", "rgba(1, 2, 3, 0.07)"},
		{"0.005", "rgba(1, 2, 3, 0.01)"},
		{"0.015", "rgba(1, 2, 3, 0.01)"},
		{"0.025", "rgba(1, 2, 3, 0.03)"},
		{"0.065", "rgba(1, 2, 3, 0.07)"},
		{"0.085", "rgba(1, 2, 3, 0.09)"},
		{"0.155", "rgba(1, 2, 3, 0.15)"},
		{"0.265", "rgba(1, 2, 3, 0.27)"},
		{"0.996", "rgba(1, 2, 3, 1.0)"},
		// **`-0.0 == 0` is true in Go, and equality alone throws the sign
		// away.** Python's `str(round(-0.0, 2))` is `'-0.0'`. Found by a
		// fresh-context verifier (iteration 14's nineteenth
		// re-verification).
		{"-0.0", "rgba(1, 2, 3, -0.0)"},
		// **A plain integer `-0` has no negative zero in Python** —
		// `int(-0) == 0` and `float(0)` is always `+0.0` — where a float
		// or a quoted string does. Regression: the `-0.0` fix above,
		// applied without this distinction, printed `-0.0` for an
		// int-sourced `-0` too. Found by a fresh-context verifier
		// (iteration 14's twentieth re-verification).
		{"-0", "rgba(1, 2, 3, 0.0)"},
	}
	for _, test := range tests {
		t.Run(test.alpha, func(t *testing.T) {
			color, err := design.ParseColorTuple([]string{"1", "2", "3", test.alpha})
			if err != nil {
				t.Fatalf("ParseColorTuple = %v, want success", err)
			}
			if got := color.String(); got != test.want {
				t.Errorf("= %q, want %q", got, test.want)
			}
		})
	}
}

// **Python's `float(str)` transliterates every Unicode `Nd` digit to ASCII
// before parsing**, so `float("١٢٣")` is `123.0` and a colour tuple written in
// Arabic-Indic, Devanagari or fullwidth digits is as valid upstream as its
// ASCII spelling. `strconv.ParseFloat` is ASCII-only, so every row here used to
// exit 1 where upstream renders at exit 0. Each `want` was measured against the
// vendored Python: `colors.body: [١٢٣, 2, 3]` renders `colors-body: rgb(123, 2,
// 3)` into the `.typ`, byte for byte what `[123, 2, 3]` renders.
func TestParseColorTupleAcceptsNonASCIIDigits(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		want     string
	}{
		{name: "an Arabic-Indic channel", elements: []string{"١٢٣", "2", "3"}, want: "rgb(123, 2, 3)"},
		{name: "a Devanagari channel", elements: []string{"१२३", "2", "3"}, want: "rgb(123, 2, 3)"},
		{name: "a fullwidth channel", elements: []string{"１２３", "2", "3"}, want: "rgb(123, 2, 3)"},
		{name: "an Eastern-Arabic channel", elements: []string{"۱۲۳", "2", "3"}, want: "rgb(123, 2, 3)"},
		// `float()` transliterates per character, so a mixed spelling and a
		// fractional one both parse — measured as `float("1٢3") == 123.0`
		// and `float("٣.٥") == 3.5`.
		{name: "a mixed-script channel", elements: []string{"1٢3", "2", "3"}, want: "rgb(123, 2, 3)"},
		{name: "a fractional Arabic-Indic channel", elements: []string{"٣.٥", "2", "3"}, want: "rgb(4, 2, 3)"},
		{name: "an Arabic-Indic exponent", elements: []string{"١٢٣e٠", "2", "3"}, want: "rgb(123, 2, 3)"},
		{name: "an Arabic-Indic alpha", elements: []string{"1", "2", "3", "٠.٥"}, want: "rgba(1, 2, 3, 0.5)"},
		{name: "an Arabic-Indic percent alpha", elements: []string{"1", "2", "3", "٥٠%"}, want: "rgba(1, 2, 3, 0.5)"},
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

// The transliteration must not open the hole the ASCII-only parse was
// incidentally closing: it runs *before* the `0x` guard, exactly as CPython's
// runs before its own parse, so a Go hex-float literal spelled in non-ASCII
// digits is rejected for the same reason `0x1p-2` is. And a non-`Nd` numeral —
// a superscript, a Roman numeral, a Han numeral — is left alone, because
// `float()` leaves it alone too: `float("²")` raises.
func TestParseColorTupleRejectsNonDecimalNumerals(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
	}{
		{name: "a non-ASCII Go hex-float channel", elements: []string{"٠x١p-٢", "0", "0"}},
		{name: "a superscript channel", elements: []string{"²", "0", "0"}},
		{name: "a Han numeral channel", elements: []string{"一二三", "0", "0"}},
		{name: "a Roman numeral channel", elements: []string{"Ⅻ", "0", "0"}},
		// Still range-checked after transliteration: `٣٠٠` is 300.
		{name: "an out-of-range Arabic-Indic channel", elements: []string{"٣٠٠", "0", "0"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := design.ParseColorTuple(test.elements); err == nil {
				t.Errorf("ParseColorTuple(%v) succeeded, want a failure", test.elements)
			}
		})
	}
}

// The colour *string* forms take the same Unicode digits a colour tuple's
// elements do, for a different reason: upstream's `r_rgb`/`r_hsl` patterns are
// `str` patterns compiled without `re.ASCII`, and Python's `\d` there matches
// every Unicode decimal digit. Go's `\d` is ASCII-only, so each row below used
// to exit 1 against upstream's exit 0 — measured end to end,
// `colors.body: "rgb(١٢٣, 2, 3)"` renders where this port refused the file.
// Each `want` is the vendored Python's `Color(input).as_rgb()`, which is what
// `design/color.py`'s `__str__` returns.
func TestParseColorAcceptsNonASCIIDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// `r255`, in all four scripts the tuple path was measured in.
		{name: "an Arabic-Indic channel", input: "rgb(١٢٣, 2, 3)", want: "rgb(123, 2, 3)"},
		{name: "a Devanagari channel", input: "rgb(१२३, 2, 3)", want: "rgb(123, 2, 3)"},
		{name: "a fullwidth channel", input: "rgb(１２３, 2, 3)", want: "rgb(123, 2, 3)"},
		{name: "an Eastern-Arabic channel", input: "rgb(۱۲۳, 2, 3)", want: "rgb(123, 2, 3)"},
		// The class is per character, so a mixed spelling and a fractional one
		// both parse.
		{name: "a mixed-script channel", input: "rgb(1٢3, 2, 3)", want: "rgb(123, 2, 3)"},
		{name: "a fractional channel", input: "rgb(٣.٥, 2, 3)", want: "rgb(4, 2, 3)"},
		// The CSS4 space form is a separate pattern built from the same pieces.
		{name: "a spaced channel", input: "rgb(1 2 ٣)", want: "rgb(1, 2, 3)"},
		{name: "a spaced-form alpha", input: "rgb(1 2 3 / ٠.٥)", want: "rgba(1, 2, 3, 0.5)"},
		// `rAlpha`, both its decimal and its percent branch.
		{name: "a decimal alpha", input: "rgba(1,2,3,٠.٥)", want: "rgba(1, 2, 3, 0.5)"},
		{name: "a percent alpha", input: "rgba(1, 2, 3, ٥٠%)", want: "rgba(1, 2, 3, 0.5)"},
		// An alpha of 1 still becomes absent, whatever script spells it.
		{name: "an alpha of one", input: "rgba(1,2,3,١)", want: "rgb(1, 2, 3)"},
		// `rHue`, each unit — the hue is the one capture not parsed through
		// `parseNumericText`, so it needs its own transliteration.
		{name: "a bare hue", input: "hsl(١٢٠, 50%, 50%)", want: "rgb(64, 191, 64)"},
		{name: "a degree hue", input: "hsl(١٢٠deg, ٥٠%, 50%)", want: "rgb(64, 191, 64)"},
		{name: "a turn hue", input: "hsl(٠.٥turn, 50%, 50%)", want: "rgb(64, 191, 191)"},
		{name: "a radian hue", input: "hsl(٣.١٤rad, 50%, 50%)", want: "rgb(64, 191, 191)"},
		// A negative hue still wraps forward through Python's `%`.
		{name: "a negative hue", input: "hsl(-٩٠, 50%, 50%)", want: "rgb(127, 64, 191)"},
		// `rSL`, both percentages.
		{name: "a saturation and a lightness", input: "hsl(120,٥٠%,٥٠%)", want: "rgb(64, 191, 64)"},
		{name: "an hsla alpha", input: "hsla(120,50%,50%,٠.٢٥)", want: "rgba(64, 191, 64, 0.25)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, err := design.ParseColor(test.input)
			if err != nil {
				t.Fatalf("ParseColor(%q) = %v, want success", test.input, err)
			}
			if got := color.String(); got != test.want {
				t.Errorf("= %q, want %q", got, test.want)
			}
		})
	}
}

// Widening the numeric groups must not widen anything else. Every row is a
// rejection measured on the vendored Python, with the message it raises: the
// hex patterns are a literal `[0-9a-f]` class upstream too and stay ASCII, a
// non-`Nd` numeral is not a `\d` in either language, and the length and range
// limits survive the wider class.
func TestParseColorRejectsNonASCIINonDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "an Arabic-Indic hex short", input: "#١٢٣", want: design.MessageBadColor},
		{name: "an Arabic-Indic hex long", input: "#١٢٣٤٥٦", want: design.MessageBadColor},
		{name: "a non-ASCII Go hex float", input: "٠x١p-٢", want: design.MessageBadColor},
		{name: "a superscript channel", input: "rgb(²,0,0)", want: design.MessageBadColor},
		{name: "a Han numeral channel", input: "rgb(一,0,0)", want: design.MessageBadColor},
		{name: "a four-digit channel", input: "rgb(١٢٣٤,2,3)", want: design.MessageBadColor},
		{name: "an unpercented saturation", input: "hsl(١٢٠,50,50)", want: design.MessageBadColor},
		{name: "a bare numeral", input: "١٢٣", want: design.MessageBadColor},
		{
			name:  "an out-of-range channel",
			input: "rgb(٣٠٠,0,0)",
			want:  "value is not a valid color: color values must be in the range 0 to 255",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := design.ParseColor(test.input)
			if err == nil {
				t.Fatalf("ParseColor(%q) succeeded, want %q", test.input, test.want)
			}
			if err.Error() != test.want {
				t.Errorf("= %q, want %q", err.Error(), test.want)
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
