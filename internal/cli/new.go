package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/cli/sample"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/locale"
	"github.com/nonamecat19/rendercv-go/internal/version"
)

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
	// **`new` has no `ProgressPanel` at all** (`new_command.py` builds no Live),
	// so every `RenderCVUserError` it raises escapes to `@handle_user_errors`
	// and is printed with `rich.print`, which ends with a newline. The
	// trailing-newline fix for `render` split the two writers but left this
	// call on the Live one, so the panel was a byte short.
	if err := checkNewFlags(options); err != nil {
		failPrintedPanel(stdout, err)
		return exitValidationError
	}

	// The two flags default in typer's signature, not in the generator
	// (`new_command.py:38`, `:49`), so an unset flag is `classic` / `english`
	// here rather than an empty axis.
	content, err := sample.Generate(
		options.Name,
		orDefault(options.Theme, defaultTheme),
		orDefault(options.Locale, defaultLocale),
	)
	if err != nil {
		fail(stderr, err)
		return exitValidationError
	}

	// **The input file is one item of the same exists-or-create loop the
	// template folders go through** (`new_command.py:110-119`), not a special
	// case: a path that already exists is added to `existing_items` and its
	// creator never runs. The port wrote unconditionally and then hardcoded
	// the "Created" row, so `new` silently overwrote a CV the user had already
	// filled in and reported the opposite of what it did.
	path := sample.FileName(options.Name)
	inputFileCreated, err := writeFileIfAbsent(path, []byte(content))
	if err != nil {
		fail(stderr, err)
		return exitValidationError
	}

	var created, existing []templateItem
	if options.CreateTypstTemplates {
		// `theme` defaults to `"classic"` upstream (`new_command.py:35`), and
		// that is the folder name `copy_templates` writes to whether or not
		// the flag was given explicitly.
		theme := orDefault(options.Theme, defaultTheme)
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

	_, _ = fmt.Fprint(stdout, newBanner(path, inputFileCreated, templateLines(created, existing), TerminalFor(stdout)))
	return 0
}

// writeFileIfAbsent is writeTemplatesIfAbsent for a single file: upstream's
// loop skips any item whose path exists, the input file included.
func writeFileIfAbsent(path string, content []byte) (created bool, err error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return false, statErr
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return false, err
	}
	return true, nil
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

// templateLines is the "Also created" / "Not modified" block
// `new_command.py:150-166` appends to the "Get started" panel. **Each section
// gets its own leading blank line**, independently of the other — a `--create-
// typst-templates --create-markdown-templates` run where one folder already
// exists produces both sections, each with one entry.
//
// **Lines, not rows**, because upstream appends them to the same `lines` list
// every other line of the panel is in and joins the lot with `"\n"` before
// handing it over as one renderable. Neither section carries any markup.
func templateLines(created, existing []templateItem) []string {
	var lines []string
	if len(created) > 0 {
		lines = append(lines, "", "Also created:")
		for _, item := range created {
			lines = append(lines, fmt.Sprintf("  ○ %s: ./%s", item.desc, item.path))
		}
	}
	if len(existing) > 0 {
		lines = append(lines, "", "Not modified (already exist):")
		for _, item := range existing {
			lines = append(lines, fmt.Sprintf("  - %s: ./%s", item.desc, item.path))
		}
	}
	if len(created) > 0 || len(existing) > 0 {
		lines = append(lines, "",
			"Templates are for advanced design customization. You can ignore or delete them.")
	}
	return lines
}

// The two defaults typer declares for `new` (`new_command.py:38`, `:49`). The
// port carries an unset flag as the empty string, so they are applied here.
const (
	defaultTheme  = "classic"
	defaultLocale = "english"
)

// Version is the upstream version this port mirrors, and it lives in
// internal/version because it has three user-visible sites, not one: `--version`
// (`cli/app.py:41`), `new`'s greeting (`cli/new_command/print_welcome.py:14`)
// and line 1 of every generated starter CV
// (`schema/sample_generator.py:161-166`). Spec 013 §3.3 behavior 26: a bump that
// misses any of the three is a byte divergence in a golden.
const Version = version.RenderCV

