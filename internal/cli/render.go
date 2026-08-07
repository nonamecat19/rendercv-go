package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/document"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/modelbuilder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
)

// RenderOptions are `render`'s flags, after the pre-pass of args.go has turned
// upstream's single-dash spellings into ordinary ones.
type RenderOptions struct {
	InputPath string

	OutputFolder string
	TypstPath    string
	PDFPath      string
	MarkdownPath string
	HTMLPath     string
	PNGPath      string

	NoTypst    bool
	NoPDF      bool
	NoPNG      bool
	NoMarkdown bool
	NoHTML     bool

	Quiet bool

	// Overrides are the arbitrary dotted `--cv.phone value` pairs of spec §2
	// behavior 9. An unknown key is **not** rejected here: it is set on the
	// document and the model reports it, which is what `err_bad_override_key`
	// measures.
	Overrides map[string]string
}

// Render is the `render` command (spec 012 §2).
//
// **PDF and PNG are iteration 10's**, so this writes the three text artifacts
// and reports the two it cannot produce rather than pretending they exist.
func Render(options RenderOptions, stdout, stderr io.Writer) int {
	raw, err := os.ReadFile(options.InputPath)
	if err != nil {
		// The trailing `!` is upstream's own message text, not this port's
		// punctuation choice, so `ST1005` is suppressed rather than obeyed —
		// obeying it would be a validation-error divergence (axis 4).
		fail(stderr, errMissingFile(options.InputPath))
		return 4
	}

	built, err := modelbuilder.BuildDictionary(string(raw), buildArguments(options))
	if err != nil {
		fail(stderr, err)
		return 4
	}

	context := &valctx.ValidationContext{InputFilePath: options.InputPath}
	model, err := modelbuilder.BuildModel(built, context)
	if err != nil {
		fail(stderr, err)
		return 4
	}

	doc := bridge.Resolve(model, context.Today())
	inputDir := filepath.Dir(options.InputPath)

	pathInput := PathInput{
		Name:         plainName(doc),
		OutputFolder: orDefault(options.OutputFolder, DefaultOutputFolder),
		Placeholders: process.BuildDatePlaceholders(doc.Settings.CurrentDate, process.Catalog{
			MonthNames:         doc.Locale.MonthNames,
			MonthAbbreviations: doc.Locale.MonthAbbreviations,
		}),
	}

	started := time.Now()

	// **The order is upstream's**: Typst, then PDF, then PNG, then Markdown,
	// then HTML — and it is the order the result panel lists them in.
	var rows []PanelRow
	markdown := ""

	if !options.NoTypst {
		out, err := document.Render(doc, templater.FormatTypst, document.Options{InputDir: inputDir})
		if err != nil {
			fail(stderr, err)
			return 4
		}
		path, err := writeArtifact(orDefault(options.TypstPath, DefaultTypstPath), pathInput, out)
		if err != nil {
			fail(stderr, err)
			return 4
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated Typst:", Value: display(path)})
	}

	if !options.NoMarkdown {
		out, err := document.Render(doc, templater.FormatMarkdown, document.Options{InputDir: inputDir})
		if err != nil {
			fail(stderr, err)
			return 4
		}
		markdown = out
		path, err := writeArtifact(orDefault(options.MarkdownPath, DefaultMarkdownPath), pathInput, out)
		if err != nil {
			fail(stderr, err)
			return 4
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated Markdown:", Value: display(path)})
	}

	if !options.NoHTML {
		// **The HTML needs the Markdown's text**, and upstream disables it
		// outright when the Markdown was not generated (`html.py:28-30`) rather
		// than rendering one just for this.
		if markdown == "" {
			return finish(options, rows, stdout)
		}
		out, err := document.RenderHTML(doc, markdown, document.Options{InputDir: inputDir})
		if err != nil {
			fail(stderr, err)
			return 4
		}
		path, err := writeArtifact(orDefault(options.HTMLPath, DefaultHTMLPath), pathInput, out)
		if err != nil {
			fail(stderr, err)
			return 4
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated HTML:", Value: display(path)})
	}

	return finish(options, rows, stdout)
}

// finish prints the result panel unless `--quiet` silenced it.
//
// **`--quiet` produces no stdout at all** — measured on `render_quiet`, whose
// golden stdout is zero bytes, not a panel without the progress lines.
func finish(options RenderOptions, rows []PanelRow, stdout io.Writer) int {
	if options.Quiet {
		return 0
	}
	_, _ = fmt.Fprint(stdout, Panel("Your CV is ready", rows))
	return 0
}

func writeArtifact(template string, input PathInput, content string) (string, error) {
	path, err := ResolvePath(template, input)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// display is how the panel spells a path: relative to the working directory and
// prefixed with `./`, which is what every golden shows.
func display(path string) string {
	if relative, err := filepath.Rel(".", path); err == nil {
		path = relative
	}
	return "./" + filepath.ToSlash(path)
}

func buildArguments(options RenderOptions) modelbuilder.BuildArguments {
	return modelbuilder.BuildArguments{
		OutputFolder:         options.OutputFolder,
		TypstPath:            options.TypstPath,
		PdfPath:              options.PDFPath,
		MarkdownPath:         options.MarkdownPath,
		HtmlPath:             options.HTMLPath,
		PngPath:              options.PNGPath,
		DontGenerateTypst:    options.NoTypst,
		DontGeneratePdf:      options.NoPDF,
		DontGeneratePng:      options.NoPNG,
		DontGenerateMarkdown: options.NoMarkdown,
		DontGenerateHtml:     options.NoHTML,
		Overrides:            options.Overrides,
	}
}

func plainName(doc bridge.Document) string {
	if doc.Model == nil || doc.Model.CvModel == nil || doc.Model.CvModel.Name == nil {
		return ""
	}
	return doc.Model.CvModel.Name.Raw
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// errMissingFile carries upstream's wording verbatim.
func errMissingFile(path string) error {
	return fmt.Errorf("The file %s does not exist!", path) //nolint:staticcheck // upstream's text
}

// fail writes one message to stderr. The write itself cannot be usefully
// reported — the channel for reporting it is the one that just failed.
func fail(stderr io.Writer, err error) {
	_, _ = fmt.Fprintln(stderr, err)
}

// timing is the duration Rich prints beside each artifact.
//
// **Its content is not part of the contract and its shape is.** The conformance
// harness rewrites `\d+(\.\d+)?\s?(ms|s)` to `<duration> `, consuming the
// padding that follows — so a timing that does not match that pattern survives
// into the comparison and fails, and one that does is erased on both sides.
func timing(since time.Time) string {
	elapsed := time.Since(since).Seconds()
	return fmt.Sprintf("%.1fs", elapsed)
}
