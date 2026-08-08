package cli

import (
	"context"
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
	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc"
	"github.com/nonamecat19/rendercv-go/internal/schema/modelbuilder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
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

	// DesignPath, LocalePath and SettingsPath are the three overlay files of
	// spec §2 behavior 7 — `--design`, `--locale-catalog` and `--settings`.
	// `modelbuilder.BuildArguments` has carried their contents since iteration
	// 2; no corpus case passes one, which is how `render` came to pass every
	// case in the suite without declaring them.
	DesignPath   string
	LocalePath   string
	SettingsPath string

	NoTypst    bool
	NoPDF      bool
	NoPNG      bool
	NoMarkdown bool
	NoHTML     bool

	Quiet bool

	// Watch is `--watch` / `-w`. It is declared so the argument vector parses
	// the way upstream's does; the watcher itself is spec §6.2's, iteration
	// 13's work.
	Watch bool

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

	arguments, err := buildArguments(options)
	if err != nil {
		failPanel(stdout, err)
		return exitValidationError
	}

	built, err := modelbuilder.BuildDictionary(string(raw), arguments)
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

	// The custom-theme folder checks used to run here, as a user error. They are
	// upstream's *validation* records (`design.py:72-86`), so they moved into
	// `design.Validate` and arrive through `BuildModel` above with every other
	// record — which is what `err_unknown_theme` compares.

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
	typstPath := ""

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
		typstPath = path
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated Typst:", Value: display(path)})
	}

	// **PDF and PNG both depend on the `.typ` file existing on disk**, which is
	// why upstream returns early from both when `typst_path is None`
	// (`pdf_png.py:33,63`): `--notyp` disables them by omission, not by a flag
	// of their own.
	if !options.NoPDF && typstPath != "" {
		path, err := renderPDF(doc, typstPath, orDefault(options.PDFPath, DefaultPDFPath), pathInput, inputDir)
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(started), Label: "Generated PDF:", Value: display(path)})
	}

	if !options.NoPNG && typstPath != "" {
		paths, err := renderPNGs(doc, typstPath, orDefault(options.PNGPath, DefaultPNGPath), pathInput, inputDir)
		if err != nil {
			failPanel(stdout, err)
			return exitValidationError
		}
		if len(paths) > 0 {
			// **The label is pluralised only for a multi-page document**
			// (`run_rendercv.py:58-64`): `timed_step` appends the `s` when the
			// step returned more than one path. A one-page CV says
			// "Generated PNG:".
			label := "Generated PNG:"
			if len(paths) > 1 {
				label = "Generated PNGs:"
			}
			rows = append(rows, PanelRow{
				Mark:   "✓",
				Timing: timing(started),
				Label:  label,
				// Upstream joins the page files with `"; "`
				// (`progress_panel.py:102`).
				Value: strings.Join(displayAll(paths), "; "),
			})
		}
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

// renderPDF is `generate_pdf` (`pdf_png.py:16-44`): resolve the path, copy the
// photo next to the `.typ`, compile.
func renderPDF(doc bridge.Document, typstPath, template string, input PathInput, inputDir string) (string, error) {
	path, err := ResolvePath(template, input)
	if err != nil {
		return "", err
	}
	if err := copyPhotoNextToTypst(doc, typstPath); err != nil {
		return "", err
	}
	_, err = typstc.Compile(context.Background(), typstc.Request{
		InputPath:  typstPath,
		OutputPath: path,
		Format:     typstc.FormatPDF,
		FontDirs:   []string{filepath.Join(inputDir, "fonts")},
		Today:      doc.Settings.CurrentDate,
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// renderPNGs is `generate_png` (`pdf_png.py:47-91`). The stale-file sweep is
// upstream's and it matters: a CV that shrinks from three pages to two would
// otherwise leave `_3.png` behind, and the golden file sets would not match.
func renderPNGs(doc bridge.Document, typstPath, template string, input PathInput, inputDir string) ([]string, error) {
	path, err := ResolvePath(template, input)
	if err != nil {
		return nil, err
	}

	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dir := filepath.Dir(path)
	stale, err := filepath.Glob(filepath.Join(dir, stem+"_*.png"))
	if err != nil {
		return nil, err
	}
	for _, existing := range stale {
		if info, err := os.Stat(existing); err == nil && info.Mode().IsRegular() {
			if err := os.Remove(existing); err != nil {
				return nil, err
			}
		}
	}

	if err := copyPhotoNextToTypst(doc, typstPath); err != nil {
		return nil, err
	}

	// The compiler writes `<prefix>_<n>.png` itself, so it is handed the stem
	// rather than a file name.
	result, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  typstPath,
		OutputPath: filepath.Join(dir, stem),
		Format:     typstc.FormatPNG,
		FontDirs:   []string{filepath.Join(inputDir, "fonts")},
		Today:      doc.Settings.CurrentDate,
	})
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, result.Pages)
	for page := 1; page <= result.Pages; page++ {
		paths = append(paths, filepath.Join(dir, fmt.Sprintf("%s_%d.png", stem, page)))
	}
	return paths, nil
}

// copyPhotoNextToTypst is `copy_photo_next_to_typst_file` (`pdf_png.py:94-111`).
// The Typst source refers to the photo by base name, because the compiler
// resolves image paths relative to the source file — so the file has to be
// beside it.
func copyPhotoNextToTypst(doc bridge.Document, typstPath string) error {
	model := doc.Model
	if model == nil || model.CvModel == nil || model.CvModel.PhotoValue == nil {
		return nil
	}
	photo := model.CvModel.PhotoValue
	if photo.Kind != cv.PhotoKindPath {
		return nil
	}

	source := photo.Path.Value
	destination := filepath.Join(filepath.Dir(typstPath), filepath.Base(source))
	if sameFile(source, destination) {
		return nil
	}

	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, raw, 0o644)
}

