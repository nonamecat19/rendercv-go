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
