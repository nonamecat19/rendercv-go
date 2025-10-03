package design

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// colorRGB is a colour's three channels, 0-255.
type colorRGB struct{ R, G, B int }

// CodeColor is `color_error`, the library's own kind
// (pydantic_extra_types/color.py). `design/color.py` subclasses `Color` and adds
// **no** validation — only a `__str__` returning `as_rgb()` — so every colour
// failure here is the library's, not RenderCV's (spec 006 §3.1 behavior 10).
const CodeColor schemaerr.Code = "color_error"

// The five messages the library raises. Only the first is reachable from a
// plain YAML string today; the other four need a numeric form that is
// out of range, which `rgb(300,0,0)` and `rgba(0,0,0,5)` produce.
//
// **The first is dictionary row 13's key**, so what a user finally sees is spec
// 004 §4.11's rewritten text ending `)".` — this package is the first live
// producer for a row that has been in the table since iteration 4
// (spec 006 §3.1 behavior 11).
const (
	MessageBadColor         = "value is not a valid color: string not recognised as a valid color"
	messageChannelNotNumber = "value is not a valid color: color values must be a valid number"
	messageChannelRange     = "value is not a valid color: color values must be in the range 0 to %d"
	messageAlphaNotFloat    = "value is not a valid color: alpha values must be a valid float"
	messageAlphaRange       = "value is not a valid color: alpha values must be in the range 0 to 1"
)

// ErrBadColor is the unrecognised-string failure, which is the only one an
// ordinary design file reaches.
var ErrBadColor = errors.New(MessageBadColor)

// The library's patterns (color.py:45-59), transcribed with Go's syntax for
// non-capturing groups and anchored because upstream uses `re.fullmatch`.
//
// The whole input is **lowercased first** (`parse_str:283`), so every one of
// these is written lowercase and `#FFF`, `RED` and `RGB(1,2,3)` all parse.
var (
	rHexShort = regexp.MustCompile(`^\s*(?:#|0x)?([0-9a-f])([0-9a-f])([0-9a-f])([0-9a-f])?\s*$`)
	rHexLong  = regexp.MustCompile(`^\s*(?:#|0x)?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})?\s*$`)

	r255   = `(\d{1,3}(?:\.\d+)?)`
	rComma = `\s*,\s*`
	rAlpha = `(\d(?:\.\d+)?|\.\d+|\d{1,2}%)`
	rHue   = `(-?\d+(?:\.\d+)?|-?\.\d+)(deg|rad|turn)?`
	rSL    = `(\d{1,3}(?:\.\d+)?)%`

	rRGB       = regexp.MustCompile(`^\s*rgba?\(\s*` + r255 + rComma + r255 + rComma + r255 + `(?:` + rComma + rAlpha + `)?\s*\)\s*$`)
	rRGBSpaced = regexp.MustCompile(`^\s*rgba?\(\s*` + r255 + `\s+` + r255 + `\s+` + r255 + `(?:\s*/\s*` + rAlpha + `)?\s*\)\s*$`)
	rHSL       = regexp.MustCompile(`^\s*hsla?\(\s*` + rHue + rComma + rSL + rComma + rSL + `(?:` + rComma + rAlpha + `)?\s*\)\s*$`)
	rHSLSpaced = regexp.MustCompile(`^\s*hsla?\(\s*` + rHue + `\s+` + rSL + `\s+` + rSL + `(?:\s*/\s*` + rAlpha + `)?\s*\)\s*$`)
)

// Color is a parsed colour. Channels are the library's 0-1 floats rather than
// bytes, because HSL produces fractions and `float_to_255` rounds only at the
// very end — rounding earlier changes `hsl(120,50%,50%)` from `rgb(64, 191, 64)`
// to something else.
type Color struct {
	r, g, b  float64
	alpha    float64
	hasAlpha bool

	// alphaIsInt reproduces a Python distinction that reaches the bytes:
	// `transparent` is built as `RGBA(0, 0, 0, 0)` with an **int** zero, whose
	// `round(_, 2)` is `0` and prints `rgba(0, 0, 0, 0)`, while a hex alpha of
	// zero is `0/255` — a float — and prints `rgba(0, 0, 0, 0.0)`. Measured on
	// `transparent` and `#ff00`.
	alphaIsInt bool
}

// ParseColor is `parse_str` (color.py:260-313), in its order — which is load
// bearing: a bare `fff` is a hex short **because the name table is checked
// first and does not contain it**, and `transparent` is checked *last* even
// though it reads like a name.
func ParseColor(value string) (Color, error) {
	lower := strings.ToLower(value)

	if rgb, ok := colorNames[lower]; ok {
		return channels(float64(rgb.R)/255, float64(rgb.G)/255, float64(rgb.B)/255, nil)
	}

	if match := rHexShort.FindStringSubmatch(lower); match != nil {
		return hexColor(match, 1)
	}
	if match := rHexLong.FindStringSubmatch(lower); match != nil {
		return hexColor(match, 2)
	}

	if match := firstMatch(lower, rRGB, rRGBSpaced); match != nil {
		return rgbColor(match)
	}
	if match := firstMatch(lower, rHSL, rHSLSpaced); match != nil {
		return hslColor(match)
	}

	// Last, after every pattern. `transparent` is not in the name table.
	if lower == "transparent" {
		return Color{r: 0, g: 0, b: 0, alpha: 0, hasAlpha: true, alphaIsInt: true}, nil
	}

	return Color{}, ErrBadColor
}

