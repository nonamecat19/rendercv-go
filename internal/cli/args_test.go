package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

// TestScanCollectsPathParametersInClicksOrder is spec 012 §2.1 behaviors 11a,
// 11c and 11d: which parameters carry the readability check, how each names
// itself, and — the part an implementation is most likely to get wrong — the
// order they are checked in.
//
// **Options are processed in the order they were typed, then the positional
// argument**, because click's `_process_args_for_args` runs after the option
// loop. Validating the input file first because it feels primary reports the
// wrong parameter for half the vectors below.
func TestScanCollectsPathParametersInClicksOrder(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		paths []pathParam
	}{
		{
			// The argument's shape has no short spelling and no slash.
			name:  "the input file alone",
			args:  []string{"render", "cv.yaml"},
			paths: []pathParam{{display: "'INPUT_FILE_NAME'", value: "cv.yaml"}},
		},
		{
			// **Both spellings, long then short, whichever was typed** (11c).
			name: "a short option names its long form too",
			args: []string{"render", "cv.yaml", "-d", "design.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "design.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			name: "the long spelling names the same pair",
			args: []string{"render", "cv.yaml", "--design", "design.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "design.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			name: "the equals form carries its value",
			args: []string{"render", "cv.yaml", "--design=design.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "design.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			// `-dX` — click's `_match_short_opt` takes the remainder as the
			// value, and the parameter is still checked.
			name: "a value attached inside a short cluster",
			args: []string{"render", "cv.yaml", "-ddesign.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "design.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			// **The option wins over the argument even when typed second**
			// (11d): measured, `render unreadable.yaml -d u2.yaml` reports
			// `--design`.
			name: "an option typed after the input file is still first",
			args: []string{"render", "cv.yaml", "-d", "design.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "design.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			// Two options keep the order they were typed in.
			name: "settings before design",
			args: []string{"render", "cv.yaml", "-s", "s.yaml", "-d", "d.yaml"},
			paths: []pathParam{
				{display: "'--settings' / '-s'", value: "s.yaml"},
				{display: "'--design' / '-d'", value: "d.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			name: "design before settings",
			args: []string{"render", "cv.yaml", "-d", "d.yaml", "-s", "s.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "d.yaml"},
				{display: "'--settings' / '-s'", value: "s.yaml"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			// No input file at all: the option is still checked, which is why
			// upstream reports the readability error rather than the missing
			// argument.
			name: "an option with no input file",
			args: []string{"render", "-d", "d.yaml"},
			paths: []pathParam{
				{display: "'--design' / '-d'", value: "d.yaml"},
			},
		},
		{
			// **All ten, not three.** The five output paths and the output
			// folder carry the same check as the three overlays.
			name: "every path option of the ten",
			args: []string{
				"render", "cv.yaml",
				"-o", "out", "-lc", "l.yaml",
				"-typ", "a.typ", "-pdf", "a.pdf", "-md", "a.md", "-html", "a.html", "-png", "a.png",
			},
			paths: []pathParam{
				{display: "'--output-folder' / '-o'", value: "out"},
				{display: "'--locale-catalog' / '-lc'", value: "l.yaml"},
				{display: "'--typst-path' / '-typ'", value: "a.typ"},
				{display: "'--pdf-path' / '-pdf'", value: "a.pdf"},
				{display: "'--markdown-path' / '-md'", value: "a.md"},
				{display: "'--html-path' / '-html'", value: "a.html"},
				{display: "'--png-path' / '-png'", value: "a.png"},
				{display: "'INPUT_FILE_NAME'", value: "cv.yaml"},
			},
		},
		{
			// `--YAMLLOCATION` is a TEXT parameter, not a path, so it is not
			// checked however it is spelled.
			name:  "the override placeholder is not a path",
			args:  []string{"render", "cv.yaml", "--YAMLLOCATION", "zzz"},
			paths: []pathParam{{display: "'INPUT_FILE_NAME'", value: "cv.yaml"}},
		},
		{
			// A dangling value option assigns nothing: pflag reports
			// `Option '-d' requires an argument.` and that error precedes the
			// readability check — measured, 553 B at exit 2.
			name:  "a dangling value option contributes nothing",
			args:  []string{"render", "cv.yaml", "-d"},
			paths: []pathParam{{display: "'INPUT_FILE_NAME'", value: "cv.yaml"}},
		},
		{
			// Outside `render` nothing is collected: `new` declares no path
			// parameters at all.
			name:  "new has no path parameters",
			args:  []string{"new", "John Doe", "--theme", "classic"},
			paths: nil,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := scan(test.args).paths
			if !reflect.DeepEqual(got, test.paths) {
				t.Errorf("paths = %+v, want %+v", got, test.paths)
			}
		})
	}
}

// TestUnreadablePathIsAUsageError is spec 012 §2.1 behaviors 11a, 11b and 11e
// at the CLI boundary.
//
// Upstream measurements, `COLUMNS=80`, uid 1000, mode-000 targets: every one of
// the ten is exit **2** with the usage line, the `Try …` line and the `Error`
// panel on **stderr** and nothing on stdout — 637 B where the message fits one
// panel line, 722 B where it wraps. A **missing** path is not checked at all
// and falls through to the later read, which is the vector that disproves a
// uniform "validate the path" rule.
func TestUnreadablePathIsAUsageError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the check cannot be observed")
	}

	// The vectors run from inside the fixture directory, as the measurements
	// did, so the names in the messages are the short ones upstream printed
	// rather than a temporary absolute path.
	t.Chdir(t.TempDir())

	const (
		readable      = "cv.yaml"
		unreadable    = "unreadable.yaml"
		second        = "u2.yaml"
		unreadableDir = "noread_dir"
		missing       = "nosuch.yaml"
	)
	if err := os.WriteFile(readable, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{unreadable, second} {
		if err := os.WriteFile(name, []byte("theme: classic\n"), 0o000); err != nil {
			t.Fatal(err)
		}
	}
	// A directory counts too — click's default is `dir_okay=True`. It is made
	// traversable again on the way out so `t.TempDir`'s own cleanup can remove
	// it.
	if err := os.Mkdir(unreadableDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadableDir, 0o700) })

	cases := []struct {
		name string
		args []string
		// want is the `Invalid value for …` message, or "" when the vector must
		// not produce one.
		want string
	}{
		{
			name: "the input file",
			args: []string{"render", unreadable},
			want: "Invalid value for 'INPUT_FILE_NAME': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "design",
			args: []string{"render", readable, "-d", unreadable},
			want: "Invalid value for '--design' / '-d': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "locale catalog",
			args: []string{"render", readable, "-lc", unreadable},
			want: "Invalid value for '--locale-catalog' / '-lc': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "settings",
			args: []string{"render", readable, "-s", unreadable},
			want: "Invalid value for '--settings' / '-s': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "output folder, a directory",
			args: []string{"render", readable, "-o", unreadableDir},
			want: "Invalid value for '--output-folder' / '-o': Path '" + unreadableDir + "' is not readable.",
		},
		{
			// **An unreadable output target is rejected before anything is
			// written.** The port used to render the Typst file and fail
			// afterwards.
			name: "typst path",
			args: []string{"render", readable, "-typ", unreadable},
			want: "Invalid value for '--typst-path' / '-typ': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "pdf path",
			args: []string{"render", readable, "-pdf", unreadable},
			want: "Invalid value for '--pdf-path' / '-pdf': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "markdown path",
			args: []string{"render", readable, "-md", unreadable},
			want: "Invalid value for '--markdown-path' / '-md': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "html path",
			args: []string{"render", readable, "-html", unreadable},
			want: "Invalid value for '--html-path' / '-html': Path '" + unreadable + "' is not readable.",
		},
		{
			name: "png path",
			args: []string{"render", readable, "-png", unreadable},
			want: "Invalid value for '--png-path' / '-png': Path '" + unreadable + "' is not readable.",
		},
		{
			// 11d, the four ordering vectors.
			name: "an option outranks the input file typed before it",
			args: []string{"render", unreadable, "-d", second},
			want: "Invalid value for '--design' / '-d': Path '" + second + "' is not readable.",
		},
		{
			name: "settings typed first",
			args: []string{"render", readable, "-s", second, "-d", unreadable},
			want: "Invalid value for '--settings' / '-s': Path '" + second + "' is not readable.",
		},
		{
			name: "design typed first",
			args: []string{"render", readable, "-d", unreadable, "-s", second},
			want: "Invalid value for '--design' / '-d': Path '" + unreadable + "' is not readable.",
		},
		{
			// **Not `Missing argument 'INPUT_FILE_NAME'.`** — the option is
			// processed before the argument, so there is no argument left to
			// miss by the time click gives up.
			name: "no input file at all",
			args: []string{"render", "-d", unreadable},
			want: "Invalid value for '--design' / '-d': Path '" + unreadable + "' is not readable.",
		},
		{
			// The check precedes the leftover-token routing of `8eb1502`, so
			// the unknown token never becomes an input file here.
			name: "an unknown token beside an unreadable option",
			args: []string{"render", "--nope", "-d", unreadable},
			want: "Invalid value for '--design' / '-d': Path '" + unreadable + "' is not readable.",
		},
		{
			// 11b: `exists=False`. Nothing is reported here — the render runs
			// and the read fails later, on its own terms.
			name: "a missing path is not checked",
			args: []string{"render", readable, "-d", missing},
			want: "",
		},
		{
			name: "a readable path is not reported",
			args: []string{"render", readable, "-d", readable},
			want: "",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			invoked := false
			code := execute(test.args, &stdout, &stderr, runners{
				render: func(RenderOptions, io.Writer, io.Writer) int {
					invoked = true
					return 0
				},
			})

			if test.want == "" {
				if !invoked {
					t.Fatalf("render was not reached; exit %d, stderr %q", code, stderr.String())
				}
				return
			}

			if invoked {
				t.Errorf("render ran; the path should have been rejected at parse time")
			}
			if code != exitUsageError {
				t.Errorf("exit = %d, want %d", code, exitUsageError)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing — click writes usage errors to stderr", stdout.String())
			}
			text := stderr.String()
			// The panel wraps a long message across lines, exactly as
			// upstream's does — `--locale-catalog` and the five output paths
			// measure 722 B against `--design`'s 637 B for that reason — so the
			// comparison is against the unwrapped text.
			if got := panelBody(text); got != test.want {
				t.Errorf("panel message = %q, want %q", got, test.want)
			}
			// All three parts, as every other usage error prints them.
			for _, line := range []string{
				"Usage: rendercv-go render [OPTIONS] INPUT_FILE_NAME\n",
				"Try 'rendercv-go render -h' for help.\n",
			} {
				if !strings.Contains(text, line) {
					t.Errorf("stderr = %q, want it to contain %q", text, line)
				}
			}
		})
	}
}

// TestHelpOutranksTheReadabilityCheck is click's eager parameter processing:
// `--help` is `is_eager`, so it is handled before any other parameter is
// converted. Measured both orders — `render -h -d unreadable.yaml` and
// `render -d unreadable.yaml -h` are each the help page on stdout at exit 0,
// 5661 B, with nothing on stderr.
func TestHelpOutranksTheReadabilityCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the check cannot be observed")
	}

	dir := t.TempDir()
	unreadable := filepath.Join(dir, "unreadable.yaml")
	if err := os.WriteFile(unreadable, []byte("theme: classic\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"render", "-h", "-d", unreadable},
		{"render", "-d", unreadable, "-h"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := execute(args, &stdout, &stderr, runners{})

			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want nothing", stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), "\nUsage:") && stdout.Len() == 0 {
				t.Errorf("stdout = %q, want the help page", stdout.String())
			}
		})
	}
}

