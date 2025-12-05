package cli

import (
	"slices"
	"testing"
)

// TestNormalizeSeparatesExtras is the other half: the pre-pass has to know
// which tokens `render` declares before it can say which ones are extras.
//
// A declared option that takes a value consumes the next token, so
// `-typ out.typ` is two tokens of flag rather than a key and a value.
func TestNormalizeSeparatesExtras(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			name:   "declared value option is not an extra",
			args:   []string{"render", "cv.yaml", "-typ", "out.typ"},
			rest:   []string{"render", "cv.yaml", "--typst-path", "out.typ"},
			extras: nil,
		},
		{
			name:   "declared bool option is not an extra",
			args:   []string{"render", "cv.yaml", "-nopdf"},
			rest:   []string{"render", "cv.yaml", "--dont-generate-pdf"},
			extras: nil,
		},
		{
			name:   "dotted override is an extra",
			args:   []string{"render", "cv.yaml", "--cv.phone", "123"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.phone", "123"},
		},
		{
			name:   "undotted unknown flag is an extra",
			args:   []string{"render", "cv.yaml", "--nope", "value"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--nope", "value"},
		},
		{
			// The first non-flag token is the input file; anything after it is
			// an extra, which is why `render a.yaml b.yaml` is an odd count and
			// not two input files.
			name:   "second positional is an extra",
			args:   []string{"render", "cv.yaml", "extra.yaml"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"extra.yaml"},
		},
		{
			// click accepts `--long=value`, and the whole token is one
			// argument. It must not be split into a key and a value.
			name:   "equals form of a declared option",
			args:   []string{"render", "cv.yaml", "--output-folder=out"},
			rest:   []string{"render", "cv.yaml", "--output-folder=out"},
			extras: nil,
		},
		{
			name:   "unknown single dash word is an extra",
			args:   []string{"render", "cv.yaml", "-x", "value"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"-x", "value"},
		},
		{
			// Extras keep their order, because the pairing is positional.
			name:   "extras keep their order",
			args:   []string{"render", "cv.yaml", "--b", "2", "--a", "1"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--b", "2", "--a", "1"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, extras := Normalize(row.args)
			if !slices.Equal(rest, row.rest) {
				t.Errorf("rest = %q, want %q", rest, row.rest)
			}
			if !slices.Equal(extras, row.extras) {
				t.Errorf("extras = %q, want %q", extras, row.extras)
			}
		})
	}
}

// TestEndOfOptions is spec 012 gaps.md G-1.
//
// **A bare `--` ends option parsing and is dropped.** Click removes it from the
// vector and every following token becomes an extra — declared flags included —
// so `-- -notyp` is the override key `-notyp` and not the flag. The port used
// to collect `--` itself as an extra and keep parsing the rest as flags, which
// made `render cv.yaml --` an error where upstream renders.
func TestEndOfOptions(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			name:   "a trailing double dash is dropped",
			args:   []string{"render", "cv.yaml", "--"},
			rest:   []string{"render", "cv.yaml"},
			extras: nil,
		},
		{
			// Measured against the vendored CLI, which reports
			// `extra arguments (-notyp,-nomd,-nopdf,-nopng,-q)`.
			name:   "declared flags after it are extras",
			args:   []string{"render", "cv.yaml", "--", "-notyp", "-nomd", "-q"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"-notyp", "-nomd", "-q"},
		},
		{
			name:   "flags before it still parse",
			args:   []string{"render", "cv.yaml", "-nopdf", "--", "-notyp"},
			rest:   []string{"render", "cv.yaml", "--dont-generate-pdf"},
			extras: []string{"-notyp"},
		},
		{
			name:   "an override after it keeps its pairing",
			args:   []string{"render", "cv.yaml", "--", "--cv.name", "Jane"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.name", "Jane"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, extras := Normalize(row.args)
			if !slices.Equal(rest, row.rest) {
				t.Errorf("rest = %q, want %q", rest, row.rest)
			}
			if !slices.Equal(extras, row.extras) {
				t.Errorf("extras = %q, want %q", extras, row.extras)
			}
		})
	}
}

