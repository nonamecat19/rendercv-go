package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// renderShortFlags maps every short spelling `render` declares to its long one
// (spec 012 §2 behavior 6, `render_command.py:33-188`).
//
// **They are not shorthands.** `-typ`, `-nopdf` and `-lc` are whole words after
// a single dash, which neither pflag nor GNU getopt accepts, so the argument
// vector is rewritten before any parser sees it. The single-character members —
// `-o`, `-d`, `-s`, `-w`, `-q` — could have been registered as real pflag
// shorthands, and are rewritten here anyway so that one mechanism covers the
// whole table and a reader can diff it against upstream's signature in one
// pass.
var renderShortFlags = map[string]string{
	"o":      "output-folder",
	"d":      "design",
	"lc":     "locale-catalog",
	"s":      "settings",
	"typ":    "typst-path",
	"pdf":    "pdf-path",
	"md":     "markdown-path",
	"html":   "html-path",
	"png":    "png-path",
	"nomd":   "dont-generate-markdown",
	"nohtml": "dont-generate-html",
	"notyp":  "dont-generate-typst",
	"nopdf":  "dont-generate-pdf",
	"nopng":  "dont-generate-png",
	"w":      "watch",
	"q":      "quiet",
	// **`-h` is `--help` on every command**, from the app's
	// `help_option_names` (`cli/app.py:22-25`). Without this entry the
	// unrecognized-token rule below collects it as an override key, and
	// `render -h` exits 2 with a missing-argument error where upstream prints
	// the page and exits 0. Found by a test, not by reading.
	"h": "help",
}

// renderValueFlags is which of `render`'s long options take a value, so the
// pre-pass knows whether the following token is that value or the next
// argument. Everything absent from this set is a boolean.
var renderValueFlags = map[string]bool{
	"output-folder":  true,
	"design":         true,
	"locale-catalog": true,
	"settings":       true,
	"typst-path":     true,
	"pdf-path":       true,
	"markdown-path":  true,
	"html-path":      true,
	"png-path":       true,
	// Declared, never read — see renderBoolFlags' note. It takes a value
	// (metavar TEXT), so it must consume the next token or that token becomes a
	// stray extra.
	"YAMLLOCATION": true,
}

// renderBoolFlags is the rest of the table, plus help.
//
// **`YAMLLOCATION` is declared and never read** (G-2). Upstream declares it
// (`render_command.py:190-197`) purely so the help panel has a row describing
// the dotted-override mechanism, and binds it to `_`. Leaving it out made
// `--YAMLLOCATION zzz` an override key the model then rejected, where upstream
// accepts it in silence.
var renderBoolFlags = map[string]bool{
	"dont-generate-markdown": true,
	"dont-generate-html":     true,
	"dont-generate-typst":    true,
	"dont-generate-pdf":      true,
	"dont-generate-png":      true,
	"watch":                  true,
	"quiet":                  true,
	"help":                   true,
}

// renderPathOptions are `render`'s options typed `pathlib.Path`, mapped to
// their short spelling (spec 012 §2.1 behavior 11a).
//
// **Typer converts a `pathlib.Path` annotation to
// `click.Path(exists=False, readable=True, dir_okay=True)`**, so every one of
// them — the three overlays, the output folder and all five output paths — is
// checked for readability at parse time, and none is checked for existence.
// `--YAMLLOCATION` is absent because it is a TEXT parameter, not a path.
var renderPathOptions = map[string]string{
	"output-folder":  "o",
	"design":         "d",
	"locale-catalog": "lc",
	"settings":       "s",
	"typst-path":     "typ",
	"pdf-path":       "pdf",
	"markdown-path":  "md",
	"html-path":      "html",
	"png-path":       "png",
}