// TestAMissingOptionValueOutranksTheReadabilityCheck is the other precedence
// click's parser imposes: a value option with nothing after it fails during
// *parsing*, before any parameter is converted. Measured:
// `render cv.yaml -d unreadable.yaml -s` is `Option '-s' requires an
// argument.` at exit 2, 553 B — the panel alone, with no usage line, which is
// G-3's asymmetry.
func TestAMissingOptionValueOutranksTheReadabilityCheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the check cannot be observed")
	}

	dir := t.TempDir()
	readable := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(readable, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(dir, "unreadable.yaml")
	if err := os.WriteFile(unreadable, []byte("theme: classic\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := execute([]string{"render", readable, "-d", unreadable, "-s"}, &stdout, &stderr, runners{})

	if code != exitUsageError {
		t.Errorf("exit = %d, want %d", code, exitUsageError)
	}
	if want := "Option '-s' requires an argument."; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
	if strings.Contains(stderr.String(), "is not readable") {
		t.Error("the readability check ran; click fails on the missing value first")
	}
}

// panelBody is a Rich panel's text with its box drawing and its wrapping
// removed, so a message can be compared as one line however wide the terminal
// was when it was rendered.
func panelBody(text string) string {
	var words []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		words = append(words, strings.Fields(strings.Trim(line, "│"))...)
	}
	return strings.Join(words, " ")
}
