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
	Usage       helpString       `json:"usage"`
	Description helpString       `json:"description"`
	Arguments   []helpParam      `json:"arguments"`
	Options     []helpParam      `json:"options"`
	Subcommands []helpSubcommand `json:"subcommands"`
}

// helpString is one captured renderable: its plain text, the style its own
// rich `Text` carries, and the spans typer's markup and highlighters left on it.
//
// **The spans arrive rather than being re-derived.** typer restyles text
// *inside* prose with three `RegexHighlighter`s (`typer/rich_utils.py:106-132`),
// and where they open a run is visible in the bytes: `--help` in a sentence
// comes out as two runs, because the option pattern captures the leading `-`
// separately from the switch pattern (spec 012 delta §4). Porting three
// lookahead patterns to a package with no lookahead would be a second place for
// that to go subtly wrong; `tools/helpprobe` asks the vendored typer instead.
type helpString struct {
	Text  string     `json:"text"`
	Style string     `json:"style"`
	Spans []helpSpan `json:"spans"`
}

// helpSpan is one style over a half-open range of the text, in codepoints —
// which is what Go counts in runes.
type helpSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Style string `json:"style"`
}

// item is the captured string as the port's own styled text, with the binary
// name rebound (D-009) and its spans moved to follow it.
func (s helpString) item() helpItem {
	text := Text{Plain: s.Text}
	for _, span := range s.Spans {
		text.Spans = append(text.Spans, Span{
			Start: span.Start,
			End:   span.End,
			Style: helpStyle(span.Style),
		})
	}
	return helpItem{Text: rebindHelpText(text), Base: helpStyle(s.Style)}
}

// helpParam is one row of an `Arguments` or `Options` panel. The four opt
// fields mirror `rich_utils.py:408-421`; a positional argument's own name lands
// in Short, because the split is on whether the string contains `--`.
type helpParam struct {
	Long           helpString   `json:"long"`
	Short          helpString   `json:"short"`
	SecondaryLong  helpString   `json:"secondary_long"`
	SecondaryShort helpString   `json:"secondary_short"`
	Metavar        helpString   `json:"metavar"`
	Help           []helpString `json:"help"`
	Required       bool         `json:"required"`
}

// helpSubcommand is one row of the root's `Commands` panel.
type helpSubcommand struct {
	Name string     `json:"name"`
	Help helpString `json:"help"`
}

