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
	rest = make([]string, 0, len(args))
	rendering := subcommand(args) == "render"

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case strings.HasPrefix(arg, "--") && strings.Contains(arg, "."):
			// A dotted override always takes the next argument as its value.
			// A trailing one with no value is left in place so the parser can
			// report it.
			if i+1 >= len(args) {
				rest = append(rest, arg)
				continue
			}
			extras = append(extras, arg, args[i+1])
			i++

		case rendering && strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") &&
			renderShortFlags[strings.TrimPrefix(arg, "-")] != "":
			rest = append(rest, "--"+renderShortFlags[strings.TrimPrefix(arg, "-")])

		default:
			rest = append(rest, arg)
		}
	}
	return rest, extras
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
