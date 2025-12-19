package design

import (
	"errors"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
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
	// Failures is why the script could not be used, one record per finding, and
	// empty when there is no script or the script is fine.
	//
	// **They are ordinary validation records.** Upstream raises inside
	// `validate_design`, so a script failure arrives in the same table as every
	// other problem rather than in a panel of its own — and, going through
	// `errorpipeline.Parse` with the rest, it gets the dictionary and the
	// trailing period from the pipeline instead of by hand.
	Failures []schemaerr.ValidationError
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
	return loadThemeScript(themeScriptPathIn(relativeTo, theme), theme)
}

// loadThemeScript is the load itself, once the caller's own spelling of the
// path has been resolved.
func loadThemeScript(scriptPath, theme string) ThemeScript {
	source, err := os.ReadFile(scriptPath)
	if err != nil {
		// **A missing script is not a failure**: a theme folder with no module
		// is valid upstream and falls back to the base tree
		// (`design.py:136-142`).
		return ThemeScript{}
	}
	// The file is there from here on, whatever it turns out to contain.

	table, err := luatheme.Run(string(source))
	if err != nil {
		return ThemeScript{Exists: true, Failures: []schemaerr.ValidationError{runFailure(theme, err)}}
	}

	options := luatheme.Options(table)

	// **A script whose shapes conflict with the tree declares nothing**, and is
	// reported rather than quietly dropped: a declared default the model rejects
	// is exit 1 with no artifact upstream (`design.py:135`). Dropping it used to
	// mean the theme's own defaults rendered at exit 0 — silently wrong.
	if errs := ValidateScript(options); len(errs) > 0 {
		return ThemeScript{Exists: true, Failures: declarationFailures(theme, errs)}
	}
	return ThemeScript{Options: options, Exists: true}
}

// LoadThemeScriptForInput is LoadThemeScript for a caller holding the input
// file's path rather than the directory it sits in — the renderer's shape,
// where `Validate` has the directory already.
//
// It goes through `ThemeScriptPath` so both callers resolve the folder by the
// one lexical rule. Deriving it a second way is what let validation and
// rendering disagree about which script a document uses.
func LoadThemeScriptForInput(inputPath, theme string) ThemeScript {
	if IsBuiltinTheme(theme) {
		return ThemeScript{}
	}
	return loadThemeScript(ThemeScriptPath(inputPath, theme), theme)
}

// themeScriptPathIn is `<dir>/<theme>/init.lua` by `pathlib`'s rules, the one
// place either entry point builds that path.
func themeScriptPathIn(dir, theme string) string {
	return uncleanedJoin(uncleanedJoin(dir, theme), scriptFileName)
}

// runFailure turns `luatheme.Run`'s error into a record, distinguishing the two
// modes it can fail in.
//
// **The two must stay apart**: a script that could not be parsed or run and a
// script that ran fine but handed back a number are different mistakes, and a
// user cannot act on a message that conflates them. gopher-lua returns a
// `*lua.ApiError` for the first and `Run`'s own error for the second, so the
// distinction is already in the type.
//
// Upstream's counterparts are a `SyntaxError` and a missing `{Theme}Theme`
// class; neither text can be reproduced, because Lua has no `SyntaxError` and a
// Lua declaration is a table with no class to be missing. That is D-013, and it
// is the only part of this failure that diverges.
func runFailure(theme string, err error) schemaerr.ValidationError {
	var apiError *lua.ApiError
	if errors.As(err, &apiError) {
		// **`Object`, not `Error()`.** `ApiError.Error` appends the Lua stack
		// traceback (`state.go:49-54`), and a runtime failure's is five lines of
		// `[G]: in function 'error'` that would be rendered as five bordered rows
		// inside the panel — `Panel` treats a newline as a hard break. The object
		// alone is the one line that names the script's own line number, which is
		// the whole reason this text is worth carrying.
		reason := strings.TrimSpace(apiError.Object.String())
		return scriptRecord("The custom theme "+theme+
			"'s init.lua file could not be run: "+reason, scriptInputElided)
	}
	return scriptRecord("The custom theme "+theme+
		"'s init.lua file did not return a table of theme options", scriptInputElided)
}

// declarationFailures reports what `ValidateScript` found, one record per
// finding in the order it found them — pydantic reports every problem with a
// model's declared defaults, not just the first.
func declarationFailures(theme string, errs []error) []schemaerr.ValidationError {
	records := make([]schemaerr.ValidationError, 0, len(errs))
	for _, err := range errs {
		// **A rejected declared value is upstream's own sentence, unprefixed.**
		// `ScriptValueError` carries the text `validateField` would produce for
		// the same value in a document, which is what
		// `theme_data_model_class(**design)` prints — so this mode is parity, and
		// prefixing it to name the option would be a divergence chosen for
		// helpfulness. Upstream names no option either; its location column says
		// `design` and nothing narrower.
		var valueError *ScriptValueError
		if errors.As(err, &valueError) {
			records = append(records, scriptRecord(valueError.Message, valueError.Input))
			continue
		}
		records = append(records, scriptRecord(
			"The custom theme "+theme+"'s init.lua file declares an option the design tree "+
				"cannot hold: "+err.Error(), scriptInputElided))
	}
	return records
}

// scriptInputElided is what upstream's Input Value column holds for a script
// failure with no single offending value: pydantic's repr of the whole `design`
// dictionary, truncated to `...`. Measured against the vendored binary.
const scriptInputElided = "..."

// scriptRecord builds one record at the location upstream reports these at.
//
// **`design`, for every mode** — upstream raises inside `validate_design`, whose
// records carry the whole design block's location and nothing narrower
// (`design.py:72-133`); measured on all four modes against the vendored binary.
//
// **No trailing period here.** The record is raw, like every other one this
// package produces, and `errorpipeline.Parse`'s step 8 (`appendPeriod`,
// `pydantic_error_handling.py:94-95`) applies it. T4 had to append it by hand
// because the record was synthesized at render time and skipped the pipeline;
// appending it here as well would print two.
func scriptRecord(message, input string) schemaerr.ValidationError {
	return schemaerr.ValidationError{
		SchemaLocation:  []string{"design"},
		YamlSource:      schemaerr.SourceMain,
		Message:         message,
		Input:           input,
		LocationIsFinal: true,
	}
}
