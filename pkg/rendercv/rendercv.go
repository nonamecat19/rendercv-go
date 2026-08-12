package rendercv

import (
	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/generate"
	"github.com/nonamecat19/rendercv-go/internal/schema/modelbuilder"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// generateOptions is generate.Options under an unexported name, so that Model
// can hold it without putting an internal type in an exported field.
type generateOptions = generate.Options

// ReadYAML parses a YAML document, mirroring read_yaml_with_validation_errors
// (schema/rendercv_model_builder.py:65-84).
//
// It validates nothing beyond the YAML syntax itself. A parse failure comes
// back as a [UserValidationError] carrying one record located at the offending
// line, the way upstream converts a ruamel error.
//
// source names which of the four input files this content is, which is what the
// resulting error records report.
func ReadYAML(content string, source YamlSource) (*Document, error) {
	return modelbuilder.ReadYamlWithValidationErrors(content, source)
}

// Build runs the whole schema pipeline, mirroring
// build_rendercv_dictionary_and_model (schema/rendercv_model_builder.py:192-210):
// it parses the main document, merges the design, locale and settings overlays
// and any overrides, validates the result, and resolves it for rendering.
//
// It returns the merged document and the model, mirroring upstream's
// (CommentedMap, RenderCVModel) tuple. A document that fails validation comes
// back as a [UserValidationError] carrying every record, in upstream's order.
//
// The returned [Model] is the only way to obtain one.
func Build(mainYAML string, options BuildOptions) (*Document, *Model, error) {
	arguments := modelbuilder.BuildArguments{
		SettingsYaml:         options.SettingsYAML,
		DesignYaml:           options.DesignYAML,
		LocaleYaml:           options.LocaleYAML,
		OutputFolder:         options.OutputFolder,
		TypstPath:            options.TypstPath,
		PdfPath:              options.PDFPath,
		PngPath:              options.PNGPath,
		MarkdownPath:         options.MarkdownPath,
		HtmlPath:             options.HTMLPath,
		DontGenerateTypst:    options.DontGenerateTypst,
		DontGeneratePdf:      options.DontGeneratePDF,
		DontGeneratePng:      options.DontGeneratePNG,
		DontGenerateMarkdown: options.DontGenerateMarkdown,
		DontGenerateHtml:     options.DontGenerateHTML,
		Overrides:            options.Overrides,
	}

	built, err := modelbuilder.BuildDictionary(mainYAML, arguments)
	if err != nil {
		return nil, nil, err
	}

	context := &valctx.ValidationContext{InputFilePath: options.InputFilePath}
	model, err := modelbuilder.BuildModel(built, context)
	if err != nil {
		return built.Document, nil, err
	}

	// Resolving is where the theme is selected and a custom theme's Lua script
	// runs, which is upstream's validate_design position (design.py:135).
	doc := bridge.Resolve(model, context.Today())

	// **The output folder resolves against the input file's directory**, not
	// the working directory: upstream types it PlannedPathRelativeToInput
	// (settings/render_command.py:30).
	inputDir := generate.InputDirFor(options.InputFilePath)
	outputFolder := generate.OutputFolderFor(options.InputFilePath, doc.Settings.RenderCommand.OutputFolder)

	return built.Document, &Model{
		doc: doc,
		options: generateOptions{
			InputDir:  inputDir,
			PathInput: generate.PathInputFor(doc, inputDir, outputFolder),
		},
	}, nil
}

// GenerateTypst writes the .typ file, mirroring generate_typst
// (renderer/typst.py:9-29).
//
// It returns the written path; or "" and a nil error when
// settings.render_command.dont_generate_typst is set, which is a successful
// outcome and not a failure.
func GenerateTypst(m *Model) (string, error) {
	if m == nil {
		return "", &InternalError{Message: "GenerateTypst called with a nil model"}
	}
	return generate.Typst(m.doc, m.options)
}

// GenerateMarkdown writes the .md file, mirroring generate_markdown
// (renderer/markdown.py:9-29).
//
// It returns the written path; or "" and a nil error when
// dont_generate_markdown is set.
func GenerateMarkdown(m *Model) (string, error) {
	if m == nil {
		return "", &InternalError{Message: "GenerateMarkdown called with a nil model"}
	}
	return generate.Markdown(m.doc, m.options)
}

// GenerateHTML writes the .html file, mirroring generate_html
// (renderer/html.py:9-33). It reads the Markdown back from markdownPath, which
// is upstream's contract rather than a convenience (html.py:29).
//
// It returns the written path; or "" and a nil error when dont_generate_html is
// set, or when markdownPath is empty — upstream disables HTML outright when the
// Markdown was not generated rather than rendering one just for it. So passing
// GenerateMarkdown's result straight through does the right thing in both
// cases.
func GenerateHTML(m *Model, markdownPath string) (string, error) {
	if m == nil {
		return "", &InternalError{Message: "GenerateHTML called with a nil model"}
	}
	return generate.HTML(m.doc, markdownPath, m.options)
}

// GeneratePDF writes the .pdf file, mirroring generate_pdf
// (renderer/pdf_png.py:16-44).
//
// It returns the written path; or "" and a nil error when dont_generate_pdf is
// set, or when typstPath is empty — the PDF is compiled from the .typ file on
// disk, so upstream returns early when there is none (pdf_png.py:33). Passing
// GenerateTypst's result straight through does the right thing in both cases.
func GeneratePDF(m *Model, typstPath string) (string, error) {
	if m == nil {
		return "", &InternalError{Message: "GeneratePDF called with a nil model"}
	}
	return generate.PDF(m.doc, typstPath, m.options)
}

// GeneratePNG writes one .png per page, named {stem}_{n} from 1, mirroring
// generate_png (renderer/pdf_png.py:47-91). It first removes any stale
// {stem}_*.png, as upstream does, so a CV that loses a page does not leave the
// old one behind.
//
// It returns the written paths in page order; or **a nil slice** and a nil
// error when dont_generate_png is set or typstPath is empty. A nil slice and an
// empty non-nil slice are different values, and the nil one is "not generated".
func GeneratePNG(m *Model, typstPath string) ([]string, error) {
	if m == nil {
		return nil, &InternalError{Message: "GeneratePNG called with a nil model"}
	}
	return generate.PNG(m.doc, typstPath, m.options)
}

// compile-time assertion that the error aliases really are the pipeline's own
// types, so errors.As across the boundary cannot silently stop working.
var (
	_ error = (*UserError)(nil)
	_ error = (*UserValidationError)(nil)
	_ error = (*InternalError)(nil)
	_ error = (*schemaerr.ValidationError)(nil)
)
