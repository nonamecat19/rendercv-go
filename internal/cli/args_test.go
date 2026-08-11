package cli

import (
	"io"
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

// TestNormalizeFillsInputFromFirstLeftover pins click's positional matching.
//
// **An unrecognized token is an argument, not an error.** `render` is declared
// `allow_extra_args` + `ignore_unknown_options`
// (`render_command.py:26`), so click's parser appends everything it cannot
// match to `state.largs` (`parser.py`, `_process_opts` and `_match_short_opt`)
// and then `_process_args_for_args` unpacks that list against the declared
// arguments — `INPUT_FILE_NAME` takes the **first** of them, whatever it looks
// like, and only what is left becomes `ctx.args`.
//
// Measured against the vendored CLI: `render --version` ends in
// `FileNotFoundError: … '/<cwd>/--version'`, and so do `--nope`, `-x`,
// `--helpx`, `--typ out.typ John_Doe_CV.yaml` (which opens `--typ`, not
// `out.typ`) and `--cv.name Jane` (which opens `--cv.name`). The port routed
// all six to the extras and reported a missing argument at exit 2.
//
// A leftover that begins with a dash is fenced behind a `--` at the end of the
// vector, because it is the *input file name* and pflag would otherwise parse
// it as an option; the fence is invisible to everything downstream, which sees
// exactly one positional.
func TestNormalizeFillsInputFromFirstLeftover(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			// `--version` is a root option; `render` does not declare it.
			name: "an unknown long option is the input file",
			args: []string{"render", "--version"},
			rest: []string{"render", "--", "--version"},
		},
		{
			name: "an unrecognized long option",
			args: []string{"render", "--nope"},
			rest: []string{"render", "--", "--nope"},
		},
		{
			name: "an unrecognized short cluster",
			args: []string{"render", "-x"},
			rest: []string{"render", "--", "-x"},
		},
		{
			// Not a prefix match for `--help`: click's table is exact.
			name: "a near miss on a declared option",
			args: []string{"render", "--helpx"},
			rest: []string{"render", "--", "--helpx"},
		},
		{
			// **`--typ` is not `-typ`.** Upstream declares the single-dash
			// spelling, so the double-dash one is unknown, becomes the input
			// file, and the real file name is demoted to an extra.
			name:   "a double-dashed short form displaces the real file",
			args:   []string{"render", "--typ", "out.typ", "John_Doe_CV.yaml"},
			rest:   []string{"render", "--", "--typ"},
			extras: []string{"out.typ", "John_Doe_CV.yaml"},
		},
		{
			name:   "a dotted override with no input file",
			args:   []string{"render", "--cv.name", "Jane"},
			rest:   []string{"render", "--", "--cv.name"},
			extras: []string{"Jane"},
		},
		{
			// **`--` does not exempt what follows from being the argument.**
			// Click returns from `_process_args_for_options` leaving the rest
			// in `rargs`, and `_process_args_for_args` unpacks
			// `largs + rargs` — so the first token after `--` is the input
			// file. Measured: `render -- -notyp -nomd` opens `-notyp`.
			name:   "the first token after a double dash",
			args:   []string{"render", "--", "-notyp", "-nomd"},
			rest:   []string{"render", "--", "-notyp"},
			extras: []string{"-nomd"},
		},
		{
			// A real file still wins when it comes first, and the fence is
			// not emitted for it.
			name:   "a plain file name keeps its place",
			args:   []string{"render", "cv.yaml", "--nope"},
			rest:   []string{"render", "cv.yaml"},
			extras: []string{"--nope"},
		},
		{
			// Order is the vector's, not "options first": the unknown option
			// precedes the file and therefore takes the argument slot.
			name:   "the earliest leftover wins",
			args:   []string{"render", "--nope", "cv.yaml"},
			rest:   []string{"render", "--", "--nope"},
			extras: []string{"cv.yaml"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rest, extras := Normalize(test.args)

			if !reflect.DeepEqual(rest, test.rest) {
				t.Errorf("rest = %q, want %q", rest, test.rest)
			}
			if !slices.Equal(extras, test.extras) {
				t.Errorf("extras = %q, want %q", extras, test.extras)
			}
		})
	}
}

