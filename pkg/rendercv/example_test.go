package rendercv_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nonamecat19/rendercv-go/pkg/rendercv"
)

// Build validates a document and returns a model ready to render.
func Example() {
	const document = `cv:
  name: Jane Roe
  sections:
    experience:
      - company: Acme
        position: Engineer
`

	dir, _ := os.MkdirTemp("", "rendercv-example")
	defer func() { _ = os.RemoveAll(dir) }()
	input := filepath.Join(dir, "cv.yaml")
	_ = os.WriteFile(input, []byte(document), 0o600)

	_, model, err := rendercv.Build(document, rendercv.BuildOptions{
		InputFilePath: input,
		// The PDF and PNGs need the Typst compiler, which is slower than an
		// example wants to be.
		DontGeneratePDF: true,
		DontGeneratePNG: true,
	})
	if err != nil {
		fmt.Println("build failed:", err)
		return
	}

	typst, err := rendercv.GenerateTypst(model)
	if err != nil {
		fmt.Println("typst failed:", err)
		return
	}
	markdown, err := rendercv.GenerateMarkdown(model)
	if err != nil {
		fmt.Println("markdown failed:", err)
		return
	}
	// Passing GenerateMarkdown's result straight through is deliberate: when
	// the Markdown was switched off it is "", and GenerateHTML then skips too,
	// which is what upstream does.
	html, err := rendercv.GenerateHTML(model, markdown)
	if err != nil {
		fmt.Println("html failed:", err)
		return
	}

	fmt.Println(model.Name())
	fmt.Println(filepath.Base(typst), filepath.Base(markdown), filepath.Base(html))
	// Output:
	// Jane Roe
	// Jane_Roe_CV.typ Jane_Roe_CV.md Jane_Roe_CV.html
}

// A format that was switched off returns an empty path and a nil error. That is
// a successful outcome, not a failure, so checking err alone will not tell you
// whether a file was written.
func Example_switchedOff() {
	const document = `cv:
  name: Jane Roe
  sections:
    experience:
      - company: Acme
        position: Engineer
`

	_, model, err := rendercv.Build(document, rendercv.BuildOptions{
		DontGenerateTypst:    true,
		DontGenerateMarkdown: true,
		DontGenerateHTML:     true,
		DontGeneratePDF:      true,
		DontGeneratePNG:      true,
	})
	if err != nil {
		fmt.Println("build failed:", err)
		return
	}

	path, err := rendercv.GenerateTypst(model)
	fmt.Printf("path=%q err=%v generated=%t\n", path, err, path != "")
	// Output:
	// path="" err=<nil> generated=false
}

// A document the user got wrong comes back as a UserValidationError carrying
// every record, in the order upstream reports them.
func Example_validationErrors() {
	_, _, err := rendercv.Build("cv:\n  name: 5\n", rendercv.BuildOptions{})

	var validation *rendercv.UserValidationError
	if errors.As(err, &validation) {
		for _, record := range validation.Errors {
			fmt.Printf("%v: %s\n", record.SchemaLocation, record.Message)
		}
	}
	// Output:
	// [cv name]: Input should be a valid string.
}