// loadHelpModel decodes the embedded capture once.
//
// **A decode failure yields the zero model rather than a panic.** The data is
// embedded and generated, so a failure is a build problem no user can cause —
// and a build problem belongs in a build's tests, which is where
// `TestEmbeddedHelpModelParses` and every `cli_*_help` golden comparison put it.
// Panicking would move it to the user's terminal instead, ending the process at
// exit 2 with a goroutine dump: neither of the two shapes upstream produces for
// a failure of its own (exit 1 with a traceback, exit 2 with a usage message).
// The zero model renders an empty page, which the exit-code contract can
// represent.
var loadHelpModel = sync.OnceValue(func() helpModel {
	var model helpModel
	if err := json.Unmarshal(helpJSON, &model); err != nil {
		return helpModel{}
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
func HelpPage(name string, terminal Terminal) string {
	model := loadHelpModel()
	command, ok := model.Commands[name]
	if name == "" {
		command, ok = model.Root, true
	}
	if !ok {
		return ""
	}

	var out strings.Builder
	for _, line := range helpPageLines(command) {
		out.WriteString(renderSegments(line, terminal))
		out.WriteString("\n")
	}
	// Every help golden ends `╯\n\n`.
	out.WriteString("\n")
	return out.String()
}

// helpPageLines is the page as runs, before any terminal has been consulted.
func helpPageLines(command helpCommand) [][]Segment {
	width := ConsoleWidth()

	// **The usage block is printed under `bold`** — `console.print(Padding(…, 1),
	// style=STYLE_USAGE_COMMAND)` (`rich_utils.py:552-554`) — and the style
	// covers the padding as well as the text, so the blank line above the usage
	// is eighty *bold* spaces. The description block, printed with no style at
	// all (`:559-570`), is eighty plain ones.
	lines := paddingBlock(command.Usage.item(), width, 1, styleUsageCommand)
	if command.Description.Text != "" {
		lines = append(lines, paddingBlock(command.Description.item(), width, 0, Style{})...)
	}

	if len(command.Arguments) > 0 {
		lines = append(lines, paramPanel("Arguments", command.Arguments)...)
	}
	if len(command.Options) > 0 {
		lines = append(lines, paramPanel("Options", command.Options)...)
	}
	if len(command.Subcommands) > 0 {
		lines = append(lines, commandsPanel(command.Subcommands)...)
	}
	return lines
}

// paddingBlock is one `rich.padding.Padding` region: `top` blank lines, the text
// wrapped inside one cell of padding on each side, then one blank line.
//
// **A `Padding` region is full-width lines of spaces, not blank lines** — rich
// paints the padded block to the console width. And it is four runs on a styled
// line, not one: the left pad, the text, the fill `adjust_line_length` appends,
// and the right pad are separate segments, which is why a `bold` usage line
// closes and reopens around its own padding.
func paddingBlock(item helpItem, width, top int, style Style) [][]Segment {
	blank := []Segment{{Text: strings.Repeat(" ", width), Style: style}}

	var lines [][]Segment
	for range top {
		lines = append(lines, blank)
	}
	for _, line := range wrapText(item.Text, width-2) {
		runes := len([]rune(line.Plain))
		segments := []Segment{{Text: " ", Style: style}}
		segments = append(segments, line.StylizeBefore(0, runes, item.Base).StylizeBefore(0, runes, style).Segments()...)
		if fill := width - 2 - cellLen(line.Plain); fill > 0 {
			segments = append(segments, Segment{Text: strings.Repeat(" ", fill), Style: style})
		}
		lines = append(lines, append(segments, Segment{Text: " ", Style: style}))
	}
	return append(lines, blank)
}

// paramPanel renders an `Arguments` or `Options` panel
// (`rich_utils.py:348-456`).
func paramPanel(title string, params []helpParam) [][]Segment {
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
			row = append(row, requiredCell(param.Required))
		}
		items := make([]helpItem, len(param.Help))
		for i, item := range param.Help {
			items[i] = item.item()
		}
		row = append(row,
			helpCell{items: []helpItem{param.Long.item()}},
			helpCell{items: []helpItem{param.Short.item()}},
			helpCell{items: []helpItem{param.SecondaryLong.item()}},
			helpCell{items: []helpItem{param.SecondaryShort.item()}},
			helpCell{items: []helpItem{param.Metavar.item()}},
			helpCell{items: items},
		)
		rows = append(rows, row)
	}

	return panelOfLines(title, helpTableLines(columns, rows, ConsoleWidth()-4))
}

// optionColumnSet is the six columns `add_row` creates, plus the required one
// when it is needed.
func optionColumnSet(required bool) []helpColumn {
	columns := make([]helpColumn, 0, 7)
	if required {
		columns = append(columns, helpColumn{})
	}
	// The fifth is the metavar, the one column typer gives its own overflow
	// (`rich_utils.py:376`), and the sixth is the help — the one cell in either
	// table that is a `Columns` rather than a `Text` (`:318`).
	return append(columns, helpColumn{}, helpColumn{}, helpColumn{}, helpColumn{},
		helpColumn{Fold: true}, helpColumn{Flexible: true, Columns: true})
}

// requiredCell is the asterisk column: `Text(REQUIRED_SHORT_STRING,
// style=STYLE_REQUIRED_SHORT)` (`rich_utils.py:402-404`), which is `red` — not
// the `dim red` of the `[required]` note that shares the row.
func requiredCell(required bool) helpCell {
	if !required {
		return helpCell{items: []helpItem{{}}}
	}
	return helpCell{items: []helpItem{{Text: PlainText("*"), Base: styleRequiredShort}}}
}

// commandsPanel renders the root's `Commands` panel
// (`rich_utils.py:459-532`). It is the one table that declares its columns: the
// first fixed at the longest command name and never wrapped, the second
// greedy.
func commandsPanel(subcommands []helpSubcommand) [][]Segment {
	// The fixed first column is as wide as the longest command name **in
	// cells** — `len()` here was bytes, which over-reserves for any name
	// outside ASCII.
	longest := 0
	for _, sub := range subcommands {
		longest = max(longest, cellLen(sub.Name))
	}

	// The first column is the one column in either table with a style of its
	// own, and it is the column's rather than the cell's — so it covers the
	// padding cell beside the name as well.
	columns := []helpColumn{
		{Width: longest, NoWrap: true, Style: styleCommandsFirstColumn},
		{Flexible: true},
	}
	rows := make([][]helpCell, 0, len(subcommands))
	for _, sub := range subcommands {
		rows = append(rows, []helpCell{plain(sub.Name), {items: []helpItem{sub.Help.item()}}})
	}

	return panelOfLines("Commands", helpTableLines(columns, rows, ConsoleWidth()-4))
}

// panelOfLines boxes an already-laid-out table. The border is `dim` on both
// help panels (`STYLE_OPTIONS_PANEL_BORDER` and `STYLE_COMMANDS_PANEL_BORDER`,
// `typer/rich_utils.py:45`, `:55`) and the title carries no style of its own, so
// the band comes out as one run inside the border's — measured,
// `ESC[2m╭─ESC[0mESC[2m Arguments ESC[0m…`.
func panelOfLines(title string, rows [][]Segment) [][]Segment {
	return panelFrame(PlainText(title), rows, ConsoleWidth(), styleHelpPanelBorder)
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
//
// **The spans move with the text.** `[yellow]rendercv render
// John_Doe_CV.yaml[/yellow]` has to stay yellow to its last character, and the
// `--help` after it has to keep its own two runs three columns further along.
func rebindHelpText(text Text) Text {
	return text.ReplaceAll(upstreamBinaryName+" ", portBinaryName+" ")
}

const (
	upstreamBinaryName = "rendercv"
	portBinaryName     = "rendercv-go"
)
