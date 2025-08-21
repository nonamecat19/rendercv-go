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
	rest, overrides := Normalize(args)

	options := RenderOptions{Overrides: overrides}
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

	flags := render.Flags()
	flags.StringVar(&options.OutputFolder, "output-folder", "", "")
	flags.StringVar(&options.TypstPath, "typ", "", "")
	flags.StringVar(&options.PDFPath, "pdf", "", "")
	flags.StringVar(&options.PNGPath, "png", "", "")
	flags.StringVar(&options.MarkdownPath, "md", "", "")
	flags.StringVar(&options.HTMLPath, "html", "", "")
	flags.BoolVar(&options.NoTypst, "notyp", false, "")
	flags.BoolVar(&options.NoPDF, "nopdf", false, "")
	flags.BoolVar(&options.NoPNG, "nopng", false, "")
	flags.BoolVar(&options.NoMarkdown, "nomd", false, "")
	flags.BoolVar(&options.NoHTML, "nohtml", false, "")
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
