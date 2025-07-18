package cli

import "strings"

// singleDashFlags are the flags upstream spells with **one** dash
// (`render_typst_only`, `render_custom_paths`). They are not shorthands: `-typ`
// is a whole word, which neither pflag nor GNU getopt would accept, so the
// argument vector is normalized before any parser sees it.
var singleDashFlags = map[string]bool{
	"nopdf": true, "nopng": true, "nomd": true, "nohtml": true, "notyp": true,
	"typ": true, "pdf": true, "png": true, "md": true, "html": true,
}

// Normalize rewrites upstream's single-dash long flags into double-dash form and
// pulls the arbitrary dotted overrides out of the vector.
//
// **The overrides cannot go through a flag parser at all**: `--cv.phone` is not
// a declared flag and never will be, since the set is every path in the schema.
// They are collected here and applied to the document, which is also why an
// unknown one is a *validation* error rather than a CLI error (spec §2
// behavior 10).
func Normalize(args []string) (rest []string, overrides map[string]string) {
	overrides = map[string]string{}
	rest = make([]string, 0, len(args))

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
			overrides[strings.TrimPrefix(arg, "--")] = args[i+1]
			i++

		case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") &&
			singleDashFlags[strings.TrimPrefix(arg, "-")]:
			rest = append(rest, "-"+arg)

		default:
			rest = append(rest, arg)
		}
	}
	return rest, overrides
}
