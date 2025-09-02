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

	if !customThemeNamePattern.MatchString(name) {
		//nolint:staticcheck // upstream's text
		failPanel(stdout, fmt.Errorf(
			"The custom theme name should only contain lowercase letters and digits."+
				" The provided value is `%s`.", name))
		return exitValidationError
	}

	folder := name
	if _, err := os.Stat(folder); err == nil {
		//nolint:staticcheck // upstream's text
		failPanel(stdout, fmt.Errorf("The theme folder %q already exists!", name))
		return exitValidationError
	} else if !errors.Is(err, os.ErrNotExist) {
		fail(stderr, err)
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

	_, _ = fmt.Fprint(stdout, createThemePanel(name))
	return 0
}

// copyTypstTemplates copies the port's own Typst template tree — the pongo2
// transform embedded at `templater.Builtin` — into dest, the same thirteen
// files `copy_templates("typst", …)` copies upstream (four top-level
// fragments and the nine entry templates).
func copyTypstTemplates(dest string) error {
	root := templater.BuiltinTemplates()
	return fs.WalkDir(root, "typst", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("typst", path)
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
// (`create_theme_command.py:41-58`), with `__init__.py` renamed to `init.lua`
// throughout — D-008's user-visible half.
func createThemePanel(themeName string) string {
	lines := []string{
		"✓ Created your custom theme: ./" + themeName,
		"",
		"What you can do with this theme:",
		"1. Modify the Typst templates in ./" + themeName + "/",
		"2. Edit ./" + themeName + "/init.lua to:",
		"    - Add your own design options to use in the YAML input file",
		"    - Change the default values of existing options",
		"    - Or simply delete it if you only want to customize templates",
		"",
		"To use your theme, set in your YAML input file:",
		"  design:",
		"    theme: " + themeName,
	}

	rows := make([]PanelRow, len(lines))
	for i, line := range lines {
		rows[i] = PanelRow{Text: line, IsText: line == ""}
	}
	return Panel("Theme created", rows)
}
