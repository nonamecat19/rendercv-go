package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/document"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/modelbuilder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
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

// exitValidationError is what upstream exits with when a document is rejected —
// `typer.Exit(code=1)` (`cli/error_handler.py:49`), and the value every `err_*`
// golden records.
//
// **The port used to exit 4**, which no golden asked for and which a
// fresh-context audit measured against twelve invalid documents. Exit codes are
// axis 2, so this was a divergence in the contract with nothing recording it.
const exitValidationError = 1

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
		failPanel(stdout, errMissingFile(options.InputPath))
		return exitValidationError
	}

	built, err := modelbuilder.BuildDictionary(string(raw), buildArguments(options))
	if err != nil {
		failPanel(stdout, err)
		return exitValidationError
	}

	context := &valctx.ValidationContext{InputFilePath: options.InputPath}
	model, err := modelbuilder.BuildModel(built, context)
	if err != nil {
		failPanel(stdout, err)
		return exitValidationError
	}

	doc := bridge.Resolve(model, context.Today())
	inputDir := filepath.Dir(options.InputPath)

	// **A custom theme's folder is checked before anything is rendered**
	// (`design.py:72-86`). Upstream reports a validation error and writes
	// nothing; the port used to render happily against a theme folder that did
	// not exist, which a verifier measured. The two messages are upstream's own.
	if theme := themeOf(doc); !design.IsBuiltinTheme(theme) {
		if err := design.ValidateCustomThemeFolder(theme, inputDir); err != nil {
			failPanel(stdout, err)
			return exitValidationError
		}
	}

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
			failPanel(stdout, err)
			return exitValidationError
		}
		path, err := writeArtifact(orDefault(options.TypstPath, DefaultTypstPath), pathInput, out)
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated Typst:", Value: display(path)})
	}

	if !options.NoMarkdown {
		out, err := document.Render(doc, templater.FormatMarkdown, document.Options{InputDir: inputDir})
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
		}
		markdown = out
		path, err := writeArtifact(orDefault(options.MarkdownPath, DefaultMarkdownPath), pathInput, out)
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
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
			failPanel(stdout, err)
			return exitValidationError
		}
		path, err := writeArtifact(orDefault(options.HTMLPath, DefaultHTMLPath), pathInput, out)
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
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

// themeOf reads the resolved theme name, which is the discriminator the folder
// check is keyed on.
func themeOf(doc bridge.Document) string {
	theme, _ := doc.Design["theme"].(string)
	return theme
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

// fail writes an error to stderr. The write itself cannot be usefully reported —
// the channel for reporting it is the one that just failed.
//
// **A validation failure is many records, not one.** `UserValidationError.Error`
// returns only the first message, so every location, input value and subsequent
// record was being discarded: a document with three problems reported one line
// and named no field. An audit measured that against upstream, which prints
// every record with its location.
//
// The *shape* here is not upstream's — that is a Rich table, and rendering it is
// iteration 12's remaining work. What this fixes is the information loss.
//
// **Errors are a Rich panel on stdout, not text on stderr.** Every `err_*`
// golden has an empty `stderr.txt` and a `╭─ Error ─…╮` box on stdout, exit 1.
// Writing to stderr meant those cases could never match no matter how right the
// message was.
func failPanel(stdout io.Writer, err error) {
	var rows []PanelRow

	var validation *schemaerr.UserValidationError
	if errors.As(err, &validation) && len(validation.Errors) > 0 {
		for _, record := range validation.Errors {
			location := strings.Join(record.SchemaLocation, ".")
			if location == "" {
				rows = append(rows, PanelRow{Text: record.Message})
				continue
			}
			rows = append(rows, PanelRow{Text: location + ": " + record.Message})
		}
	} else {
		rows = append(rows, PanelRow{Text: err.Error()})
	}

	_, _ = fmt.Fprint(stdout, Panel("Error", rows))
}

func fail(stderr io.Writer, err error) {
	var validation *schemaerr.UserValidationError
	if errors.As(err, &validation) && len(validation.Errors) > 0 {
		for _, record := range validation.Errors {
			location := strings.Join(record.SchemaLocation, ".")
			if location == "" {
				_, _ = fmt.Fprintln(stderr, record.Message)
				continue
			}
			_, _ = fmt.Fprintf(stderr, "%s: %s\n", location, record.Message)
		}
		return
	}
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
