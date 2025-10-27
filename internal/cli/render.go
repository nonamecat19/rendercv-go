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
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
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

	// Extras are the tokens the flag parser did not recognize, in order — the
	// `--cv.phone value` pairs of spec §2 behavior 9 and anything else that
	// looks like one. They stay a list until `Render` has read the input file,
	// because that is where upstream parses them (`render_command.py:217`,
	// after the read at `:205`), and the two failures order differently if it
	// happens sooner.
	Extras []string

	// Overrides is what ParseOverrideArguments made of Extras. `Render` fills
	// it; nothing outside sets it.
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

// errWatchNotImplemented is what `--watch` reports until iteration 13 ports
// the watcher.
var errWatchNotImplemented = errors.New(
	"--watch is not implemented yet: rendercv-go does not have a file watcher")

// Render is the `render` command (spec 012 §2).
//
// **PDF and PNG are iteration 10's**, so this writes the three text artifacts
// and reports the two it cannot produce rather than pretending they exist.
func Render(options RenderOptions, stdout, stderr io.Writer) int {
	// **`--watch` used to parse and silently do a single render**, which is a
	// worse failure than rejecting it: upstream `--watch` never returns until
	// a watched file changes, so a one-shot render that exits 0 answers a
	// different question than the one asked (G-10). The watcher itself is
	// spec §6.2's, iteration 13's work; until it lands, saying so beats
	// pretending nothing was requested.
	if options.Watch {
		fail(stderr, errWatchNotImplemented)
		return exitValidationError
	}

	// **`--quiet` silences the console the progress panel renders on**, not
	// just the success box: upstream builds the whole `ProgressPanel` on
	// `rich.console.Console(quiet=quiet)` (`progress_panel.py:62`), so
	// `print_user_error` and `print_validation_errors` emit nothing under `-q`
	// too. The port gated only the success panel, so a validation failure
	// printed its full table where upstream prints zero bytes.
	//
	// The decorator path is deliberately **not** silenced: it is a plain
	// `rich.print` outside the Live's console, and an empty input file under
	// `-q` is 553 bytes on both sides — measured.
	liveOut := stdout
	if options.Quiet {
		liveOut = io.Discard
	}

	raw, err := os.ReadFile(options.InputPath)
	if err != nil {
		// The trailing `!` is upstream's own message text, not this port's
		// punctuation choice, so `ST1005` is suppressed rather than obeyed —
		// obeying it would be a validation-error divergence (axis 4).
		failPanel(liveOut, errMissingFile(options.InputPath))
		return exitValidationError
	}

	// **Overlay files the document names itself** (G-7). Upstream's
	// `collect_input_file_paths` (`run_rendercv.py:113-122`) reads the main YAML
	// before anything else and resolves `settings.render_command.design` and
	// `.locale` **relative to the input file's directory**, unless the CLI flag
	// already supplied one. Without this the port rendered `classic` where
	// upstream rendered the named theme — 240 differing `.typ` lines.
	//
	// **It runs before the extras are parsed** (`render_command.py:205` against
	// `:228`), which is what an empty file passed with an odd override reports:
	// upstream names the empty file, not the override.
	if err := resolveNamedOverlays(&options, raw); err != nil {
		failPrintedPanel(stdout, err)
		return exitValidationError
	}

	// **After the read, not before.** Upstream parses the extras at
	// `render_command.py:228`, and `:205` has already opened the input file by
	// then, so a missing file and a malformed extra report in that order.
	options.Overrides, err = ParseOverrideArguments(options.Extras)
	if err != nil {
		failPrintedPanel(stdout, err)
		return exitValidationError
	}

	arguments, err := buildArguments(options)
	if err != nil {
		failPanel(liveOut, err)
		return exitValidationError
	}

	built, err := modelbuilder.BuildDictionary(string(raw), arguments)
	if err != nil {
		failPanel(liveOut, err)
		return exitValidationError
	}

	context := &valctx.ValidationContext{InputFilePath: options.InputPath}
	model, err := modelbuilder.BuildModel(built, context)
	if err != nil {
		failPanel(liveOut, err)
		return exitValidationError
	}

	doc := bridge.Resolve(model, context.Today())
	inputDir := filepath.Dir(options.InputPath)

	// The custom-theme folder checks used to run here, as a user error. They are
	// upstream's *validation* records (`design.py:72-86`), so they moved into
	// `design.Validate` and arrive through `BuildModel` above with every other
	// record — which is what `err_unknown_theme` compares.

	pathInput := PathInput{
		Name: plainName(doc),
		// **Relative to the input file, not the working directory** (G-8).
		// Upstream types it `PlannedPathRelativeToInput`
		// (`schema/models/settings/render_command.py:30`), so
		// `render sub/cv.yaml` writes `sub/rendercv_output/` — measured. The
		// port wrote `./rendercv_output/`, which is the same path only when the
		// input is in the working directory, as it is in every corpus case.
		OutputFolder: outputFolderFor(options),
		Placeholders: process.BuildDatePlaceholders(doc.Settings.CurrentDate, process.Catalog{
			MonthNames:         doc.Locale.MonthNames,
			MonthAbbreviations: doc.Locale.MonthAbbreviations,
		}),
	}

	// **The order is upstream's**: Typst, then PDF, then PNG, then Markdown,
	// then HTML — and it is the order the result panel lists them in.
	var rows []PanelRow
	markdown := ""
	typstPath := ""

	// **Gated on the merged model, not the CLI flag** (G-6). The flags were
	// already merged into `settings.render_command` by `buildArguments`, and
	// upstream gates inside each generator on the merged value — so a document
	// or a `--settings` overlay switching a format off works exactly as the
	// flag does.
	generate := doc.Settings.RenderCommand

	if !generate.DontGenerateTypst {
		stepStart := time.Now()
		out, err := document.Render(doc, templater.FormatTypst, document.Options{InputDir: inputDir})
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		path, err := writeArtifact(orDefault(options.TypstPath, DefaultTypstPath), pathInput, out)
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		typstPath = path
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(stepStart), Label: "Generated Typst:", Value: display(path)})
	}

	// **PDF and PNG both depend on the `.typ` file existing on disk**, which is
	// why upstream returns early from both when `typst_path is None`
	// (`pdf_png.py:33,63`): `--notyp` disables them by omission, not by a flag
	// of their own.
	if !generate.DontGeneratePDF && typstPath != "" {
		stepStart := time.Now()
		path, err := renderPDF(doc, typstPath, orDefault(options.PDFPath, DefaultPDFPath), pathInput, inputDir)
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(stepStart), Label: "Generated PDF:", Value: display(path)})
	}

	if !generate.DontGeneratePNG && typstPath != "" {
		stepStart := time.Now()
		paths, err := renderPNGs(doc, typstPath, orDefault(options.PNGPath, DefaultPNGPath), pathInput, inputDir)
		if err != nil {
			failPanel(liveOut, err)
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
				Timing: timing(stepStart),
				Label:  label,
				// Upstream joins the page files with `"; "`
				// (`progress_panel.py:102`).
				Value: strings.Join(displayAll(paths), "; "),
			})
		}
	}

	if !generate.DontGenerateMarkdown {
		stepStart := time.Now()
		out, err := document.Render(doc, templater.FormatMarkdown, document.Options{InputDir: inputDir})
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		markdown = out
		path, err := writeArtifact(orDefault(options.MarkdownPath, DefaultMarkdownPath), pathInput, out)
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(stepStart), Label: "Generated Markdown:", Value: display(path)})
	}

	if !generate.DontGenerateHTML {
		// **The HTML needs the Markdown's text**, and upstream disables it
		// outright when the Markdown was not generated (`html.py:28-30`) rather
		// than rendering one just for this.
		if markdown == "" {
			return finish(options, rows, liveOut)
		}
		stepStart := time.Now()
		out, err := document.RenderHTML(doc, markdown, document.Options{InputDir: inputDir})
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		path, err := writeArtifact(orDefault(options.HTMLPath, DefaultHTMLPath), pathInput, out)
		if err != nil {
			failPanel(liveOut, err)
			return exitValidationError
		}
		rows = append(rows, PanelRow{Mark: "✓", Timing: timing(stepStart), Label: "Generated HTML:", Value: display(path)})
	}

	return finish(options, rows, liveOut)
}

