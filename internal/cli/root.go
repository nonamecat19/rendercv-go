package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Execute builds the command tree and runs it.
//
// **Only `render` is wired.** `new`, `create-theme` and the Rich-rendered help
// panels are the rest of iteration 12; a command that does not exist yet exits
// the way the placeholder binary did rather than printing a cobra help screen
// that would be wrong in every byte.
func Execute(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, runners{render: Render, newCV: New})
}

// runners are the two command bodies, injected rather than called directly so a
// test can observe what the parser produced without doing the work. Parsing is
// parity axis 2 on its own — upstream declares seventeen `render` options and
// the corpus exercises ten of them, all by their short spelling — so the
// argument vector needs a gate that does not go through a real render.
type runners struct {
	render func(RenderOptions, io.Writer, io.Writer) int
	newCV  func(NewOptions, io.Writer, io.Writer) int
}

func execute(args []string, stdout, stderr io.Writer, run runners) int {
	rest, extras := Normalize(args)

	options := RenderOptions{Extras: extras}
	code := 70

	render := &cobra.Command{
		Use:  "render [input]",
		Args: exactlyOne("[OPTIONS] INPUT_FILE_NAME", "INPUT_FILE_NAME"),
		RunE: func(_ *cobra.Command, positional []string) error {
			options.InputPath = positional[0]
			code = run.render(options, stdout, stderr)
			return nil
		},
	}

	// The names are upstream's long forms (`render_command.py:33-188`); the
	// short spellings every corpus case uses are rewritten onto them by
	// args.go, because they are whole words after one dash and pflag has no
	// such concept.
	flags := render.Flags()
	flags.StringVar(&options.OutputFolder, "output-folder", "", "")
	flags.StringVar(&options.DesignPath, "design", "", "")
	flags.StringVar(&options.LocalePath, "locale-catalog", "", "")
	flags.StringVar(&options.SettingsPath, "settings", "", "")
	flags.StringVar(&options.TypstPath, "typst-path", "", "")
	flags.StringVar(&options.PDFPath, "pdf-path", "", "")
	flags.StringVar(&options.PNGPath, "png-path", "", "")
	flags.StringVar(&options.MarkdownPath, "markdown-path", "", "")
	flags.StringVar(&options.HTMLPath, "html-path", "", "")
	flags.BoolVar(&options.NoTypst, "dont-generate-typst", false, "")
	flags.BoolVar(&options.NoPDF, "dont-generate-pdf", false, "")
	flags.BoolVar(&options.NoPNG, "dont-generate-png", false, "")
	flags.BoolVar(&options.NoMarkdown, "dont-generate-markdown", false, "")
	flags.BoolVar(&options.NoHTML, "dont-generate-html", false, "")
	flags.BoolVar(&options.Watch, "watch", false, "")
	// **Declared so it parses, and deliberately discarded** (G-2). Upstream's
	// own parameter is bound to `_`; it exists to give the help panel a row for
	// the dotted-override mechanism. Without it the token becomes an override
	// key and the model rejects a document upstream renders.
	var yamlLocation string
	flags.StringVar(&yamlLocation, "YAMLLOCATION", "", "")
	flags.BoolVar(&options.Quiet, "quiet", false, "")

	newOptions := NewOptions{}
	newCmd := &cobra.Command{
		Use:  "new [name]",
		Args: atLeastOne("[OPTIONS] FULL_NAME", "FULL_NAME"),
		RunE: func(_ *cobra.Command, positional []string) error {
			// Typer joins the positional words, so `new John Doe` — unquoted —
			// is the same as `new "John Doe"`. Every corpus case writes it
			// unquoted.
			newOptions.Name = strings.Join(positional, " ")
			code = run.newCV(newOptions, stdout, stderr)
			return nil
		},
	}
	newFlags := newCmd.Flags()
	newFlags.StringVar(&newOptions.Theme, "theme", "", "")
	newFlags.StringVar(&newOptions.Locale, "locale", "", "")
	newFlags.BoolVar(&newOptions.CreateTypstTemplates, "create-typst-templates", false, "")
	newFlags.BoolVar(&newOptions.CreateMarkdownTemplates, "create-markdown-templates", false, "")

	// **`--version` prints upstream's version, not the port's**, and without the
	// binary name: the golden is the single line `RenderCV v2.8`. It is the one
	// output in the whole CLI that carries no `rendercv` token at all, which is
	// why it is the only help-family case reachable before the binary-name
	// question of `STATE.md` is answered.
	version := false

	root := &cobra.Command{
		Use:           "rendercv-go",
		SilenceUsage:  true,
		SilenceErrors: true,
		// A first token that names no subcommand is click's
		// `No such command`, not cobra's own wording, and not a run of the
		// root with leftover arguments.
		Args: func(cmd *cobra.Command, positional []string) error {
			if len(positional) == 0 {
				return nil
			}
			return &usageError{
				usage:   cmd.CommandPath() + " [OPTIONS] COMMAND [ARGS]...",
				command: cmd.CommandPath(),
				message: fmt.Sprintf("No such command '%s'.", positional[0]),
			}
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if version {
				_, _ = fmt.Fprintf(stdout, "RenderCV v%s\n", Version)
				code = 0
				return nil
			}
			// **No arguments at all is the help page, and exit 0**
			// (`cli/app.py:41-44`). The port used to exit 70 in silence.
			_, _ = fmt.Fprint(stdout, HelpPage(""))
			code = 0
			return nil
		},
	}
	root.Flags().BoolVarP(&version, "version", "v", false, "")

	// Typer renders its own help, and cobra's bears no resemblance to it. The
	// function is inherited by every subcommand, so `render -h` reaches it too.
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		name := ""
		if cmd != cmd.Root() {
			name = cmd.Name()
		}
		_, _ = fmt.Fprint(stdout, HelpPage(name))
		code = 0
	})
	root.AddCommand(render)
	root.AddCommand(newCmd)
	root.SetArgs(rest)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		var usage *usageError
		if !errors.As(err, &usage) {
			// **The original vector, not the rewritten one.** Click quotes
			// the spelling the user typed — `Option '-o' requires an
			// argument.` — and args.go has already turned `-o` into
			// `--output-folder` by the time cobra fails on it.
			usage = flagUsageError(root, args, rest, err)
		}
		if usage != nil {
			writeUsageError(stderr, usage)
			return exitUsageError
		}
		return 70
	}
	return code
}