// TestRenderTakesInputFromFirstLeftover is the same rule at the CLI boundary:
// the six measured vectors must reach `render` as an input file it then fails
// to open — upstream's `FileNotFoundError`, exit 1 — rather than dying in the
// parser with `Missing argument 'INPUT_FILE_NAME'.` at exit 2.
func TestRenderTakesInputFromFirstLeftover(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
	}{
		{name: "version", args: []string{"render", "--version"}, input: "--version"},
		{name: "unknown long", args: []string{"render", "--nope"}, input: "--nope"},
		{name: "unknown short", args: []string{"render", "-x"}, input: "-x"},
		{name: "near miss", args: []string{"render", "--helpx"}, input: "--helpx"},
		{
			name:  "double-dashed short form",
			args:  []string{"render", "--typ", "out.typ", "John_Doe_CV.yaml"},
			input: "--typ",
		},
		{name: "dotted override", args: []string{"render", "--cv.name", "Jane"}, input: "--cv.name"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var seen *RenderOptions
			code := execute(test.args, io.Discard, io.Discard, runners{
				render: func(options RenderOptions, _, _ io.Writer) int {
					seen = &options
					return 0
				},
			})

			if seen == nil {
				t.Fatalf("render was not invoked; exit code %d", code)
			}
			if seen.InputPath != test.input {
				t.Errorf("InputPath = %q, want %q", seen.InputPath, test.input)
			}
		})
	}
}

// TestNormalizeSplitsShortClusters pins click's `_match_short_opt` fallback.
//
// A single-dash token that is not an exact match in the option table is not
// simply an extra: click walks it one character at a time against the
// **one-character** options, and a match that takes a value swallows the rest
// of the token — or the following argument when it sits at the end.
// Characters that match nothing come back together as one `-xyz` extra.
//
// Every row was measured against the vendored CLI, checking the output folder
// it actually wrote as well as its exit code, because a mis-split silently
// renders to the wrong directory rather than failing.
func TestNormalizeSplitsShortClusters(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		rest   []string
		extras []string
	}{
		{
			// The value is the remainder of the token: upstream writes ./OUT.
			name: "a value option with an attached value",
			args: []string{"render", "cv.yaml", "-oOUT"},
			rest: []string{"render", "cv.yaml", "--output-folder", "OUT"},
		},
		{
			// **The `=` is not stripped** for a short option: upstream writes
			// a folder literally named `=OUT`.
			name: "an equals sign is part of the value",
			args: []string{"render", "cv.yaml", "-o=OUT"},
			rest: []string{"render", "cv.yaml", "--output-folder", "=OUT"},
		},
		{
			name: "a repeated boolean",
			args: []string{"render", "cv.yaml", "-qq"},
			rest: []string{"render", "cv.yaml", "--quiet", "--quiet"},
		},
		{
			// A boolean then a value option, which takes the next token.
			name: "a boolean followed by a value option",
			args: []string{"render", "cv.yaml", "-qo", "OUT"},
			rest: []string{"render", "cv.yaml", "--quiet", "--output-folder", "OUT"},
		},
		{
			// Both halves at once: `t`, `y` and `p` match nothing and come
			// back as `-typ`, while `o` matches and takes `ut.typ`.
			name:   "unknown characters and a match in one token",
			args:   []string{"render", "cv.yaml", "-typout.typ"},
			rest:   []string{"render", "cv.yaml", "--output-folder", "ut.typ"},
			extras: []string{"-typ"},
		},
		{
			name:   "no character matches anything",
			args:   []string{"render", "cv.yaml", "-xyz"},
			extras: []string{"-xyz"},
			rest:   []string{"render", "cv.yaml"},
		},
		{
			// The exact table still wins: `-notyp` is a whole word, not a
			// cluster of `n`, `o`, `t`, `y`, `p`.
			name: "an exact whole-word form is not split",
			args: []string{"render", "cv.yaml", "-notyp"},
			rest: []string{"render", "cv.yaml", "--dont-generate-typst"},
		},
		{
			// `-lc` likewise, which would otherwise split into two unknowns.
			name: "a two-letter whole-word form is not split",
			args: []string{"render", "cv.yaml", "-lc", "en.yaml"},
			rest: []string{"render", "cv.yaml", "--locale-catalog", "en.yaml"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rest, extras := Normalize(test.args)

			if !reflect.DeepEqual(rest, test.rest) {
				t.Errorf("rest = %q, want %q", rest, test.rest)
			}
			if len(extras) != len(test.extras) || (len(extras) > 0 && !slices.Equal(extras, test.extras)) {
				t.Errorf("extras = %q, want %q", extras, test.extras)
			}
		})
	}
}