// finish prints the result panel unless `--quiet` silenced it.
//
// **`--quiet` produces no stdout at all** — measured on `render_quiet`, whose
// golden stdout is zero bytes, not a panel without the progress lines.
// **A run that completed no steps still has a body.**
// `print_progress_panel` ends with
// `content = "\n".join(lines) if lines else "Rendering..."`
// (`progress_panel.py:110`), so switching every format off — `-notyp -nopdf
// -nopng -nomd -nohtml` — leaves the literal `Rendering...` in the finished
// panel, where the port printed an empty box. No corpus case disables
// everything, which is why the suite never saw it.
func finish(options RenderOptions, rows []PanelRow, stdout io.Writer) int {
	if options.Quiet {
		return 0
	}
	if len(rows) == 0 {
		rows = []PanelRow{{Text: "Rendering..."}}
	}
	writeLivePanel(stdout, Panel("Your CV is ready", rows))
	return 0
}

// writeLivePanel writes a panel the way `render`'s three panels reach the
// user: through `rich.live.Live` (`progress_panel.py`'s `ProgressPanel`), not
// a plain `print()`. `Live.stop()` only calls `console.line()` — the blank
// line after the panel — when the console is a terminal
// (`rich/live.py:169-172`); piped or redirected, as every golden and this
// port's own stdout capture are, upstream's last byte is the panel's own
// closing corner, with no trailing newline at all.
//
// `Panel` itself keeps its trailing `\n`, because `new` and `create-theme`
// print through plain `rich.print`, which always ends with one — measured
// separately and differently from `render`'s three.
func writeLivePanel(stdout io.Writer, panel string) {
	_, _ = fmt.Fprint(stdout, strings.TrimSuffix(panel, "\n"))
}