// flagUsageError translates pflag's parse failures into click's, so a malformed
// option reports what upstream reports instead of exiting 70 in silence
// (G-3, G-4).
//
// **Only two shapes reach here, and they print differently.** A missing value
// gets an `Error` panel and **no usage line at all**; an unknown option gets the
// usage line as well. Both were measured against the vendored CLI — the
// asymmetry is click's, not a simplification.
func flagUsageError(root *cobra.Command, typed, rest []string, err error) *usageError {
	command := commandFor(root, rest)

	if name, ok := strings.CutPrefix(err.Error(), "flag needs an argument: "); ok {
		// pflag spells it `--design` or `-d, --design`; click names the
		// spelling the user typed.
		return &usageError{
			command: command.CommandPath(),
			message: fmt.Sprintf("Option '%s' requires an argument.", typedSpelling(name, typed)),
		}
	}
	if name, ok := unknownFlag(err.Error()); ok {
		return &usageError{
			usage:   command.CommandPath() + " " + usageSuffix(command),
			command: command.CommandPath(),
			message: fmt.Sprintf("No such option: %s", name),
		}
	}
	return nil
}

// unknownFlag matches pflag's two spellings of the same failure.
func unknownFlag(text string) (string, bool) {
	for _, prefix := range []string{"unknown flag: ", "unknown shorthand flag: "} {
		if rest, ok := strings.CutPrefix(text, prefix); ok {
			// The shorthand form reads `'d' in -d`; take the spelling as typed.
			if _, after, found := strings.Cut(rest, " in "); found {
				return after, true
			}
			return rest, true
		}
	}
	return "", false
}

// typedSpelling recovers the option as the user wrote it, because click quotes
// that and pflag reports its canonical name.
func typedSpelling(canonical string, args []string) string {
	canonical = strings.TrimPrefix(canonical, "--")
	for _, arg := range args {
		trimmed := strings.TrimLeft(arg, "-")
		if trimmed == canonical || renderShortFlags[trimmed] == canonical {
			return arg
		}
	}
	return "--" + canonical
}

// commandFor is the subcommand the vector names, or the root.
func commandFor(root *cobra.Command, args []string) *cobra.Command {
	found, _, err := root.Find(args)
	if err != nil || found == nil {
		return root
	}
	return found
}

// usageSuffix is the argument spelling click prints after the command path.
func usageSuffix(command *cobra.Command) string {
	switch command.Name() {
	case "render":
		return "[OPTIONS] INPUT_FILE_NAME"
	case "new":
		return "[OPTIONS] FULL_NAME"
	case "create-theme":
		return "[OPTIONS] THEME_NAME"
	default:
		return "[OPTIONS] COMMAND [ARGS]..."
	}
}

// usageError is click's `UsageError` — a malformed invocation, as opposed to a
// document the model rejected.
//
// **It prints in three parts and none of them is the panel a RenderCVUserError
// gets**: the usage line, the `Try ... for help.` line, and only then the same
// `Error` box. All three go to stderr, where every other error this CLI prints
// goes to stdout.
type usageError struct {
	usage   string
	command string
	message string
}

func (e *usageError) Error() string { return e.message }

func writeUsageError(stderr io.Writer, err *usageError) {
	// **A missing option value prints the panel alone** — no usage line and no
	// `Try …` line (G-3). Every other usage error prints all three. The
	// asymmetry is click's and was measured, not inferred.
	if err.usage != "" {
		_, _ = fmt.Fprintf(stderr, "Usage: %s\n", err.usage)
		_, _ = fmt.Fprintf(stderr, "Try '%s -h' for help.\n", err.command)
	}
	_, _ = fmt.Fprint(stderr, Panel("Error", []PanelRow{{Text: err.message}}))
}

// exactlyOne and atLeastOne are cobra argument validators that report click's
// missing-argument text rather than cobra's `accepts 1 arg(s), received 0`.
//
// The two differ because `new` joins its positional words — typer's
// `full_name` takes the rest of the line, so `new John Doe` unquoted is the
// same invocation as `new "John Doe"`, which every corpus case relies on.
func exactlyOne(usage, placeholder string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, positional []string) error {
		if len(positional) != 1 {
			return missingArgument(cmd, usage, placeholder)
		}
		return nil
	}
}

func atLeastOne(usage, placeholder string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, positional []string) error {
		if len(positional) == 0 {
			return missingArgument(cmd, usage, placeholder)
		}
		return nil
	}
}

func missingArgument(cmd *cobra.Command, usage, placeholder string) error {
	return &usageError{
		usage:   cmd.CommandPath() + " " + usage,
		command: cmd.CommandPath(),
		message: fmt.Sprintf("Missing argument '%s'.", placeholder),
	}
}

// exitUsageError is click's `UsageError.exit_code` — the code every malformed
// invocation carries, and the one shape of failure that is neither a validation
// error (1) nor a success (0).
//
// **The port returned 70 for all of them**, which is this function's initial
// value and therefore indistinguishable from an internal failure.
const exitUsageError = 2
