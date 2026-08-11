package design

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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
//
// **This treats every element as eligible for the bool-word/hex/octal/binary
// coercion**, which is only correct once the document has already validated
// — `ColorElements` is what `validColorNode` actually calls, because that
// coercion is only sound for an element YAML itself resolved to a bool or an
// int, not a merely bool-or-hex-*looking* quoted string. This variant exists
// for `normalizeColors`, whose `[]string` input has already lost that
// distinction by the time it gets there (`bridge.mappingOf`'s projection),
// on a value that either already passed validation or is inside the
// documented "value validation skipped when a script exists" gap either way.
func ParseColorTuple(elements []string) (Color, error) {
	typed := make([]colorElement, len(elements))
	for i, raw := range elements {
		typed[i] = colorElement{Raw: raw, Coerce: true, IsPythonInt: looksLikePlainInt(raw)}
	}
	return parseColorElements(typed)
}

// looksLikePlainInt is `validColorNode`'s `elem.Kind == yamldoc.KindInt`,
// reconstructed from text alone — this function's caller has already lost
// the original `yamldoc.Kind` by the time a document's sequence reaches
// here (`bridge.mappingOf`'s projection). A plain decimal integer is far
// the common case for an unquoted YAML int; a hex/octal/binary literal
// never produces a signed zero in the first place (`strconv.ParseInt`'s
// `int64` has none), so only the plain-decimal shape needs the check.
func looksLikePlainInt(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	unsigned := strings.TrimLeft(trimmed, "+-")
	if unsigned == "" {
		return false
	}
	for _, r := range unsigned {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// colorElement is one colour-tuple element together with whether the
// bool-word/hex/octal/binary coercion is legal for it — true only when YAML
// itself resolved the node to a bool or an int, matching `float(value)`
// being called on that parsed Python object rather than on a string that
// happens to spell the same token.
type colorElement struct {
	Raw    string
	Coerce bool

	// IsPythonInt marks an element YAML itself resolved to a plain integer
	// (`KindInt`, no decimal point). **A Python `int` has no negative
	// zero** — `-0 == 0` and `float(0)` is always `+0.0` — where a `float`
	// or a quoted `str` does (`float(-0.0)` and `float("-0")` are both
	// `-0.0`). `strconv.ParseFloat("-0", 64)` produces Go's signed zero
	// regardless of the source, which the fix for a *float*-sourced
	// `-0.0` alpha (below) would otherwise print as `-0.0` for an
	// int-sourced `-0` too — a value Python never produces. Found by a
	// fresh-context verifier (iteration 14's twentieth re-verification).
	IsPythonInt bool

	// NonScalar marks an element YAML resolved to a sequence or a mapping,
	// which `Raw` cannot carry — `yamldoc.Node.Raw` is scalar-only, so such
	// an element arrives as the empty string and is indistinguishable from
	// "nothing written here". In the alpha position that is fatal:
	// `parseAlpha("")` is `parse_float_alpha(None)`, "no alpha at all", so
	// `[1, 2, 3, [1]]` used to validate as the three-channel `rgb(1, 2, 3)`
	// and render, at exit 0, a colour the document never asked for. Upstream
	// has no such reading — `float(CommentedSeq)` raises `TypeError`, which
	// `parse_float_alpha` does not catch. Found by a fresh-context verifier
	// (iteration 15's colour-tuple sweep).
	NonScalar bool
}

// parseColorElements is `ParseColorTuple` with each element's coercion
// eligibility carried alongside its text — what `validColorNode` actually
// calls, since it still has each element's `yamldoc.Kind`. Found by a
// fresh-context verifier (iteration 14's sixteenth re-verification).
func parseColorElements(elements []colorElement) (Color, error) {
	if len(elements) != 3 && len(elements) != 4 {
		return Color{}, errColorTupleLength
	}
	channelValues := make([]float64, 3)
	for i := 0; i < 3; i++ {
		// **`parse_color_value` catches `TypeError` as well as
		// `ValueError`** (color.py:355-361), so a sequence or a mapping
		// channel is this message and not a crash. An empty `Raw` already
		// produced it, which is why only the alpha position ever rendered
		// wrong — stated outright here so the two positions stop agreeing
		// by coincidence.
		if elements[i].NonScalar {
			return Color{}, errors.New(messageChannelNotNumber)
		}
		value, err := parseChannel(elements[i].Raw, 255, elements[i].Coerce)
		if err != nil {
			return Color{}, err
		}
		channelValues[i] = value
	}
	var alpha *float64
	if len(elements) == 4 {
		// **`parse_float_alpha` catches only `ValueError`**
		// (color.py:391), so upstream dies on a non-scalar alpha with an
		// unhandled `TypeError` — a traceback this port declines to
		// reproduce (D-011's territory). It reports the message the same
		// function raises for the failure it *does* catch, which the
		// dictionary rewrites to the text every colour failure shares and
		// which exits 1 exactly as upstream's traceback does.
		if elements[3].NonScalar {
			return Color{}, errors.New(messageAlphaNotFloat)
		}
		value, err := parseAlpha(elements[3].Raw, elements[3].Coerce)
		if err != nil {
			return Color{}, err
		}
		if value != nil && *value == 0 && elements[3].IsPythonInt {
			positive := math.Copysign(0, 1)
			value = &positive
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
		value, err := parseChannel(match[i+1], 255, false)
		if err != nil {
			return Color{}, err
		}
		values[i] = value
	}
	alpha, err := parseAlpha(match[4], false)
	if err != nil {
		return Color{}, err
	}
	return channels(values[0], values[1], values[2], alpha)
}

// hslColor is `parse_hsl` (color.py:408-433). The hue unit decides the modulus —
// degrees, radians or turns — and saturation and lightness are percentages, so
// they are parsed against a maximum of 100 rather than 255.
func hslColor(match []string) (Color, error) {
	saturation, err := parseChannel(match[3], 100, false)
	if err != nil {
		return Color{}, err
	}
	lightness, err := parseChannel(match[4], 100, false)
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

	alpha, err := parseAlpha(match[5], false)
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
func parseChannel(text string, max float64, coerce bool) (float64, error) {
	value, ok := parseNumericText(text, coerce)
	if !ok {
		return 0, errors.New(messageChannelNotNumber)
	}
	// **Python's chained comparison `0 <= color <= max_val` is false for
	// NaN in both directions**, so the `else` (range error) branch runs.
	// Go's `value < 0 || value > max` is *also* false in both directions
	// for NaN — but that is the accept condition here, inverted from
	// Python's, so NaN fell through as "in range" and `int(NaN)` produced
	// undefined-overflow garbage in the artifact. Written as the same
	// chained shape closes the gap for every NaN-producing input, not just
	// a literal `nan` token. Found by a fresh-context verifier (iteration
	// 14's fifteenth re-verification).
	if !(value >= 0 && value <= max) {
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
//
// **`coerce` gates the bool-word and hex/octal/binary branches**, and it
// must be false for a genuinely `KindString` element. `parse_color_value`
// is `float(value)` on whatever Python object YAML already resolved — a
// bool-word or hex-literal *token* only becomes that coercible object when
// YAML itself parsed it as a bool or an int; a document that quoted the
// same spelling (`colors.name: ["0x10", 0, 0]`) hands `float()` a `str`,
// and `float("0x10")` raises just as it does for any other non-numeric
// string. Passing every element through unconditionally coerced a quoted
// `"0x10"`/`"true"` the same as an unquoted one, accepting documents
// upstream rejects. Found by a fresh-context verifier (iteration 14's
// sixteenth re-verification).
//
// **A digit is any Unicode `Nd` character, not just an ASCII one** —
// `float("١٢٣")` is `123.0`, where `strconv.ParseFloat` is ASCII-only. See
// `asciiDecimalDigits`.
func parseNumericText(text string, coerce bool) (float64, bool) {
	// `float(" 1 ")` succeeds in Python — surrounding whitespace is stripped
	// before anything else, coerced or not (a plain decimal string strips
	// the same way). Found by a fresh-context verifier (iteration 14's
	// fourteenth re-verification).
	text = strings.TrimSpace(text)
	// **Python parses a `str` float on the ASCII *transliteration* of its
	// digits, not on the digits as written** — see `asciiDecimalDigits`.
	//
	// A token the transform *changes* contained a character outside ASCII,
	// and so is a token ruamel's resolvers — ASCII regexes, every one —
	// could never have turned into a bool or an int. It is a Python `str`
	// however the caller found it, which is why a changed token takes the
	// string branch below even when `coerce` is set: `٠x١p-٢` has to be
	// rejected by the same `0x` guard `0x1p-2` is, not slip past a guard
	// that only knows ASCII and then be accepted by Go's hex-float parser.
	ascii := asciiDecimalDigits(text)
	if !coerce || ascii != text {
		text = ascii
		// **Go's hex-float literal syntax (`0x1p-2`) is not a Python
		// `float()` literal.** `strconv.ParseFloat` accepts Go's numeric
		// grammar, which is a superset of Python's for strings — Python's
		// `float("0x1p-2")` raises `ValueError`. A quoted colour-tuple
		// element spelling one used to validate and then leak a raw Go
		// slice into the artifact, because `ParseColorTuple` (the
		// merge-time, always-coercing caller) routes anything starting
		// `0x` to `ParseInt` instead, which rejects it — the two paths
		// disagreed on a shape `validColorNode` had just accepted. Found
		// by a fresh-context verifier (iteration 14's colour-slice sweep).
		lower := strings.ToLower(strings.TrimLeft(text, "+-"))
		if strings.HasPrefix(lower, "0x") {
			return 0, false
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	}
	switch strings.ToLower(text) {
	case "true":
		return 1, true
	case "false":
		return 0, true
	}
	// **The prefix has to match YAML 1.1's exact case, not either case.**
	// `0X1F`/`0O17` are not integers to ruamel — ruamel's own resolver is
	// lowercase-only for these — so upstream sees the plain string
	// `"0X1F"`, and `float("0X1F")` raises. `strings.ToLower` here would
	// route an uppercase-prefixed token into `ParseInt(text, 0, 64)`, which
	// (unlike ruamel) accepts either case, accepting a token upstream
	// rejects. Found by a fresh-context verifier (iteration 14's
	// fifteenth re-verification).
	unsigned := strings.TrimLeft(text, "+-")
	if strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0o") || strings.HasPrefix(unsigned, "0b") {
		value, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return 0, false
		}
		return float64(value), true
	}
	// **A NaN or an infinity still counts as "a valid number" here** —
	// `float("nan")` succeeds in Python too, string or not; what upstream
	// actually rejects it on is the *range* check next, which is why this
	// function returns it rather than failing outright, and why
	// `parseChannel`/`normalizeAlpha` are the ones written as a chained
	// comparison rather than this one refusing the token.
	//
	// `text` is pure ASCII by here — anything else took the string branch
	// above — so this call needs no transliteration of its own.
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// asciiDecimalDigits is the digit half of CPython's
// `_PyUnicode_TransformDecimalAndSpaceToASCII`, which `float(str)` runs over
// its argument before parsing it (`Objects/floatobject.c`'s
// `PyFloat_FromString`). Every character in Unicode's `Nd` category is
// replaced by the ASCII digit of the same value, so `float("١٢٣")` is
// `123.0` — Arabic-Indic, Devanagari and fullwidth digits all parse, and all
// parse to exactly the value their ASCII spelling would give (measured
// against upstream: `colors.body: [١٢٣, 2, 3]`, `[१२३, 2, 3]` and
// `[１２３, 2, 3]` each render `rgb(123, 2, 3)` at exit 0).
// `strconv.ParseFloat` is ASCII-only by documented contract, so without this
// pass those three documents exited 1 here — the port rejecting what upstream
// accepts, the wrong direction of divergence. Found by a fresh-context
// verifier (iteration 14's colour-slice sweep, deferred there and recorded in
// specs/STATE.md).
//
// The space half of the same CPython transform is already covered:
// `parseNumericText` calls `strings.TrimSpace`, whose `unicode.IsSpace` is
// the same set of characters.
func asciiDecimalDigits(text string) string {
	if isASCIIOnly(text) {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for _, r := range text {
		if digit, ok := asciiDigitOf(r); ok {
			out.WriteRune(digit)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// isASCIIOnly is the fast path out of `asciiDecimalDigits`: nothing outside
// ASCII means nothing to transliterate, which is every colour a document
// realistically writes.
func isASCIIOnly(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// asciiDigitOf maps an `Nd` rune to the ASCII digit of the same numeric
// value. The standard library has no "decimal value of this rune" call —
// `unicode.IsDigit` answers only the category question — so the value comes
// from the structure of `unicode.Nd` itself: every one of its ranges has
// stride 1 and lists whole ten-digit blocks starting at that script's zero,
// making the value `(r - Lo) % 10`. Verified exhaustively against Python's
// `unicodedata.decimal` over all 680 runes in Go's table: zero mismatches.
// A range with any other stride would break that arithmetic, so one is
// refused rather than guessed at.
func asciiDigitOf(r rune) (rune, bool) {
	if !unicode.Is(unicode.Nd, r) {
		return 0, false
	}
	for _, rg := range unicode.Nd.R16 {
		if r >= rune(rg.Lo) && r <= rune(rg.Hi) && rg.Stride == 1 {
			return '0' + (r-rune(rg.Lo))%10, true
		}
	}
	for _, rg := range unicode.Nd.R32 {
		if r >= rune(rg.Lo) && r <= rune(rg.Hi) && rg.Stride == 1 {
			return '0' + (r-rune(rg.Lo))%10, true
		}
	}
	return 0, false
}

// parseAlpha is `parse_float_alpha`. **An alpha of exactly 1 becomes absent**,
// which is why `rgba(1,2,3,1)` renders as `rgb(1, 2, 3)` — the `a` in the input
// does not survive into the output.
func parseAlpha(text string, coerce bool) (*float64, error) {
	if text == "" {
		return nil, nil
	}

	var value float64
	if percent, found := strings.CutSuffix(text, "%"); found {
		// **`endswith('%')` is checked before anything else, including
		// whitespace** — `parse_float_alpha` strips the `%` and then does
		// `float(value[:-1])`, which is `parseNumericText`'s job, not a
		// second, unrelated `ParseFloat` call. Pass 14 fixed this branch's
		// sibling below and missed this one. Found by a fresh-context
		// verifier (iteration 14's fifteenth re-verification).
		//
		// **A `%`-suffixed value is always string-sourced** — only a `str`
		// can end in a literal `%` character, so `color.py:387` guards this
		// whole branch with `isinstance(value, str)`. `coerce` is therefore
		// always false here regardless of the caller's own element, the
		// same way it always is for a `KindString` element elsewhere.
		// Found by a fresh-context verifier (iteration 14's sixteenth
		// re-verification).
		parsed, ok := parseNumericText(percent, false)
		if !ok {
			return nil, errors.New(messageAlphaNotFloat)
		}
		value = parsed / 100
	} else {
		// **`parse_float_alpha` is also `float(value)`** — a bool or a
		// hex/octal/binary literal alpha is upstream's `float(True)`/
		// `float(0x0)`, the same coercion `parseChannel` already got.
		// `strconv.ParseFloat` alone rejected both. Found by a
		// fresh-context verifier (iteration 14's fourteenth
		// re-verification).
		parsed, ok := parseNumericText(text, coerce)
		if !ok {
			return nil, errors.New(messageAlphaNotFloat)
		}
		value = parsed
	}

	return normalizeAlpha(value)
}

// normalizeAlpha is the tail of `parse_float_alpha` (color.py:394-407), shared
// because the hex forms reach it with a float rather than a string.
func normalizeAlpha(value float64) (*float64, error) {
	if math.Abs(value-1) <= 1e-9 {
		return nil, nil
	}
	// **The same chained-comparison shape `parseChannel` needed.**
	// `value < 0 || value > 1` is false in both directions for a NaN
	// alpha (`math.IsClose`'s `abs(nan-1)` also fails the branch above), so
	// the un-chained form let it through as "in range" and printed NaN into
	// the artifact. Found by a fresh-context verifier (iteration 14's
	// fifteenth re-verification).
	if !(value >= 0 && value <= 1) {
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
// **The rounding itself must not go through `c.alpha*100`.** Python's
// `round(x, 2)` rounds the float's *exact* binary value to two decimal
// places — `0.075` is not exactly `0.075` in binary, its true value is
// fractionally below it, so `round(0.075, 2)` is `0.07`. Multiplying by
// 100 first (`math.RoundToEven(c.alpha*100)`) computes on a *different*,
// re-rounded number — `0.075*100` lands on exactly `7.5` in float64 even
// though the true value is fractionally below `7.5` — and rounds that
// instead, giving `0.08`. `strconv.FormatFloat(v, 'f', 2, 64)` performs
// the correctly-rounded decimal conversion Go already has, on the exact
// value, with no such intermediate. Found by a fresh-context verifier
// (iteration 14's colour-slice sweep).
//
// Zero is the one place Go and Python disagree: `str(0.0)` is `0.0` and Go's
// shortest form is `0`. See Color.alphaIsInt for why the int-zero case wants
// Go's answer and the float-zero case does not.
//
// **An alpha that rounds up to 1 is still a Python `float`, never an
// `int`.** `normalizeAlpha` only drops an alpha that is already
// `math.isclose` to 1 before rounding (spec: "an alpha of exactly 1 becomes
// absent") — an alpha that merely *rounds* to 1, like `0.996`, still
// reaches here and `round(0.996, 2)` is the float `1.0`, printing
// `rgba(…, 1.0)`, not `rgba(…, 1)`. Found by a fresh-context verifier
// (iteration 14's colour-slice sweep).
func (c Color) formatAlpha() string {
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(c.alpha, 'f', 2, 64), 64)
	if err != nil {
		rounded = c.alpha
	}
	if rounded == 0 {
		if c.alphaIsInt {
			return "0"
		}
		// **`-0.0 == 0` is true in Go, and equality alone throws the sign
		// away.** Python's `str(round(-0.0, 2))` is `'-0.0'`, not `'0.0'`
		// — the same distinction this function already draws between an
		// `int` zero and a `float` zero, one bit further down. Found by a
		// fresh-context verifier (iteration 14's nineteenth
		// re-verification).
		if math.Signbit(rounded) {
			return "-0.0"
		}
		return "0.0"
	}
	if rounded == 1 {
		return "1.0"
	}
	return strconv.FormatFloat(rounded, 'g', -1, 64)
}
