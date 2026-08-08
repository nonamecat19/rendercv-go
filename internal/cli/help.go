package cli

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

// helpJSON is the model behind typer's help pages, captured from the vendored
// typer by `tools/helpprobe`. GENERATED; regenerate with `just helpprobe`.
//
//go:embed helpdata/help.json
var helpJSON []byte

// helpModel is the whole capture: the root group and its three subcommands.
type helpModel struct {
	Root     helpCommand            `json:"root"`
	Commands map[string]helpCommand `json:"commands"`
}

// helpCommand is one command's page.
type helpCommand struct {
	Usage       string           `json:"usage"`
	Description string           `json:"description"`
	Arguments   []helpParam      `json:"arguments"`
	Options     []helpParam      `json:"options"`
	Subcommands []helpSubcommand `json:"subcommands"`
}

// helpParam is one row of an `Arguments` or `Options` panel. The four opt
// fields mirror `rich_utils.py:408-421`; a positional argument's own name lands
// in Short, because the split is on whether the string contains `--`.
type helpParam struct {
	Long           string   `json:"long"`
	Short          string   `json:"short"`
	SecondaryLong  string   `json:"secondary_long"`
	SecondaryShort string   `json:"secondary_short"`
	Metavar        string   `json:"metavar"`
	Help           []string `json:"help"`
	Required       bool     `json:"required"`
}

// helpSubcommand is one row of the root's `Commands` panel.
type helpSubcommand struct {
	Name string `json:"name"`
	Help string `json:"help"`
}

var loadHelpModel = sync.OnceValue(func() helpModel {
	var model helpModel
	// The data is embedded and generated, so a failure here is a build
	// problem rather than anything a user can cause.
	if err := json.Unmarshal(helpJSON, &model); err != nil {
		panic("cli: embedded help model is not valid JSON: " + err.Error())
	}
	return model
})

// HelpPage renders one command's help, or the root's when name is empty.
//
// The page is `rich_format_help` (`typer/rich_utils.py:535-620`): the usage
// line inside a `Padding` of 1, the description inside a `Padding` of
// `(0, 1, 1, 1)`, then the `Arguments`, `Options` and `Commands` panels.
//
// **A `Padding` region is full-width lines of spaces, not blank lines** — rich
// paints the padded block to the console width, so the line above the usage is
// eighty spaces rather than nothing.
func HelpPage(name string) string {
	model := loadHelpModel()
	command, ok := model.Commands[name]
	if name == "" {
		command, ok = model.Root, true
	}
	if !ok {
		return ""
	}

	var out strings.Builder
	blank := strings.Repeat(" ", PanelWidth)
	// The text width inside a one-cell padding on each side.
	inner := PanelWidth - 2

	out.WriteString(blank + "\n")
	for _, line := range wrap(rebindHelpText(command.Usage), inner) {
		out.WriteString(pad(" "+line, PanelWidth) + "\n")
	}
	out.WriteString(blank + "\n")

	if command.Description != "" {
		for _, line := range wrap(rebindHelpText(command.Description), inner) {
			out.WriteString(pad(" "+line, PanelWidth) + "\n")
		}
		out.WriteString(blank + "\n")
	}

	if len(command.Arguments) > 0 {
		out.WriteString(paramPanel("Arguments", command.Arguments))
	}
	if len(command.Options) > 0 {
		out.WriteString(paramPanel("Options", command.Options))
	}
	if len(command.Subcommands) > 0 {
		out.WriteString(commandsPanel(command.Subcommands))
	}

	// Every help golden ends `╯\n\n`.
	out.WriteString("\n")
	return out.String()
}

// paramPanel renders an `Arguments` or `Options` panel
// (`rich_utils.py:348-456`).
func paramPanel(title string, params []helpParam) string {
	// **The required column exists only when some parameter is required**
	// (`:422-427`), which is what makes an `Arguments` panel seven columns wide
	// and an `Options` panel six.
	required := false
	for _, param := range params {
		if param.Required {
			required = true
			break
		}
	}

	columns := optionColumnSet(required)
	rows := make([][]helpCell, 0, len(params))
	for _, param := range params {
		row := make([]helpCell, 0, len(columns))
		if required {
			row = append(row, plain(requiredMark(param.Required)))
		}
		items := make([]string, len(param.Help))
		for i, item := range param.Help {
			items[i] = rebindHelpText(item)
		}
		row = append(row,
			plain(param.Long),
			plain(param.Short),
			plain(param.SecondaryLong),
			plain(param.SecondaryShort),
			plain(param.Metavar),
			helpCell{items: items},
		)
		rows = append(rows, row)
	}

	return panelOfLines(title, helpTable(columns, rows, PanelWidth-4))
}

// optionColumnSet is the six columns `add_row` creates, plus the required one
// when it is needed.
func optionColumnSet(required bool) []helpColumn {
	columns := make([]helpColumn, 0, 7)
	if required {
		columns = append(columns, helpColumn{})
	}
	return append(columns, helpColumn{}, helpColumn{}, helpColumn{}, helpColumn{},
		helpColumn{}, helpColumn{Flexible: true})
}

func requiredMark(required bool) string {
	if required {
		return "*"
	}
	return ""
}

// commandsPanel renders the root's `Commands` panel
// (`rich_utils.py:459-532`). It is the one table that declares its columns: the
// first fixed at the longest command name and never wrapped, the second
// greedy.
func commandsPanel(subcommands []helpSubcommand) string {
	longest := 0
	for _, sub := range subcommands {
		longest = max(longest, len(sub.Name))
	}

	columns := []helpColumn{{Width: longest, NoWrap: true}, {Flexible: true}}
	rows := make([][]helpCell, 0, len(subcommands))
	for _, sub := range subcommands {
		rows = append(rows, []helpCell{plain(sub.Name), plain(rebindHelpText(sub.Help))})
	}

	return panelOfLines("Commands", helpTable(columns, rows, PanelWidth-4))
}

func panelOfLines(title string, lines []string) string {
	rows := make([]PanelRow, len(lines))
	for i, line := range lines {
		rows[i] = PanelRow{Text: line}
	}
	return Panel(title, rows)
}

// rebindHelpText prints the port's own binary name where upstream's help prints
// its own.
//
// **This is the sanctioned divergence reaching the one place it cannot be
// undone.** Every help page quotes commands the reader is meant to run —
// `Example: rendercv render John_Doe_CV.yaml` — and printing `rendercv` there
// would be an instruction that does not work. The conformance harness rebinds
// the token and re-pads the row it shortened (D-009), but a help page wraps its
// prose to the console *before* the harness sees it, and a longer name breaks
// the line somewhere else. Re-padding cannot undo a re-wrap.
//
// So the five `cli_*_help` cases differ from their goldens on exactly the lines
// carrying this token, and on nothing else. `specs/012-cli/help.md` §7 records
// the measurement and the three ways out; choosing between them is a human
// gate.
func rebindHelpText(text string) string {
	return strings.ReplaceAll(text, upstreamBinaryName+" ", portBinaryName+" ")
}

const (
	upstreamBinaryName = "rendercv"
	portBinaryName     = "rendercv-go"
)
