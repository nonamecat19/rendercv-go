package cli

import "strings"

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
}

// renderBoolFlags is the rest of the table, plus help.
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
	// Only `render` collects extras — it is the one command declared with
	// `allow_extra_args`. Everything else goes through untouched, so a token
	// `new` does not declare stays the unknown option it is upstream.
	if subcommand(args) != "render" {
		return args, nil
	}

	rest = make([]string, 0, len(args))
	seenInput := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, long := longName(arg)

		switch {
		case long && renderValueFlags[name]:
			rest = append(rest, arg)
			// `--output-folder=out` carries its value in the same token.
			if !strings.Contains(arg, "=") && i+1 < len(args) {
				rest = append(rest, args[i+1])
				i++
			}

		case long && renderBoolFlags[name]:
			rest = append(rest, arg)

		case long:
			// **An unrecognized option does not swallow the next token.**
			// click appends it to `ctx.args` and goes on parsing, so
			// `--nope -nopdf` leaves one extra and a real flag, and the pairing
			// into keys and values happens later over the whole list. Measured
			// against the vendored CLI, which reports `(--nope)` there.
			//
			// The `=` form is not split either: `--cv.name=Jane` is one token
			// and therefore an odd count, which is upstream's answer too.
			extras = append(extras, arg)

		case strings.HasPrefix(arg, "-") && arg != "-":
			// A single-dash token: either one of upstream's whole-word short
			// forms, or an extra.
			if replacement := renderShortFlags[strings.TrimPrefix(arg, "-")]; replacement != "" {
				rest = append(rest, "--"+replacement)
				if renderValueFlags[replacement] && i+1 < len(args) {
					rest = append(rest, args[i+1])
					i++
				}
				continue
			}
			extras = append(extras, arg)

		case arg == "render" && !seenInput && len(rest) == 0:
			rest = append(rest, arg)

		case !seenInput:
			// The first bare token after the subcommand is `input_file_name`.
			// Every later one is an extra, which is why `render a.yaml b.yaml`
			// is an odd count rather than two input files.
			seenInput = true
			rest = append(rest, arg)

		default:
			extras = append(extras, arg)
		}
	}
	return rest, extras
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