// ParseColorTuple is `parse_tuple` (color.py:238-249): a 3- or 4-element
// sequence of channel numbers, the fourth being alpha. It is the shape
// `validColorNode`'s length-only check used to wave through unchecked — a
// document could write `[300, 0, 0]` or `[a, b, c]` and this is what
// actually range- and type-checks each element, the same way `ParseColor`
// does for a string. Found by a fresh-context verifier (iteration 14's
// twelfth re-verification).
func ParseColorTuple(elements []string) (Color, error) {
	if len(elements) != 3 && len(elements) != 4 {
		return Color{}, errColorTupleLength
	}
	channelValues := make([]float64, 3)
	for i := 0; i < 3; i++ {
		value, err := parseChannel(elements[i], 255)
		if err != nil {
			return Color{}, err
		}
		channelValues[i] = value
	}
	var alpha *float64
	if len(elements) == 4 {
		value, err := parseAlpha(elements[3])
		if err != nil {
			return Color{}, err
		}
		alpha = value
	}
	return channels(channelValues[0], channelValues[1], channelValues[2], alpha)
}

// String is `design/color.py`'s `__str__`: `as_rgb()`, because the Typst
// templates need `rgb(r, g, b)`. It is also what the JSON Schema shows as a
// colour field's default, which is why `Colors.body` reads `rgb(0, 0, 0)`
// rather than `black`.
func (c Color) String() string {
	if !c.hasAlpha {
		return fmt.Sprintf("rgb(%d, %d, %d)", floatTo255(c.r), floatTo255(c.g), floatTo255(c.b))
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)",
		floatTo255(c.r), floatTo255(c.g), floatTo255(c.b), c.formatAlpha())
}

// ValidColor reports whether a string parses, for callers that only need the
// yes-or-no.
func ValidColor(value string) bool {
	_, err := ParseColor(value)
	return err == nil
}

func firstMatch(value string, patterns ...*regexp.Regexp) []string {
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(value); match != nil {
			return match
		}
	}
	return nil
}

// hexColor handles both hex forms. `width` is 1 for the short form, whose digits
// are **doubled** rather than scaled — `#fff` is `ffffff`, not `f0f0f0`.
func hexColor(match []string, width int) (Color, error) {
	component := func(text string) float64 {
		if width == 1 {
			text += text
		}
		value, _ := strconv.ParseUint(text, 16, 16)
		return float64(value)
	}

	// The alpha goes through `parse_float_alpha` like every other form, which
	// is why `#ffff` is opaque white rather than white with an alpha of 1:
	// `math.isclose(alpha, 1)` drops it.
	var alpha *float64
	if match[4] != "" {
		value, err := normalizeAlpha(component(match[4]) / 255)
		if err != nil {
			return Color{}, err
		}
		alpha = value
	}
	return channels(component(match[1])/255, component(match[2])/255, component(match[3])/255, alpha)
}

func rgbColor(match []string) (Color, error) {
	values := [3]float64{}
	for i := range values {
		value, err := parseChannel(match[i+1], 255)
		if err != nil {
			return Color{}, err
		}
		values[i] = value
	}
	alpha, err := parseAlpha(match[4])
	if err != nil {
		return Color{}, err
	}
	return channels(values[0], values[1], values[2], alpha)
}

// hslColor is `parse_hsl` (color.py:408-433). The hue unit decides the modulus —
// degrees, radians or turns — and saturation and lightness are percentages, so
// they are parsed against a maximum of 100 rather than 255.
func hslColor(match []string) (Color, error) {
	saturation, err := parseChannel(match[3], 100)
	if err != nil {
		return Color{}, err
	}
	lightness, err := parseChannel(match[4], 100)
	if err != nil {
		return Color{}, err
	}

	hue, convErr := strconv.ParseFloat(match[1], 64)
	if convErr != nil {
		return Color{}, errors.New(messageChannelNotNumber)
	}
	switch match[2] {
	case "rad":
		hue = pythonMod(hue, 2*math.Pi) / (2 * math.Pi)
	case "turn":
		hue = pythonMod(hue, 1)
	default: // "" and "deg"
		hue = pythonMod(hue, 360) / 360
	}

	alpha, err := parseAlpha(match[5])
	if err != nil {
		return Color{}, err
	}
	r, g, b := hlsToRGB(hue, lightness, saturation)
	return channels(r, g, b, alpha)
}

func channels(r, g, b float64, alpha *float64) (Color, error) {
	color := Color{r: r, g: g, b: b}
	if alpha != nil {
		color.alpha = *alpha
		color.hasAlpha = true
	}
	return color, nil
}

