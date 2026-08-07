package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// Execute builds the command tree and runs it.
//
// **Only `render` is wired.** `new`, `create-theme` and the Rich-rendered help
// panels are the rest of iteration 12; a command that does not exist yet exits
// the way the placeholder binary did rather than printing a cobra help screen
// that would be wrong in every byte.
func Execute(args []string, stdout, stderr io.Writer) int {
	rest, overrides := Normalize(args)

	options := RenderOptions{Overrides: overrides}
	code := 70

	render := &cobra.Command{
		Use:  "render [input]",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, positional []string) error {
			options.InputPath = positional[0]
			code = Render(options, stdout, stderr)
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

	root := &cobra.Command{Use: "rendercv-go", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(render)
	root.SetArgs(rest)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		return 70
	}
	return code
}