// newBanner is `new`'s stdout: a greeting, a links panel and a next-steps panel.
//
// **`rendercv render …` in the last row is the sanctioned binary-name
// divergence** (`AGENTS.md` §1, spec 012 §1 behavior 4). The instruction has to
// name the binary the user actually has, so this line cannot match the golden
// and must not.
func newBanner(path string, inputFileCreated bool, templatesLines []string, terminal Terminal) string {
	var out strings.Builder
	// **A leading blank line**, which Rich emits before the greeting and which
	// is easy to lose: it is the first byte of the golden, so dropping it shifts
	// the whole comparison by one.
	//
	// **And it wraps.** `rich.print` lays a plain string out at the console
	// width under the default `"fold"` overflow, so at 20 columns the greeting
	// is two lines and not one. Nothing else on this surface is outside a box,
	// which is why the overflow went unseen.
	out.WriteString("\n")
	for _, line := range wrapText(newGreeting(), ConsoleWidth()) {
		out.WriteString(line.RenderSegments(terminal))
		out.WriteString("\n")
	}
	out.WriteString("\n")

	out.WriteString(StyledPanel(PlainText("Useful Links"),
		[]PanelRow{{Body: Markup(strings.Join(usefulLinks(), "\n"))}},
		StyleBrightBlack, terminal))

	// `new_command.py:126-136` picks between the two first lines on whether the
	// input file was among the items actually created. Only the created form
	// carries the check mark, and both carry the `purple` path.
	firstLine := "Your YAML input file already exists: [purple]./" + path + "[/purple]"
	if inputFileCreated {
		firstLine = "[green]✓[/green] Created your YAML input file: [purple]./" + path + "[/purple]"
	}

	lines := []string{
		firstLine,
		"",
		"Next steps:",
		"  1. Edit the YAML input file with your information",
		"  2. Run: [cyan]rendercv-go render " + path + "[/cyan]",
	}
	// **The templates block is appended, not interleaved** — upstream builds
	// the same "Get started" panel one `lines.append` at a time
	// (`new_command.py:150-166`), so its lines always follow the next-steps
	// block rather than replacing any of it.
	lines = append(lines, templatesLines...)
	out.WriteString(StyledPanel(PlainText("Get started"),
		[]PanelRow{{Body: Markup(strings.Join(lines, "\n"))}},
		StyleBrightBlack, terminal))
	return out.String()
}

// newGreeting is the one **bare string** RenderCV hands to `rich.print`
// (`cli/new_command/print_welcome.py:14`), and therefore the one place the
// repr highlighter runs (delta §2.6).
func newGreeting() Text {
	return HighlightRepr(Markup(
		fmt.Sprintf("Welcome to [dodger_blue3]RenderCV v%s[/dodger_blue3]!", Version)))
}

// usefulLinks is `print_welcome.py:15-24`: the four titles padded to fifteen
// columns in `bold cyan`, a plain space, and the URL as an **OSC 8 hyperlink**
// rather than a colour.
//
// The padding is `f"{title + ':':<15}"` and is applied *inside* the markup, so
// it is part of the styled run — the same rule as the progress panel's timing
// field.
func usefulLinks() []string {
	links := []struct{ title, url string }{
		{title: "RenderCV App", url: "https://rendercv.com"},
		{title: "Documentation", url: "https://docs.rendercv.com"},
		{title: "Source code", url: "https://github.com/rendercv/rendercv/"},
		{title: "Bug reports", url: "https://github.com/rendercv/rendercv/issues/"},
	}

	lines := make([]string, 0, len(links))
	for _, link := range links {
		lines = append(lines, fmt.Sprintf("[bold cyan]%s[/bold cyan] [link=%s]%s[/link]",
			pad(link.title+":", linkTitleWidth), link.url, link.url))
	}
	return lines
}

// linkTitleWidth is the column the welcome panel's URLs start in — `:<15`
// (`print_welcome.py:22`).
const linkTitleWidth = 15

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