// parseChannel is `parse_color_value`: a number scaled into 0-1, rejected
// outside the range.
func parseChannel(text string, max float64) (float64, error) {
	value, ok := parseNumericText(text)
	if !ok {
		return 0, errors.New(messageChannelNotNumber)
	}
	if value < 0 || value > max {
		return 0, fmt.Errorf(messageChannelRange, int(max))
	}
	return value / max, nil
}

// parseNumericText is `float(value)` for the token shapes a colour tuple's
// element can actually carry once it comes from a YAML document rather than
// a regex capture: a bare decimal (`strconv.ParseFloat` already handles
// this, digit separators included), a YAML 1.1 bool word (`float(True) ==
// 1.0` in Python), or a hex/octal/binary integer literal (`0x10`, `0o17`,
// `0b101` all resolve to an `int` upstream, and `float(int)` never fails).
// `strconv.ParseFloat` alone rejects all three, which is what let
// `colors.name: [true, 0, 0]` and `colors.name: [0x10, 0, 0]` — both valid
// upstream — fail here. Found by a fresh-context verifier (iteration 14's
// thirteenth re-verification).
func parseNumericText(text string) (float64, bool) {
	switch strings.ToLower(text) {
	case "true":
		return 1, true
	case "false":
		return 0, true
	}
	lower := strings.ToLower(strings.TrimLeft(text, "+-"))
	if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0o") || strings.HasPrefix(lower, "0b") {
		value, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return 0, false
		}
		return float64(value), true
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseAlpha is `parse_float_alpha`. **An alpha of exactly 1 becomes absent**,
// which is why `rgba(1,2,3,1)` renders as `rgb(1, 2, 3)` — the `a` in the input
// does not survive into the output.
func parseAlpha(text string) (*float64, error) {
	if text == "" {
		return nil, nil
	}

	var value float64
	var err error
	if percent, found := strings.CutSuffix(text, "%"); found {
		value, err = strconv.ParseFloat(percent, 64)
		value /= 100
	} else {
		value, err = strconv.ParseFloat(text, 64)
	}
	if err != nil {
		return nil, errors.New(messageAlphaNotFloat)
	}

	return normalizeAlpha(value)
}

// normalizeAlpha is the tail of `parse_float_alpha` (color.py:394-407), shared
// because the hex forms reach it with a float rather than a string.
func normalizeAlpha(value float64) (*float64, error) {
	if math.Abs(value-1) <= 1e-9 {
		return nil, nil
	}
	if value < 0 || value > 1 {
		return nil, errors.New(messageAlphaRange)
	}
	return &value, nil
}

// hlsToRGB is Python's `colorsys.hls_to_rgb`, transcribed. Go has no equivalent
// in the standard library, and a conversion written from the usual HSL formula
// differs in the sixth decimal — which `float_to_255` can turn into an
// off-by-one channel.
func hlsToRGB(h, l, s float64) (float64, float64, float64) {
	if s == 0 {
		return l, l, l
	}
	var m2 float64
	if l <= 0.5 {
		m2 = l * (1 + s)
	} else {
		m2 = l + s - l*s
	}
	m1 := 2*l - m2
	return hueToRGB(m1, m2, h+1.0/3.0), hueToRGB(m1, m2, h), hueToRGB(m1, m2, h-1.0/3.0)
}

func hueToRGB(m1, m2, hue float64) float64 {
	hue = pythonMod(hue, 1)
	switch {
	case hue < 1.0/6.0:
		return m1 + (m2-m1)*hue*6
	case hue < 0.5:
		return m2
	case hue < 2.0/3.0:
		return m1 + (m2-m1)*(2.0/3.0-hue)*6
	}
	return m1
}

// pythonMod is `%` with Python's sign rule: the result takes the divisor's sign,
// so `-90 % 360` is `270` and not `-90`. Go's `math.Mod` keeps the dividend's,
// which would turn a negative hue into a negative fraction and then into a
// wrong colour rather than an error.
func pythonMod(value, modulus float64) float64 {
	result := math.Mod(value, modulus)
	if result != 0 && (result < 0) != (modulus < 0) {
		result += modulus
	}
	return result
}

// floatTo255 is `float_to_255` (color.py:436-445): `round(c * 255)`, and
// **Python's round is half-to-even**. `math.Round` is half-away-from-zero, so a
// channel landing exactly on `.5` would differ by one.
func floatTo255(c float64) int {
	return int(math.RoundToEven(c * 255))
}

// formatAlpha is Python's `round(x, 2)` followed by `str`, which prints the
// shortest form: `0.5` and not `0.50`.
//
// Zero is the one place Go and Python disagree: `str(0.0)` is `0.0` and Go's
// shortest form is `0`. See Color.alphaIsInt for why the int-zero case wants
// Go's answer and the float-zero case does not.
func (c Color) formatAlpha() string {
	rounded := math.RoundToEven(c.alpha*100) / 100
	if rounded == 0 {
		if c.alphaIsInt {
			return "0"
		}
		return "0.0"
	}
	return strconv.FormatFloat(rounded, 'g', -1, 64)
}
