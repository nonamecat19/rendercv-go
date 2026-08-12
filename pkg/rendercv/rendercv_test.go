package rendercv_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nonamecat19/rendercv-go/pkg/rendercv"
)

// minimalCV is the smallest document that validates and renders.
const minimalCV = `cv:
  name: Jane Roe
  sections:
    experience:
      - company: Acme
        position: Engineer
`

// build is the common preamble: write the document into a temp directory and
// build it from there, so every artifact lands beside it rather than in the
// repository.
func build(t *testing.T, yaml string, options rendercv.BuildOptions) *rendercv.Model {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	options.InputFilePath = path

	_, model, err := rendercv.Build(yaml, options)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if model == nil {
		t.Fatal("Build returned a nil model with a nil error")
	}
	return model
}

// Spec §5.3 — the three outcomes of a generator are distinguishable.
func TestNotGeneratedIsNotAnError(t *testing.T) {
	model := build(t, minimalCV, rendercv.BuildOptions{
		DontGenerateTypst:    true,
		DontGenerateMarkdown: true,
		DontGenerateHTML:     true,
		DontGeneratePDF:      true,
		DontGeneratePNG:      true,
	})

	typstPath, err := rendercv.GenerateTypst(model)
	if err != nil || typstPath != "" {
		t.Errorf("GenerateTypst = %q, %v; want the switched-off outcome: \"\", nil", typstPath, err)
	}

	markdownPath, err := rendercv.GenerateMarkdown(model)
	if err != nil || markdownPath != "" {
		t.Errorf("GenerateMarkdown = %q, %v; want \"\", nil", markdownPath, err)
	}

	htmlPath, err := rendercv.GenerateHTML(model, markdownPath)
	if err != nil || htmlPath != "" {
		t.Errorf("GenerateHTML = %q, %v; want \"\", nil", htmlPath, err)
	}

	pdfPath, err := rendercv.GeneratePDF(model, typstPath)
	if err != nil || pdfPath != "" {
		t.Errorf("GeneratePDF = %q, %v; want \"\", nil", pdfPath, err)
	}

	// **A nil slice, not an empty one** — the two are different values in Go
	// and only the nil one means "not generated".
	pngPaths, err := rendercv.GeneratePNG(model, typstPath)
	if err != nil || pngPaths != nil {
		t.Errorf("GeneratePNG = %v, %v; want nil, nil", pngPaths, err)
	}
}

// The text formats generate, and each returns a path that exists.
func TestTheTextFormatsGenerate(t *testing.T) {
	model := build(t, minimalCV, rendercv.BuildOptions{
		DontGeneratePDF: true,
		DontGeneratePNG: true,
	})

	typstPath, err := rendercv.GenerateTypst(model)
	if err != nil {
		t.Fatalf("GenerateTypst: %v", err)
	}
	markdownPath, err := rendercv.GenerateMarkdown(model)
	if err != nil {
		t.Fatalf("GenerateMarkdown: %v", err)
	}
	htmlPath, err := rendercv.GenerateHTML(model, markdownPath)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}

	for _, path := range []string{typstPath, markdownPath, htmlPath} {
		if path == "" {
			t.Fatal("a format that was not switched off returned no path")
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// **HTML follows Markdown's switch, not only its own** — upstream disables it
// outright when the Markdown was not generated (html.py:26-30), so passing
// GenerateMarkdown's empty result through is the documented way to reach it.
func TestHTMLIsSkippedWhenMarkdownIs(t *testing.T) {
	model := build(t, minimalCV, rendercv.BuildOptions{
		DontGenerateMarkdown: true,
		DontGeneratePDF:      true,
		DontGeneratePNG:      true,
	})

	markdownPath, err := rendercv.GenerateMarkdown(model)
	if err != nil || markdownPath != "" {
		t.Fatalf("GenerateMarkdown = %q, %v; want the switched-off outcome", markdownPath, err)
	}
	htmlPath, err := rendercv.GenerateHTML(model, markdownPath)
	if err != nil || htmlPath != "" {
		t.Errorf("GenerateHTML = %q, %v; want it skipped because the Markdown was", htmlPath, err)
	}
}

// Spec §5.5 — the error types are reachable and distinguishable by errors.As
// rather than by string matching.
func TestAValidationFailureIsAUserValidationError(t *testing.T) {
	_, _, err := rendercv.Build("cv:\n  name: 5\n", rendercv.BuildOptions{})
	if err == nil {
		t.Fatal("Build accepted a non-string name")
	}

	var validation *rendercv.UserValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err is %T, want a *UserValidationError", err)
	}
	if len(validation.Errors) == 0 {
		t.Fatal("a UserValidationError carrying no records")
	}

	// The alias really is the pipeline's own type, so a record raised deep
	// inside internal/ arrives intact rather than as a re-wrapped copy.
	record := validation.Errors[0]
	if record.Message == "" {
		t.Error("a record with no message")
	}
	if len(record.SchemaLocation) == 0 {
		t.Error("a record with no schema location")
	}
}

// A YAML syntax error is a validation error too, mirroring how upstream's
// read_yaml_with_validation_errors converts a ruamel failure.
func TestASyntaxErrorIsAUserValidationError(t *testing.T) {
	_, err := rendercv.ReadYAML("cv: [\n", rendercv.SourceMain)
	if err == nil {
		t.Fatal("ReadYAML accepted an unterminated flow sequence")
	}
	var validation *rendercv.UserValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err is %T, want a *UserValidationError", err)
	}
}

// ReadYAML parses without validating: a document that is well-formed YAML but a
// nonsense CV comes back fine, because validation is Build's job.
func TestReadYAMLDoesNotValidate(t *testing.T) {
	doc, err := rendercv.ReadYAML("cv:\n  name: 5\n", rendercv.SourceMain)
	if err != nil {
		t.Fatalf("ReadYAML rejected well-formed YAML: %v", err)
	}
	if doc == nil {
		t.Fatal("ReadYAML returned no document and no error")
	}
}

// A nil model is the caller's bug, not the user's, so it is an InternalError —
// upstream's RenderCVInternalError descends from RuntimeError for the same
// reason.
func TestANilModelIsAnInternalError(t *testing.T) {
	_, err := rendercv.GenerateTypst(nil)
	var internal *rendercv.InternalError
	if !errors.As(err, &internal) {
		t.Fatalf("err is %T, want an *InternalError", err)
	}

	// It is specifically not a user error, which is the distinction upstream
	// draws with ValueError against RuntimeError.
	var user *rendercv.UserError
	if errors.As(err, &user) {
		t.Error("a nil model reported as a user error")
	}
}

// Model.Name is what a caller uses to tell several documents apart.
func TestModelNameIsTheDocumentsName(t *testing.T) {
	model := build(t, minimalCV, rendercv.BuildOptions{})
	if got := model.Name(); got != "Jane Roe" {
		t.Errorf("Name() = %q, want %q", got, "Jane Roe")
	}
	var absent *rendercv.Model
	if got := absent.Name(); got != "" {
		t.Errorf("a nil model's Name() = %q, want \"\"", got)
	}
}