// pathParam is one path-typed parameter as click will convert it: the name it
// reports itself by, and the value as the user spelled it.
type pathParam struct {
	// display is click's parameter name in an `Invalid value for …` message.
	// **An option names both spellings, long then short** — `'--design' / '-d'`
	// — whichever the user typed, and the argument has no slash at all
	// (behavior 11c).
	display string
	value   string
}

// inputFileDisplay is the argument's name in click's messages. Upstream's
// parameter is `input_file_name`, and click upper-cases it for the metavar.
const inputFileDisplay = "'INPUT_FILE_NAME'"

// scanned is one walk of the argument vector: what a flag parser can read, what
// it cannot, and every path parameter the walk assigned.
type scanned struct {
	rest   []string
	extras []string
	// paths is in click's processing order — **options in the order they were
	// typed, then the positional argument** (behavior 11d), because
	// `_process_args_for_args` runs after the option loop.
	paths []pathParam
}

// renderShortChars are the one-character members of renderShortFlags, which
// are the only options click's `_match_short_opt` can match inside a cluster.
// The whole-word forms (`-typ`, `-nopdf`, `-lc`) are matched exactly, before
// any splitting, and never participate here.
var renderShortChars = map[byte]string{
	'o': "output-folder",
	'd': "design",
	's': "settings",
	'w': "watch",
	'q': "quiet",
	'h': "help",
}

// splitShortCluster walks a single-dash token character by character the way
// click's `_match_short_opt` does, appending each match's long form to rest.
//
// A matched option that takes a value consumes the **rest of the token** as
// that value, or the following argument when it sits at the end — so `-oOUT`
// and `-o OUT` are the same invocation, and parsing of the token stops there.
// Characters that match nothing are collected and returned together, to be
// re-emitted as one `-xyz` extra exactly as click does.
//
// assign is told about every value the walk hands to an option, so a path
// parameter spelled inside a cluster — `-ddesign.yaml` — is checked like any
// other.
func splitShortCluster(arg string, rest *[]string, args []string, i *int, assign func(long, value string)) (consumed bool, unknown string) {
	var unmatched []byte

	for pos := 1; pos < len(arg); pos++ {
		long, ok := renderShortChars[arg[pos]]
		if !ok {
			unmatched = append(unmatched, arg[pos])
			continue
		}

		*rest = append(*rest, "--"+long)
		consumed = true

		if !renderValueFlags[long] {
			continue
		}
		if remainder := arg[pos+1:]; remainder != "" {
			*rest = append(*rest, remainder)
			assign(long, remainder)
		} else if *i+1 < len(args) {
			*rest = append(*rest, args[*i+1])
			assign(long, args[*i+1])
			*i++
		}
		break // a value option swallows the remainder of the token
	}
	return consumed, string(unmatched)
}

// Normalize rewrites `render`'s single-dash spellings into their long form and
// splits the vector into what a flag parser can read and what it cannot.
//
// **The extras cannot go through a flag parser at all**: `--cv.phone` is not a
// declared flag and never will be, since the set is every path in the schema.
// They come back as an ordered list for ParseOverrideArguments, which is where
// upstream reads them too — and reading them there rather than here is what
// keeps an unknown key a *validation* error rather than a CLI error (spec §2
// behavior 10).
//
// The short-form rewrite is scoped to `render` because that is where upstream
// declares the options. `new -d x` is an unknown option to upstream and stays
// one here rather than turning into a flag `new` does not have.
func Normalize(args []string) (rest, extras []string) {
	vector := scan(args)
	return vector.rest, vector.extras
}