// writePrintedPanel is the other way a panel reaches the user: `rich.print`
// inside the `@handle_user_errors` decorator (`cli/error_handler.py:40-47`),
// which wraps the command function and therefore only ever sees failures raised
// **before** `with ProgressPanel(...)` (`render_command.py:231`).
//
// `rich.print` terminates with a newline, so this panel's last byte is `\n`
// where `writeLivePanel`'s is the closing corner. Measured on an empty input
// file (553 bytes, last byte `0a`) and on an odd override vector (638 bytes,
// last byte `0a`) against the vendored Python at `COLUMNS=80`.
func writePrintedPanel(stdout io.Writer, panel string) {
	_, _ = fmt.Fprint(stdout, panel)
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
	// The overlay file is a **whole document keyed by the field**, not the
	// field's body: `-d` on a file reading `theme: moderncv` makes upstream die
	// with `KeyError: 'design'`. So the content goes across unchanged and
	// `modelbuilder` does the extraction, which it has always been able to do.
	settingsYaml, err := overlayFile(options.SettingsPath)
	if err != nil {
		return modelbuilder.BuildArguments{}, err
	}
	designYaml, err := overlayFile(options.DesignPath)
	if err != nil {
		return modelbuilder.BuildArguments{}, err
	}
	localeYaml, err := overlayFile(options.LocalePath)
	if err != nil {
		return modelbuilder.BuildArguments{}, err
	}

	return modelbuilder.BuildArguments{
		SettingsYaml:         settingsYaml,
		DesignYaml:           designYaml,
		LocaleYaml:           localeYaml,
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

// outputFolderFor resolves the output folder against the input file's
// directory. An absolute folder is taken as given.
func outputFolderFor(options RenderOptions) string {
	folder := orDefault(options.OutputFolder, DefaultOutputFolder)
	if filepath.IsAbs(folder) {
		return folder
	}
	return filepath.Join(filepath.Dir(options.InputPath), folder)
}

// resolveNamedOverlays fills in the two overlay paths a document can name for
// itself.
//
// **A parse failure is ignored, exactly as upstream ignores it.** The read sits
// inside `contextlib.suppress(RenderCVUserValidationError)` so that a broken
// document still reaches the validator and reports there, rather than failing
// here with a worse message.
func resolveNamedOverlays(options *RenderOptions, raw []byte) error {
	document, err := modelbuilder.ReadYamlWithValidationErrors(string(raw), schemaerr.SourceMain)
	if err != nil {
		// **`contextlib.suppress(RenderCVUserValidationError)`
		// (`run_rendercv.py:113`) suppresses that one class and nothing else.**
		// A YAML *syntax* error is swallowed here and raised again inside the
		// progress panel, which is why it prints a validation table with no
		// trailing newline; an *empty* file is a plain `RenderCVUserError`,
		// escapes to the decorator, and prints one with. Measured: 553 bytes
		// ending `0a` against 1411 ending `af`.
		var userError *schemaerr.UserError
		if errors.As(err, &userError) {
			return userError
		}
		return nil
	}

	block := mappingChild(mappingChild(document, "settings"), "render_command")
	inputDir := filepath.Dir(options.InputPath)

	for _, named := range []struct {
		key    string
		target *string
	}{
		{key: "design", target: &options.DesignPath},
		{key: "locale", target: &options.LocalePath},
	} {
		// **The CLI flag wins** (`render_command.py:206-209`): the document's
		// name is only consulted when the flag left the slot empty.
		if *named.target != "" {
			continue
		}
		value := mappingChild(block, named.key)
		if value == nil || value.Kind != yamldoc.KindString || value.Raw == "" {
			continue
		}
		*named.target = filepath.Join(inputDir, value.Raw)
	}

	return nil
}

// mappingChild is one key of a mapping node, or nil.
func mappingChild(node *yamldoc.Node, key string) *yamldoc.Node {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil
	}
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value
		}
	}
	return nil
}

