package cli

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
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
	// beside the input, in the port's own pongo2 dialect (D-005) — the same
	// files `create-theme` writes, minus `init.lua`, which a built-in theme
	// has no use for.
	CreateTypstTemplates bool
	// CreateMarkdownTemplates is the companion of the above
	// (`new_command.py:57`), and is unimplemented for the same reason.
	CreateMarkdownTemplates bool
}

// New is the `new` command.
func New(options NewOptions, stdout, stderr io.Writer) int {
	// **The two flag checks come before anything is printed**, greeting
	// included (`new_command.py:65-77`): an unknown theme or locale is a
	// `RenderCVUserError`, which reaches the user as the `Error` panel and
	// exit 1.
	if err := checkNewFlags(options); err != nil {
		failPanel(stdout, err)
		return exitValidationError
	}

	if options.Name != "John Doe" {
		fail(stderr, ErrSampleNameUnsupported)
		return exitValidationError
	}
	variant, err := sampleVariant(options)
	if err != nil {
		fail(stderr, err)
		return exitValidationError
	}

	content, err := samples.ReadFile("samples/" + variant + ".yaml")
	if err != nil {
		fail(stderr, fmt.Errorf("no starter CV for %s: %w", variant, err))
		return exitValidationError
	}

	path := strings.ReplaceAll(options.Name, " ", "_") + "_CV.yaml"
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fail(stderr, err)
		return exitValidationError
	}

	var created, existing []templateItem
	if options.CreateTypstTemplates {
		// `theme` defaults to `"classic"` upstream (`new_command.py:35`), and
		// that is the folder name `copy_templates` writes to whether or not
		// the flag was given explicitly.
		theme := options.Theme
		if theme == "" {
			theme = "classic"
		}
		item := templateItem{desc: "Typst templates", path: theme}
		wrote, err := writeTemplatesIfAbsent(theme, copyTypstTemplates)
		if err != nil {
			fail(stderr, err)
			return exitValidationError
		}
		if wrote {
			created = append(created, item)
		} else {
			existing = append(existing, item)
		}
	}
	if options.CreateMarkdownTemplates {
		// The folder name is a fixed literal upstream (`new_command.py:87`),
		// not derived from any flag.
		item := templateItem{desc: "Markdown templates", path: "markdown"}
		wrote, err := writeTemplatesIfAbsent("markdown", copyMarkdownTemplates)
		if err != nil {
			fail(stderr, err)
			return exitValidationError
		}
		if wrote {
			created = append(created, item)
		} else {
			existing = append(existing, item)
		}
	}

	_, _ = fmt.Fprint(stdout, newBanner(path, templateRows(created, existing)))
	return 0
}

// templateItem is one row of the "Also created" / "Not modified" block —
// `new_command.py`'s `(description, path)` pair, minus the input file, which
// the banner's first row already covers.
type templateItem struct {
	desc string
	path string
}

// writeTemplatesIfAbsent writes a template folder unless one already sits
// there — `new_command.py:110-115` skips a creator whose path already exists
// rather than overwriting it.
func writeTemplatesIfAbsent(folder string, copy func(string) error) (created bool, err error) {
	if _, statErr := os.Stat(folder); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return false, statErr
	}
	if err := copy(folder); err != nil {
		return false, err
	}
	return true, nil
}

// templateRows is the "Also created" / "Not modified" block
// `new_command.py:150-166` appends to the "Get started" panel. **Each section
// gets its own leading blank row**, independently of the other — a `--create-
// typst-templates --create-markdown-templates` run where one folder already
// exists produces both sections, each with one entry.
func templateRows(created, existing []templateItem) []PanelRow {
	var rows []PanelRow
	if len(created) > 0 {
		rows = append(rows, PanelRow{IsText: true}, PanelRow{Text: "Also created:"})
		for _, item := range created {
			rows = append(rows, PanelRow{Text: fmt.Sprintf("  ○ %s: ./%s", item.desc, item.path)})
		}
	}
	if len(existing) > 0 {
		rows = append(rows, PanelRow{IsText: true}, PanelRow{Text: "Not modified (already exist):"})
		for _, item := range existing {
			rows = append(rows, PanelRow{Text: fmt.Sprintf("  - %s: ./%s", item.desc, item.path)})
		}
	}
	if len(created) > 0 || len(existing) > 0 {
		rows = append(rows, PanelRow{IsText: true},
			PanelRow{Text: "Templates are for advanced design customization. You can ignore or delete them."})
	}
	return rows
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
func newBanner(path string, templatesRows []PanelRow) string {
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

	rows := []PanelRow{
		{Text: "✓ Created your YAML input file: ./" + path},
		{IsText: true},
		{Text: "Next steps:"},
		{Text: "  1. Edit the YAML input file with your information"},
		{Text: "  2. Run: rendercv-go render " + path},
	}
	// **The templates block is appended, not interleaved** — upstream builds
	// the same "Get started" panel one `lines.append` at a time
	// (`new_command.py:150-166`), so its rows always follow the next-steps
	// block rather than replacing any of it.
	rows = append(rows, templatesRows...)
	out.WriteString(Panel("Get started", rows))
	return out.String()
}

// checkNewFlags is `new_command.py:65-77`: the theme first, then the locale,
// each against the list the schema publishes. The messages are upstream's, with
// the list joined by `", "` exactly as `str.join` produces it.
func checkNewFlags(options NewOptions) error {
	if options.Theme != "" && !slices.Contains(design.BuiltInThemes, options.Theme) {
		//nolint:staticcheck // upstream's text
		return fmt.Errorf("Theme %s is not available. Available themes are: %s",
			options.Theme, strings.Join(design.BuiltInThemes, ", "))
	}
	if options.Locale != "" && !slices.Contains(locale.AvailableLocales(), options.Locale) {
		//nolint:staticcheck // upstream's text
		return fmt.Errorf("Locale %s is not available. Available locales are: %s",
			options.Locale, strings.Join(locale.AvailableLocales(), ", "))
	}
	return nil
}