// scan is Normalize's whole body, plus the path parameters the walk assigns.
// Both readings come from one pass because they have to agree: the option that
// reports an unreadable path is the option whose value the walk consumed, down
// to the cluster and `=` spellings.
func scan(args []string) scanned {
	// Only `render` collects extras — it is the one command declared with
	// `allow_extra_args`. Everything else goes through untouched, so a token
	// `new` does not declare stays the unknown option it is upstream. It
	// declares no path parameters either, so nothing is checked there.
	if subcommand(args) != "render" {
		return scanned{rest: args}
	}

	var (
		extras []string
		paths  []pathParam
	)
	rest := make([]string, 0, len(args))

	// assign records a value handed to a path option, in the order the walk
	// meets it — which is the order it was typed, and therefore click's
	// processing order for options.
	assign := func(long, value string) {
		short, isPath := renderPathOptions[long]
		if !isPath {
			return
		}
		paths = append(paths, pathParam{
			display: fmt.Sprintf("'--%s' / '-%s'", long, short),
			value:   value,
		})
	}

	// leftover is click's `state.largs`: every token the parser could not
	// consume, in the order it was typed. `INPUT_FILE_NAME` comes off the front
	// of it once the whole vector has been walked, and only what remains is
	// `ctx.args` — see the split below.
	var leftover []string
	// inputAt is where in rest the first leftover was typed, so a plain file
	// name goes back exactly where it stood.
	inputAt := 0
	collect := func(token string) {
		if len(leftover) == 0 {
			inputAt = len(rest)
		}
		leftover = append(leftover, token)
	}

	endOfOptions := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, long := longName(arg)

		// **A bare `--` ends option parsing and is itself dropped** (G-1).
		// Click removes it from the vector and every following token becomes a
		// leftover — declared flags included, so `-- -notyp` is the argument
		// `-notyp`, not the flag. `_process_args_for_options` returns on it and
		// `_process_args_for_args` unpacks `largs + rargs`, so what follows a
		// `--` is still eligible to be the input file: measured,
		// `render -- -notyp -nomd` opens `-notyp` and reports `(-nomd)`.
		if endOfOptions {
			collect(arg)
			continue
		}
		if arg == "--" {
			endOfOptions = true
			continue
		}

		switch {
		case long && renderValueFlags[name]:
			rest = append(rest, arg)
			// `--output-folder=out` carries its value in the same token.
			if value, hasValue := strings.CutPrefix(arg, "--"+name+"="); hasValue {
				assign(name, value)
			} else if i+1 < len(args) {
				rest = append(rest, args[i+1])
				assign(name, args[i+1])
				i++
			}

		case long && renderBoolFlags[name]:
			rest = append(rest, arg)

		case long:
			// **An unrecognized option does not swallow the next token.**
			// click appends it to `state.largs` and goes on parsing, so
			// `--nope -nopdf` leaves one leftover and a real flag, and the
			// pairing into keys and values happens later over the whole list.
			// Measured against the vendored CLI, which reports `(--nope)`
			// there.
			//
			// The `=` form is not split either: `--cv.name=Jane` is one token
			// and therefore an odd count, which is upstream's answer too.
			collect(arg)

		case strings.HasPrefix(arg, "-") && arg != "-":
			// A single-dash token: one of upstream's whole-word short forms,
			// or a cluster click splits per character.
			if replacement := renderShortFlags[strings.TrimPrefix(arg, "-")]; replacement != "" {
				rest = append(rest, "--"+replacement)
				if renderValueFlags[replacement] && i+1 < len(args) {
					rest = append(rest, args[i+1])
					assign(replacement, args[i+1])
					i++
				}
				continue
			}

			// **An inexact single-dash token is not simply an extra.** click
			// tries `_match_long_opt` first — that is the exact table above,
			// which is how the whole-word forms `-notyp` and `-lc` work — and
			// falls back to `_match_short_opt`, which walks the token one
			// character at a time against the *one-character* options only.
			//
			// Measured: `-oOUT` sets the output folder to `OUT`, `-o=OUT` sets
			// it to `=OUT` (the `=` is not stripped for a short option), `-qq`
			// is `--quiet` twice, `-qo OUT` takes its value from the next
			// token, and `-typout.typ` does **both** halves at once — `t`, `y`
			// and `p` are unknown and come back as the single extra `-typ`,
			// while `o` matches and swallows the rest of the token as its
			// value.
			consumed, unknown := splitShortCluster(arg, &rest, args, &i, assign)
			if unknown != "" {
				collect("-" + unknown)
			}
			if !consumed && unknown == "" {
				collect(arg)
			}

		case arg == "render" && len(leftover) == 0 && len(rest) == 0:
			rest = append(rest, arg)

		default:
			collect(arg)
		}
	}

	// **`INPUT_FILE_NAME` is the first leftover, whatever it looks like.**
	// `_process_args_for_args` unpacks `state.largs` against the declared
	// arguments before anything else sees it, and `render`'s one argument takes
	// the front of the list — so `render --version` opens a *file* named
	// `--version` (measured: `FileNotFoundError: … '/<cwd>/--version'`) rather
	// than reporting a usage error. Only what is left is `ctx.args`, which is
	// why `render a.yaml b.yaml` is an odd count rather than two input files.
	if len(leftover) > 0 {
		rest = withInput(rest, inputAt, leftover[0])
		if len(leftover) > 1 {
			extras = leftover[1:]
		}
		// **The argument is checked after every option** (behavior 11d), which
		// is why `render unreadable.yaml -d u2.yaml` reports `--design` though
		// the input file was typed first.
		paths = append(paths, pathParam{display: inputFileDisplay, value: leftover[0]})
	}
	return scanned{rest: rest, extras: extras, paths: paths}
}