// overlayFile reads one of the three overlay options. An empty path is the
// option not being passed, which is the path every corpus document takes.
//
// **Upstream has no handling for a missing one**: `design.read_text()`
// (`render_command.py:213`) is unguarded, so `-d nosuch.yaml` produces a
// `FileNotFoundError` traceback — the same shape as a missing *input* file,
// whose golden `err_missing_file` is a traceback for that reason and is
// unreachable by construction. The port answers both with the one message it
// already had rather than inventing a second.
func overlayFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errMissingFile(path)
	}
	return string(content), nil
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
	writeLivePanel(stdout, Panel("Error", []PanelRow{{Text: err.Error()}}))
}

// failPrintedPanel is `failPanel`'s pre-progress-panel twin: the `Error` box the
// `@handle_user_errors` decorator prints (`cli/error_handler.py:40-47`) for a
// `RenderCVUserError` raised before `with ProgressPanel(...)`
// (`render_command.py:231`).
//
// **The choice between the two is positional, not type-based.** Upstream raises
// `RenderCVUserError` on both sides of that line — a Typst compile failure is
// one too — and only the ones raised earlier get the decorator's trailing
// newline. So this is called from the two pre-panel phases and nowhere else.
//
// The decorator never renders a validation table: `collect_input_file_paths`
// suppresses `RenderCVUserValidationError` (`run_rendercv.py:113`), so the only
// box that can reach `rich.print` is the one-message one.
func failPrintedPanel(stdout io.Writer, err error) {
	writePrintedPanel(stdout, Panel("Error", []PanelRow{{Text: err.Error()}}))
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
	table := Table(columns, rows, ConsoleWidth()-4)

	var panelRows []PanelRow
	for line := range strings.SplitSeq(strings.TrimRight(table, "\n"), "\n") {
		panelRows = append(panelRows, PanelRow{Text: line})
	}
	writeLivePanel(stdout, Panel("There are validation errors!", panelRows))
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

// timing is the duration Rich prints beside each artifact — `run_rendercv.py:57`
// (`f"{(end - start) * 1000:.0f}"`) concatenated with `" ms"` in
// `progress_panel.py:97`. Upstream always reports milliseconds, rounded to the
// nearest whole one; there is no seconds form.
//
// **Its content is not part of the contract and its shape is.** The conformance
// harness rewrites `\d+(\.\d+)?\s?(ms|s)` to `<duration> `, consuming the
// padding that follows — so a timing that does not match that pattern survives
// into the comparison and fails, and one that does is erased on both sides.
//
// **A fresh-context verifier caught the port printing `%.1fs`** (seconds, one
// decimal) where upstream always prints whole milliseconds — invisible to
// `TestParity` because the harness's own pattern matches both shapes, but a
// real, undocumented divergence in anything reading raw output.
func timing(since time.Time) string {
	elapsed := time.Since(since)
	return fmt.Sprintf("%.0f ms", elapsed.Seconds()*1000)
}
