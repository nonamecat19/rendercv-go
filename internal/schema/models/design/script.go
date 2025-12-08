package design

import (
	"os"

	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
)

// scriptFileName is D-002's stand-in for the `__init__.py` upstream looks for
// in a custom theme's folder (`design.py:91`).
const scriptFileName = "init.lua"

// ThemeScript is a custom theme's `<theme>/init.lua` after loading: the options
// it declares, and whether the file was there at all.
//
// It mirrors what upstream gets from importing `<theme>/__init__.py` and
// reading the `<Theme>Theme` class off the module (`design.py:90-135`) — the
// theme's declared shape, available **at validation time**, which is the point.
// Upstream instantiates that class with the document's whole `design` block
// (`:135`), so the shape has to exist before any of the block is judged.
type ThemeScript struct {
	// Options is what the script declared, flattened into the plain values
	// `Effective` merges. It is nil when there is no script, and also when
	// there is one that failed to parse, run or agree with the base tree —
	// `Exists` is what tells those apart.
	Options map[string]any
	// Exists reports whether an `init.lua` **file** was found, which is not the
	// same question as whether `Options` is usable.
	//
	// Upstream's no-module fallback builds `ThemeOptionsAreNotProvided`
	// (`design.py:137-142`) and discards everything the document said, so the
	// two cases lead to different documents; `EffectiveWithScript` already
	// depends on the distinction and would throw a user's whole `design` block
	// away on a theme whose script merely broke.
	Exists bool
}

// LoadThemeScript reads `<relativeTo>/<theme>/init.lua` and reduces it to the
// options it declares — the loading step of upstream's `validate_design`
// (`design.py:90-135`), performed where upstream performs it.
//
// **Before this, the script was read only at render time** (`bridge.Resolve`),
// so `Validate` returned unconditionally for every custom theme and nothing
// about a scripted theme's declared shape was ever checked. Moving the load
// here is a control-flow change on its own and reports nothing; what reads the
// returned shape is the value check on a tree-declared key and the unknown-key
// check, each its own unit.
//
// `relativeTo` is the input file's directory, resolved the way
// `PurePath.parent` resolves it (`validate.go`'s `relativeTo`), and the folder
// is joined the way `ValidateCustomThemeFolder` joins it — an uncleaned `..`
// segment must survive both, or validation and the folder checks disagree about
// which directory the theme is in.
//
// **A built-in theme never reads a script**, which upstream gets by only
// entering the custom-theme path when the built-in discriminator fails
// (`design.py:36-50`). `Validate` only calls this on the custom path, so the
// guard is for the benefit of any other caller: reading `classic/init.lua`
// beside a CV silently changed a built-in theme's artifact once already, a
// parity break a verifier measured as `page-size: "a5"` where upstream emits
// `"us-letter"`.
func LoadThemeScript(relativeTo, theme string) ThemeScript {
	if IsBuiltinTheme(theme) {
		return ThemeScript{}
	}

	folder := uncleanedJoin(relativeTo, theme)
	source, err := os.ReadFile(uncleanedJoin(folder, scriptFileName))
	if err != nil {
		// **A missing script is not a failure**: a theme folder with no module
		// is valid upstream and falls back to the base tree
		// (`design.py:136-142`).
		return ThemeScript{}
	}
	// The file is there from here on, whatever it turns out to contain.

	table, err := luatheme.Run(string(source))
	if err != nil {
		return ThemeScript{Exists: true}
	}

	options := luatheme.Options(table)

	// **A script whose shapes conflict with the tree declares nothing.**
	// Dropping it whole is what `bridge.themeScript` has always done with a
	// `ValidateScript` finding, and this must agree with it: a script the
	// renderer will not merge is not a shape the validator may judge a document
	// against. Surfacing the finding is a separate unit's business.
	if errs := ValidateScript(options); len(errs) > 0 {
		return ThemeScript{Exists: true}
	}
	return ThemeScript{Options: options, Exists: true}
}