// firstUnreadable is click's `Path(readable=True)` conversion over the walk's
// parameters, in its processing order (spec 012 §2.1 behavior 11a).
//
// Click converts each parameter in turn and fails on the first that cannot be
// read, so only the first is ever reported however many are unreadable.
func firstUnreadable(paths []pathParam) (pathParam, bool) {
	for _, param := range paths {
		if !readablePath(param.value) {
			return param, true
		}
	}
	return pathParam{}, false
}

// readablePath is `click.Path.convert`'s readability test.
//
// **A path that does not exist is not checked at all** (behavior 11b): click
// stats it first and, because these parameters are `exists=False`, returns the
// value untouched when the stat fails. So `-d nosuch.yaml` is not a usage error
// — it falls through to the unguarded read later, which is upstream's
// `FileNotFoundError` at exit 1, and a rule that validated every path uniformly
// would report it here instead.
//
// Readability itself is `os.access(rv, os.R_OK)` upstream. Opening the file is
// the portable equivalent: a mode-000 file and a mode-000 directory both fail
// with a permission error, and any other failure is left alone because click
// reports only this one.
func readablePath(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return true
	}
	file, err := os.Open(path)
	if err != nil {
		return !errors.Is(err, fs.ErrPermission)
	}
	_ = file.Close()
	return true
}

// withInput puts the input file name back into the vector a flag parser reads.
//
// A plain file name goes back where it was typed, so the vector keeps the shape
// the user gave it. **One that begins with a dash cannot**: pflag would parse
// `--version` as an option and fail the invocation where click opens a file of
// that name. It is fenced behind a `--` at the end instead — pflag consumes the
// fence and treats everything after it as positional, so `render` still sees
// exactly one argument, spelled as typed.
func withInput(rest []string, at int, input string) []string {
	if !strings.HasPrefix(input, "-") {
		return slices.Insert(rest, at, input)
	}
	return append(rest, "--", input)
}

// longName reports a `--flag`'s name with its value clipped off, and whether the
// token was a long option at all.
func longName(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "--") || arg == "--" {
		return "", false
	}
	name, _, _ := strings.Cut(arg[2:], "=")
	return name, true
}

// subcommand is the first token that is not a flag or a flag's value. It is
// only ever compared against `render`, so a dotted override's value being
// mistaken for it is harmless: the vector would have to start with one, and a
// dotted override before the subcommand is not a shape upstream accepts either.
func subcommand(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}
