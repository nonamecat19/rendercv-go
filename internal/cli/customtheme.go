package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
)

// customThemeNamePattern is `custom_theme_name_pattern`
// (`schema/models/design/design.py:17`).
var customThemeNamePattern = regexp.MustCompile(`^[a-z0-9]+$`)

// CreateThemeOptions is `create-theme`'s one argument (spec 012 §3).
type CreateThemeOptions struct {
	ThemeName string
}

// CreateTheme is the `create-theme` command
// (`create_theme_command.py`).
//
// **Two of its fourteen files cannot be upstream's bytes — D-008, approved.**
// `__init__.py` is Python the port cannot execute (D-002), so it writes
// `init.lua` instead; the `.j2.typ` files are the pongo2 transform this
// binary's own loader reads (D-005), not upstream's Jinja source.
func CreateTheme(options CreateThemeOptions, stdout, stderr io.Writer) int {
	name := options.ThemeName

	// **The existing-folder guard comes first**, because upstream runs it
	// first: `new_theme_folder.exists()` raises at `create_theme_command.py:34`
	// and the name pattern is not looked at until `create_init_file_for_theme`
	// at `:39`. The port had the two the other way round and answered
	// `create-theme Bad`, in a directory already holding `Bad`, with the
	// name-pattern message where upstream answers `The theme folder "Bad"
	// already exists!`.
	folder := name
	if _, err := os.Stat(folder); err == nil {
		//nolint:staticcheck // upstream's text
		failPanel(stdout, fmt.Errorf("The theme folder %q already exists!", name))
		return exitValidationError
	} else if !errors.Is(err, os.ErrNotExist) {
		fail(stderr, err)
		return exitValidationError
	}

	// **The name check stays ahead of the copy, which upstream's is not.**
	// Upstream copies thirteen files and only then validates, so an invalid
	// name leaves a partial theme on disk — and `create-theme ../escaped`
	// leaves it outside the working directory, measured. Matching that means
	// writing a template tree to a path this binary has already judged invalid,
	// so it is recorded in specs/divergences.md rather than reproduced here.
	if !customThemeNamePattern.MatchString(name) {
		//nolint:staticcheck // upstream's text
		failPanel(stdout, fmt.Errorf(
			"The custom theme name should only contain lowercase letters and digits."+
				" The provided value is `%s`.", name))
		return exitValidationError
	}

	if err := copyTypstTemplates(folder); err != nil {
		fail(stderr, err)
		return exitValidationError
	}
	if err := writeThemeInitLua(folder, name); err != nil {
		fail(stderr, err)
		return exitValidationError
	}

	_, _ = fmt.Fprint(stdout, createThemePanel(name, TerminalFor(stdout)))
	return 0
}

// copyTypstTemplates copies the port's own Typst template tree — the pongo2
// transform embedded at `templater.Builtin` — into dest, the same thirteen
// files `copy_templates("typst", …)` copies upstream (four top-level
// fragments and the nine entry templates).
func copyTypstTemplates(dest string) error {
	return copyBuiltinTemplates("typst", dest)
}

// copyMarkdownTemplates is copyTypstTemplates' companion for the Markdown
// template set — `copy_templates("markdown", …)`'s twelve files (three
// top-level fragments and the nine entry templates; Markdown has no
// `Preamble`).
func copyMarkdownTemplates(dest string) error {
	return copyBuiltinTemplates("markdown", dest)
}

// copyBuiltinTemplates copies one of `templater.Builtin`'s template sets into
// dest, mirroring `copy_templates`'s directory shape.
func copyBuiltinTemplates(kind, dest string) error {
	root := templater.BuiltinTemplates()
	return fs.WalkDir(root, kind, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(kind, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}

// writeThemeInitLua writes the Lua declaration D-002 scripts a custom theme
// with, in place of upstream's `__init__.py` (a `ClassicTheme` pydantic model
// templated by string substitution — `create_init_file_for_theme.py`). A Lua
// script has no such class to derive from: `luatheme.Options` reads whatever
// table the script returns, keyed the same way `design:` fields are, so the
// starter is a documented empty table rather than a restatement of every
// classic-theme default.
func writeThemeInitLua(dest, themeName string) error {
	content := fmt.Sprintf(themeInitLuaTemplate, themeName)
	return os.WriteFile(filepath.Join(dest, "init.lua"), []byte(content), 0o644)
}

const themeInitLuaTemplate = `-- Custom design options for the %q theme.
--
-- Return a table of overrides here. Each key mirrors a field under the
-- document's "design" block, and the default value given here decides its
-- type: a string default declares a string option, a number a number, and so
-- on. A nested table declares a nested block, the same way "colors" and
-- "page" work in the built-in themes.
--
-- Example:
--   return {
--     page = { size = "a4" },
--     colors = { name = "rgb(0, 79, 144)" },
--   }

return {}
`

// createThemePanel is the "Theme created" panel
// (`create_theme_command.py:41-64`), with `__init__.py` renamed to `init.lua`
// throughout — D-008's user-visible half.
//
// **The markup is upstream's, tag for tag, including the three tags it never
// closes**, because they are what the colours on a terminal actually are. The
// `[purple]` opened on the `1. Modify` line is closed by the `[/purple]` on the
// *next* line — `pop_style` takes the nearest match, not the innermost — so the
// outer one keeps running to the end of the message, and the two `[cyan]` tags
// override its colour over the last two lines without ending it. Measured:
// `2. Edit …` comes out as three separately opened `ESC[38;5;129m` runs, and
// the last two lines as one `ESC[36m` run each.
func createThemePanel(themeName string, terminal Terminal) string {
	message := "[green]✓[/green] Created your custom theme: [purple]./" + themeName + "[/purple]\n" +
		"\n" +
		"What you can do with this theme:\n" +
		"1. Modify the Typst templates in [purple]./" + themeName + "/\n" +
		"2. Edit [purple]./" + themeName + "/init.lua[/purple] to:\n" +
		"    - Add your own design options to use in the YAML input file\n" +
		"    - Change the default values of existing options\n" +
		"    - Or simply delete it if you only want to customize templates\n" +
		"\n" +
		"To use your theme, set in your YAML input file:\n" +
		"[cyan]  design:\n" +
		"[cyan]    theme: " + themeName

	return StyledPanel(PlainText("Theme created"), []PanelRow{{Body: Markup(message)}},
		StyleBrightBlack, terminal)
}
