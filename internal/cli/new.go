package cli

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// samples are the starter CVs `rendercv new` writes, captured from the vendored
// Python by tools/sampleprobe. They are **data, not a template**: upstream
// builds them from its own models, and 369 lines of hand-copied YAML would be a
// golden by another name (AGENTS.md §10.1).
//
//go:embed samples/*.yaml
var samples embed.FS

// ErrSampleNameUnsupported is what `new` returns for a name other than the one
// the samples were captured with.
//
// **The samples are per-name, and only `John Doe` was captured.** Upstream
// generates the file from the name it is given, so `new "Jane Roe"` produces a
// document with her name in `cv.name` *and* in the file name. This port cannot
// synthesize that from a captured sample without knowing which of the ~40
// occurrences of `John Doe` in the file are the name and which are sample prose
// — the header's `John Doe`, yes, but also `John_Doe`, `johndoe` in URLs, and
// `John Doe`'s appearance inside publication author lists.
//
// Guessing is exactly the kind of silent wrongness this port keeps finding, so
// it reports instead. Resolving it needs either a real port of upstream's
// sample builder or a probe that captures a second name and diffs the two — the
// diff is the substitution rule, measured rather than assumed.
var ErrSampleNameUnsupported = errors.New(
	`only "John Doe" is supported by ` + "`new`" + ` so far: the starter CV is captured from
upstream per name, and the substitution rule for another name has not been measured
(see internal/cli/new.go)`)

// NewOptions are `new`'s flags (spec 012 §3).
type NewOptions struct {
	Name   string
	Theme  string
	Locale string
	// CreateTypstTemplates additionally writes the theme's Typst templates
	// beside the input. It is **not implemented**, for the reason
	// `create-theme` is not: the port's templates are the pongo2 transform of
	// upstream's, so writing either form is wrong in a different way. See
	// `STATE.md`.
	CreateTypstTemplates bool
}

// New is the `new` command.
func New(options NewOptions, stdout, stderr io.Writer) int {
	if options.Name != "John Doe" {
		fail(stderr, ErrSampleNameUnsupported)
		return 4
	}
	if options.CreateTypstTemplates {
		fail(stderr, errors.New(
			"--create-typst-templates is not implemented: the port's templates are the pongo2 "+
				"transform of upstream's, so neither form is the right thing to write"))
		return 4
	}

	variant, err := sampleVariant(options)
	if err != nil {
		fail(stderr, err)
		return 4
	}

	content, err := samples.ReadFile("samples/" + variant + ".yaml")
	if err != nil {
		fail(stderr, fmt.Errorf("no starter CV for %s: %w", variant, err))
		return 4
	}

	path := strings.ReplaceAll(options.Name, " ", "_") + "_CV.yaml"
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fail(stderr, err)
		return 4
	}

	_, _ = fmt.Fprint(stdout, newBanner(path))
	return 0
}

// sampleVariant maps the theme and locale flags onto a captured sample.
//
// **Only one of the two may be non-default**, because that is all the probe
// captured — the corpus has no case setting both, and inventing the combination
// would mean shipping a sample no upstream run ever produced.
func sampleVariant(options NewOptions) (string, error) {
	theme := options.Theme
	locale := options.Locale

	switch {
	case theme == "" && locale == "":
		return "default", nil
	case theme != "" && locale != "":
		return "", errors.New("setting both --theme and --locale is not supported yet: " +
			"no starter CV was captured for the combination")
	case theme != "":
		return "theme_" + theme, nil
	default:
		return "locale_" + locale, nil
	}
}

// Version is the upstream version this port mirrors. It appears in `new`'s
// greeting and is the one place the port must claim upstream's number rather
// than its own.
const Version = "2.8"

// newBanner is `new`'s stdout: a greeting, a links panel and a next-steps panel.
//
// **`rendercv render …` in the last row is the sanctioned binary-name
// divergence** (`AGENTS.md` §1, spec 012 §1 behavior 4). The instruction has to
// name the binary the user actually has, so this line cannot match the golden
// and must not.
func newBanner(path string) string {
	var out strings.Builder
	// **A leading blank line**, which Rich emits before the greeting and which
	// is easy to lose: it is the first byte of the golden, so dropping it shifts
	// the whole comparison by one.
	fmt.Fprintf(&out, "\nWelcome to RenderCV v%s!\n\n", Version)

	out.WriteString(Panel("Useful Links", []PanelRow{
		{Text: "RenderCV App:   https://rendercv.com"},
		{Text: "Documentation:  https://docs.rendercv.com"},
		{Text: "Source code:    https://github.com/rendercv/rendercv/"},
		{Text: "Bug reports:    https://github.com/rendercv/rendercv/issues/"},
	}))

	out.WriteString(Panel("Get started", []PanelRow{
		{Text: "✓ Created your YAML input file: ./" + path},
		{IsText: true},
		{Text: "Next steps:"},
		{Text: "  1. Edit the YAML input file with your information"},
		{Text: "  2. Run: rendercv-go render " + path},
	}))
	return out.String()
}
