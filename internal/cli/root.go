package cli

import (
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
		Args: cobra.ExactArgs(1),
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
	flags.BoolVar(&options.Quiet, "quiet", false, "")

	newOptions := NewOptions{}
	newCmd := &cobra.Command{
		Use:  "new [name]",
		Args: cobra.MinimumNArgs(1),
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
		RunE: func(_ *cobra.Command, _ []string) error {
			if version {
				_, _ = fmt.Fprintf(stdout, "RenderCV v%s\n", Version)
				code = 0
			}
			return nil
		},
	}
	root.Flags().BoolVarP(&version, "version", "v", false, "")
	root.AddCommand(render)
	root.AddCommand(newCmd)
	root.SetArgs(rest)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		return 70
	}
	return code
}

// exitUsageError is click's `UsageError.exit_code` — the code every malformed
// invocation carries, and the one shape of failure that is neither a validation
// error (1) nor a success (0).
//
// **The port returned 70 for all of them**, which is this function's initial
// value and therefore indistinguishable from an internal failure.
const exitUsageError = 2