// TestUnrecognizedOptionDoesNotSwallowTheNextToken is the discriminating vector
// for the first of `Normalize`'s three override rules, which nothing gated.
//
// **`--nope value` cannot tell the two behaviors apart.** Whether the unknown
// option swallows the following token or leaves it to the loop, the extras come
// out `--nope value` either way — which is exactly the case
// `TestNormalizeSeparatesExtras` has, so making `case long:` consume `args[i+1]`
// unconditionally left the whole suite green.
//
// `--nope -nopdf` separates them. Click appends the unknown option to
// `ctx.args` and **goes on parsing**, so `-nopdf` is still matched as
// `--dont-generate-pdf`; a swallowing port would eat it as `--nope`'s value and
// produce an even, silently-accepted pair where upstream reports `(--nope)`.
func TestUnrecognizedOptionDoesNotSwallowTheNextToken(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			// The vector: the swallowed token is a *declared* flag, so eating
			// it changes both halves of the split at once.
			name:   "a declared short flag after an unknown option still parses",
			args:   []string{"render", "cv.yaml", "--nope", "-nopdf"},
			rest:   []string{"render", "cv.yaml", "--dont-generate-pdf"},
			extras: []string{"--nope"},
		},
		{
			name:   "and a declared long flag likewise",
			args:   []string{"render", "cv.yaml", "--nope", "--quiet"},
			rest:   []string{"render", "cv.yaml", "--quiet"},
			extras: []string{"--nope"},
		},
		{
			// A value option after an unknown one keeps its own value, which a
			// swallowing port turns into a stray positional.
			name:   "a declared value option after an unknown option keeps its value",
			args:   []string{"render", "cv.yaml", "--nope", "-o", "out"},
			rest:   []string{"render", "cv.yaml", "--output-folder", "out"},
			extras: []string{"--nope"},
		},
		{
			// Two unknowns in a row are two extras — an even count, and
			// therefore a key/value pair upstream accepts. A swallowing port
			// produces one extra and an odd count.
			name:   "two unknown options are two extras",
			args:   []string{"render", "cv.yaml", "--nope", "--alsonope"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--nope", "--alsonope"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, extras := Normalize(row.args)
			if !slices.Equal(rest, row.rest) {
				t.Errorf("rest = %q, want %q", rest, row.rest)
			}
			if !slices.Equal(extras, row.extras) {
				t.Errorf("extras = %q, want %q", extras, row.extras)
			}
		})
	}
}

// TestUnknownEqualsFormStaysOneToken is the second ungated override rule.
//
// **`--cv.name=Jane` is one token, and that is what makes it an error.**
// `parse_override_arguments` pairs `ctx.args` two at a time and rejects an odd
// count (`parse_override_arguments.py:35-42`), so the single token upstream
// appends is `There is a problem with the extra arguments (--cv.name=Jane)!`,
// not the override `cv.name: Jane`.
//
// The existing `=` case pins `--output-folder=out`, a **declared** option, which
// takes the `renderValueFlags` branch — so splitting an *unknown* long option on
// `=` left the suite green. Every case here is an undeclared key, the only shape
// that reaches `case long:`.
func TestUnknownEqualsFormStaysOneToken(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		extras []string
	}{
		{
			name:   "a dotted override with an equals sign",
			args:   []string{"render", "cv.yaml", "--cv.name=Jane"},
			extras: []string{"--cv.name=Jane"},
		},
		{
			// The value's own `=` is not a separator either: click splits on
			// the first one only, and the port splits on none.
			name:   "an equals sign inside the value",
			args:   []string{"render", "cv.yaml", "--cv.email=a=b"},
			extras: []string{"--cv.email=a=b"},
		},
		{
			// The even-count shape: two `=` tokens are a key and a value to
			// `parse_override_arguments`, however odd that reads.
			name:   "two equals tokens stay two",
			args:   []string{"render", "cv.yaml", "--cv.name=Jane", "--cv.phone=123"},
			extras: []string{"--cv.name=Jane", "--cv.phone=123"},
		},
		{
			// The space form is still two tokens; the rule is about not
			// *creating* a second one, not about rejecting the pair.
			name:   "the space form is unaffected",
			args:   []string{"render", "cv.yaml", "--cv.name", "Jane"},
			extras: []string{"--cv.name", "Jane"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, extras := Normalize(row.args)
			if want := []string{"render", "cv.yaml"}; !slices.Equal(rest, want) {
				t.Errorf("rest = %q, want %q", rest, want)
			}
			if !slices.Equal(extras, row.extras) {
				t.Errorf("extras = %q, want %q", extras, row.extras)
			}
		})
	}
}

// TestYamlLocationIsADeclaredFlag is G-2: upstream declares --YAMLLOCATION and
// binds it to `_`, so it must parse and vanish rather than becoming an override
// key the model rejects.
func TestYamlLocationIsADeclaredFlag(t *testing.T) {
	rest, extras := Normalize([]string{"render", "cv.yaml", "--YAMLLOCATION", "zzz"})
	if want := []string{"render", "cv.yaml", "--YAMLLOCATION", "zzz"}; !slices.Equal(rest, want) {
		t.Errorf("rest = %q, want %q", rest, want)
	}
	if len(extras) != 0 {
		t.Errorf("extras = %q, want none — it is a declared option", extras)
	}
}
