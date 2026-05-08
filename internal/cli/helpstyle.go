package cli

// The styles `--help` is written in — spec 012 delta §4.
//
// **`--help` is typer's, not RenderCV's.** It is rendered by
// `typer.rich_utils.rich_format_help` through a console typer builds itself
// (`typer/rich_utils.py:140-158`), with its own theme, its own style constants
// and its own rules for when a terminal is a terminal (§3.5, `terminal.go`).
// Two of the names collide with RenderCV's own and mean something else: the
// usage-error panel's border is plain `red` where RenderCV's `Error` panel is
// `bold red`, and the help panels' border is `dim` where every RenderCV panel is
// `bright_black`.

// typerTheme is the `rich.theme.Theme` typer installs on its console
// (`typer/rich_utils.py:141-152`). It is what turns a highlighter's capture
// group into a style: the group is named `switch`, and the theme says a switch
// is `bold green`.
//
// It is a closed set, like `namedStyles`: a name that is not here is a name
// nobody measured.
var typerTheme = map[string]Style{
	"option":          StyleBoldCyan,    // STYLE_OPTION (`:29`)
	"switch":          StyleBoldGreen,   // STYLE_SWITCH (`:30`)
	"negative_option": StyleBoldMagenta, // STYLE_NEGATIVE_OPTION (`:31`)
	"negative_switch": StyleBoldRed,     // STYLE_NEGATIVE_SWITCH (`:32`)
	"metavar":         StyleBoldYellow,  // STYLE_METAVAR (`:33`)
	"metavar_sep":     StyleDim,         // STYLE_METAVAR_SEPARATOR (`:34`)
	"usage":           StyleYellow,      // STYLE_USAGE (`:35`)
}

// The styles that belong to the page's *structure* rather than to a captured
// string, so `helpprobe` never sees them.
var (
	// styleUsageCommand is `STYLE_USAGE_COMMAND` (`:36`), the style the whole
	// usage `Padding` is printed under (`:552-554`) — text, fill and padding
	// cells alike.
	styleUsageCommand = StyleBold
	// styleHelpPanelBorder is `STYLE_OPTIONS_PANEL_BORDER` (`:45`) and
	// `STYLE_COMMANDS_PANEL_BORDER` (`:55`), which are the same value.
	styleHelpPanelBorder = StyleDim
	// styleRequiredShort is `STYLE_REQUIRED_SHORT` (`:43`), the `*` column.
	styleRequiredShort = StyleRed
	// styleCommandsFirstColumn is `STYLE_COMMANDS_TABLE_FIRST_COLUMN` (`:64`),
	// declared on the column rather than on the cell (`:487-491`) — which is why
	// it covers that column's padding cell too.
	styleCommandsFirstColumn = StyleBoldCyan
)

// helpStyle resolves one style name from the captured model: a theme name
// first, then the literal spellings `ParseStyle` knows, which is where the
// markup in RenderCV's own docstrings lands (`[yellow]`, `[cyan]`).
//
// **An unknown name renders plain rather than panicking.** The names come from
// generated data, so an unknown one is a build problem no user can cause, and
// package `cli` may not panic (`exitcode_test.go`). `TestHelpModelStylesAreKnown`
// is what turns it into a build problem instead of a silently colourless page.
func helpStyle(name string) Style {
	style, _ := helpStyleOf(name)
	return style
}

func helpStyleOf(name string) (Style, bool) {
	if style, ok := typerTheme[name]; ok {
		return style, true
	}
	return ParseStyle(name)
}