// sameFile is upstream's `photo_path != copy_to` guard, by identity rather than
// by string: the two spellings can differ and still name one file.
func sameFile(a, b string) bool {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	if errA != nil || errB != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

// displayAll is display over a list, preserving order.
func displayAll(paths []string) []string {
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = display(path)
	}
	return out
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

// buildArguments assembles what `modelbuilder` needs from the parsed options.
//
// It can fail because three of those options name **files**, and reading one is
// the first thing `render` does that can go wrong for a reason the argument
// vector alone cannot show.
func buildArguments(options RenderOptions) (modelbuilder.BuildArguments, error) {
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
	}, nil
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
// **Errors are a Rich panel on stdout, not text on stderr.** Every `err_*`
// golden has an empty `stderr.txt` and a box on stdout, exit 1. Writing to
// stderr meant those cases could never match no matter how right the message
// was.
//
// **There are two panels, not one**, and upstream picks between them by the kind
// of failure: `print_user_error` writes a one-message `Error` box
// (`progress_panel.py:120-136`), and `print_validation_errors` writes a
// `There are validation errors!` box wrapping a three-column table
// (`:138-169`). Rendering every failure as the first is what the port did, and
// it could not match a single validation golden.
func failPanel(stdout io.Writer, err error) {
	var validation *schemaerr.UserValidationError
	if errors.As(err, &validation) && len(validation.Errors) > 0 {
		validationPanel(stdout, validation.Errors)
		return
	}
	_, _ = fmt.Fprint(stdout, Panel("Error", []PanelRow{{Text: err.Error()}}))
}

// validationPanel is `print_validation_errors` (`progress_panel.py:138-169`).
func validationPanel(stdout io.Writer, records []schemaerr.ValidationError) {
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, []string{
			validationLocation(record),
			record.Input,
			record.Message,
		})
	}

	columns := []TableColumn{
		{Header: "Location", NoWrap: true},
		{Header: "Input Value", NoWrap: true},
		{Header: "Explanation"},
	}

	// The table is laid out at the panel's inner width, then each of its lines
	// becomes a row of that panel.
	table := Table(columns, rows, PanelWidth-4)

	var panelRows []PanelRow
	for line := range strings.SplitSeq(strings.TrimRight(table, "\n"), "\n") {
		panelRows = append(panelRows, PanelRow{Text: line})
	}
	_, _ = fmt.Fprint(stdout, Panel("There are validation errors!", panelRows))
}

// validationLocation is `format_validation_error_location`
// (`progress_panel.py:14-35`): the schema path when there is one, and otherwise
// the YAML source with the line span, because a parse error has no schema path
// to name.
func validationLocation(record schemaerr.ValidationError) string {
	if len(record.SchemaLocation) > 0 {
		return strings.Join(record.SchemaLocation, ".")
	}
	if record.YamlLocation == nil {
		return string(record.YamlSource)
	}
	start, end := record.YamlLocation.Start.Line, record.YamlLocation.End.Line
	if start == end {
		return fmt.Sprintf("%s: line %d", record.YamlSource, start)
	}
	return fmt.Sprintf("%s: line %d to line %d", record.YamlSource, start, end)
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
