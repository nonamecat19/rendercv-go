package cli

import (
	"reflect"
	"testing"
)

// Spec 012 §2 behaviors 6, 7 and 9, driven by the argument vectors the corpus
// cases actually use.
func TestNormalize(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		rest      []string
		overrides map[string]string
	}{
		{
			// `render_typst_only`: four negative flags, all single-dash. **The
			// rewrite is short-to-long, not dash-to-dashes** — upstream's own
			// name for `-nopdf` is `--dont-generate-pdf`, and the port used to
			// declare a `--nopdf` that upstream has never had.
			name:      "single dash negatives",
			args:      []string{"render", "cv.yaml", "-nopdf", "-nopng", "-nomd", "-nohtml"},
			rest:      []string{"render", "cv.yaml", "--dont-generate-pdf", "--dont-generate-png", "--dont-generate-markdown", "--dont-generate-html"},
			overrides: map[string]string{},
		},
		{
			// `render_custom_paths`: single-dash flags that take a value.
			name:      "single dash paths",
			args:      []string{"render", "cv.yaml", "-typ", "out/custom.typ", "-md", "out/n/c.md"},
			rest:      []string{"render", "cv.yaml", "--typst-path", "out/custom.typ", "--markdown-path", "out/n/c.md"},
			overrides: map[string]string{},
		},
		{
			// **The rewrite is scoped to `render`.** `new` declares none of
			// these, so a token that looks like one must reach the parser
			// unchanged rather than becoming a flag the subcommand lacks.
			name:      "short forms are not rewritten outside render",
			args:      []string{"new", "John Doe", "-typ"},
			rest:      []string{"new", "John Doe", "-typ"},
			overrides: map[string]string{},
		},
		{
			// `render_override_scalar`.
			name:      "scalar override",
			args:      []string{"render", "cv.yaml", "--cv.phone", "+1-555-555-5555"},
			rest:      []string{"render", "cv.yaml"},
			overrides: map[string]string{"cv.phone": "+1-555-555-5555"},
		},
		{
			// `render_override_indexed` — the index is part of the path.
			name:      "indexed override",
			args:      []string{"render", "cv.yaml", "--cv.sections.education.0.institution", "MIT"},
			rest:      []string{"render", "cv.yaml"},
			overrides: map[string]string{"cv.sections.education.0.institution": "MIT"},
		},
		{
			// **An unknown key is still collected.** `err_bad_override_key`
			// expects the model to reject it, not the parser — so nothing here
			// may filter on whether the path exists.
			name:      "unknown override is collected",
			args:      []string{"render", "cv.yaml", "--cv.no_such_field", "x"},
			rest:      []string{"render", "cv.yaml"},
			overrides: map[string]string{"cv.no_such_field": "x"},
		},
		{
			// A dotted flag with no value is left for the parser to report
			// rather than silently consuming the end of the vector.
			name:      "dangling override",
			args:      []string{"render", "cv.yaml", "--cv.phone"},
			rest:      []string{"render", "cv.yaml", "--cv.phone"},
			overrides: map[string]string{},
		},
		{
			// A single-dash token that is not one of upstream's long flags is
			// left alone, so a future real shorthand is not corrupted.
			name:      "unknown single dash is untouched",
			args:      []string{"render", "cv.yaml", "-x"},
			rest:      []string{"render", "cv.yaml", "-x"},
			overrides: map[string]string{},
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			rest, overrides := Normalize(row.args)
			if !reflect.DeepEqual(rest, row.rest) {
				t.Errorf("rest = %q, want %q", rest, row.rest)
			}
			if !reflect.DeepEqual(overrides, row.overrides) {
				t.Errorf("overrides = %v, want %v", overrides, row.overrides)
			}
		})
	}
}
