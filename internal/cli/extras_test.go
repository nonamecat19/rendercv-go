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
