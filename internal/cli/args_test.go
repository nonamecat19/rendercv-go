package cli

import (
	"reflect"
	"slices"
	"testing"
)

// Spec 012 §2 behaviors 6, 7 and 9, driven by the argument vectors the corpus
// cases actually use.
func TestNormalize(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			// `render_typst_only`: four negative flags, all single-dash. **The
			// rewrite is short-to-long, not dash-to-dashes** — upstream's own
			// name for `-nopdf` is `--dont-generate-pdf`, and the port used to
			// declare a `--nopdf` that upstream has never had.
			name:   "single dash negatives",
			args:   []string{"render", "cv.yaml", "-nopdf", "-nopng", "-nomd", "-nohtml"},
			rest:   []string{"render", "cv.yaml", "--dont-generate-pdf", "--dont-generate-png", "--dont-generate-markdown", "--dont-generate-html"},
			extras: nil,
		},
		{
			// `render_custom_paths`: single-dash flags that take a value.
			name:   "single dash paths",
			args:   []string{"render", "cv.yaml", "-typ", "out/custom.typ", "-md", "out/n/c.md"},
			rest:   []string{"render", "cv.yaml", "--typst-path", "out/custom.typ", "--markdown-path", "out/n/c.md"},
			extras: nil,
		},
		{
			// **The rewrite is scoped to `render`.** `new` declares none of
			// these, so a token that looks like one must reach the parser
			// unchanged rather than becoming a flag the subcommand lacks.
			name:   "short forms are not rewritten outside render",
			args:   []string{"new", "John Doe", "-typ"},
			rest:   []string{"new", "John Doe", "-typ"},
			extras: nil,
		},
		{
			// `render_override_scalar`.
			name:   "scalar override",
			args:   []string{"render", "cv.yaml", "--cv.phone", "+1-555-555-5555"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.phone", "+1-555-555-5555"},
		},
		{
			// `render_override_indexed` — the index is part of the path.
			name:   "indexed override",
			args:   []string{"render", "cv.yaml", "--cv.sections.education.0.institution", "MIT"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.sections.education.0.institution", "MIT"},
		},
		{
			// **An unknown key is still collected.** `err_bad_override_key`
			// expects the model to reject it, not the parser — so nothing here
			// may filter on whether the path exists.
			name:   "unknown override is collected",
			args:   []string{"render", "cv.yaml", "--cv.no_such_field", "x"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.no_such_field", "x"},
		},
		{
			// **A dotted flag with no value is an extra, not a parser
			// error.** Upstream collects it and the odd count is what
			// reports: `There is a problem with the extra arguments
			// (--cv.phone)!`. Measured against the vendored CLI.
			name:   "dangling override",
			args:   []string{"render", "cv.yaml", "--cv.phone"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--cv.phone"},
		},
		{
			// A single-dash token that is not one of upstream's short forms
			// is an extra: click puts it in `ctx.args`, and
			// `parse_override_arguments` is what rejects it, with
			// `The key (-x) should start with double dashes!` once it has a
			// value beside it.
			name:   "unknown single dash is an extra",
			args:   []string{"render", "cv.yaml", "-x"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"-x"},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, extras := Normalize(row.args)
			if !reflect.DeepEqual(rest, row.rest) {
				t.Errorf("rest = %q, want %q", rest, row.rest)
			}
			if !slices.Equal(extras, row.extras) {
				t.Errorf("extras = %q, want %q", extras, row.extras)
			}
		})
	}
}
