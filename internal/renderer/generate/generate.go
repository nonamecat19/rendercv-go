package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/document"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// Options is what a generator needs besides the model itself.
//
// Upstream needs no equivalent: each `generate_*` reads these off
// `rendercv_model.settings.render_command` and off the model's own
// `_input_file_path` (`rendercv_model.py:44-62`). The port keeps the resolved
// input directory and the path templates beside the model rather than inside
// it, so they are passed here.
type Options struct {
	// InputDir is the directory of the input file. Every relative path in a
	// document resolves against it, not against the working directory
	// (`schema/models/path.py:39`).
	InputDir string

	// PathInput carries what the path templates substitute into.
	PathInput PathInput

	// TypstPath and the rest are the path templates. An empty string means the
	// default for that format, exactly as an absent CLI flag does.
	TypstPath    string
	PDFPath      string
	PNGPath      string
	MarkdownPath string
	HTMLPath     string
}

// Typst is `generate_typst` (`renderer/typst.py:9-29`).
//
// It returns the written path, or **an empty path and a nil error** when
// `settings.render_command.dont_generate_typst` is set — upstream returns
// `None` there, and it is a successful outcome rather than a failure.
func Typst(doc bridge.Document, options Options) (string, error) {
	if doc.Settings.RenderCommand.DontGenerateTypst {
		return "", nil
	}
	out, err := document.Render(doc, templater.FormatTypst, document.Options{InputDir: options.InputDir})
	if err != nil {
		return "", err
	}
	return writeArtifact(orDefault(options.TypstPath, DefaultTypstPath), options.PathInput, out)
}

// Markdown is `generate_markdown` (`renderer/markdown.py:9-29`).
//
// It returns the written path, or an empty path and a nil error when
// `dont_generate_markdown` is set.
func Markdown(doc bridge.Document, options Options) (string, error) {
	if doc.Settings.RenderCommand.DontGenerateMarkdown {
		return "", nil
	}
	out, err := document.Render(doc, templater.FormatMarkdown, document.Options{InputDir: options.InputDir})
	if err != nil {
		return "", err
	}
	return writeArtifact(orDefault(options.MarkdownPath, DefaultMarkdownPath), options.PathInput, out)
}

// HTML is `generate_html` (`renderer/html.py:9-33`).
//
// **It reads the Markdown back from disk**, which is upstream's contract
// (`html.py:29`, `markdown_path.read_text`) rather than a convenience: the
// parameter is a path, and a caller may hand it one this run did not write.
//
// It returns an empty path and a nil error when `dont_generate_html` is set
// **or when markdownPath is empty** — upstream disables HTML outright when the
// Markdown was not generated (`html.py:26-30`) rather than rendering one just
// for this.
func HTML(doc bridge.Document, markdownPath string, options Options) (string, error) {
	if doc.Settings.RenderCommand.DontGenerateHTML || markdownPath == "" {
		return "", nil
	}
	markdown, err := os.ReadFile(markdownPath)
	if err != nil {
		return "", err
	}
	out, err := document.RenderHTML(doc, string(markdown), document.Options{InputDir: options.InputDir})
	if err != nil {
		return "", err
	}
	return writeArtifact(orDefault(options.HTMLPath, DefaultHTMLPath), options.PathInput, out)
}

// PDF is `generate_pdf` (`renderer/pdf_png.py:16-44`): resolve the path, copy
// the photo next to the `.typ`, compile.
//
// It returns an empty path and a nil error when `dont_generate_pdf` is set or
// when typstPath is empty — **PDF and PNG both depend on the `.typ` existing on
// disk**, which is why upstream returns early from both when `typst_path is
// None` (`pdf_png.py:33,63`): `--notyp` disables them by omission, not by a
// flag of their own.
func PDF(doc bridge.Document, typstPath string, options Options) (string, error) {
	if doc.Settings.RenderCommand.DontGeneratePDF || typstPath == "" {
		return "", nil
	}
	path, err := ResolvePath(orDefault(options.PDFPath, DefaultPDFPath), options.PathInput)
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
		FontDirs:   []string{design.Join(options.InputDir, "fonts")},
		Today:      doc.Settings.CurrentDate,
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// PNG is `generate_png` (`renderer/pdf_png.py:47-91`). The stale-file sweep is
// upstream's and it matters: a CV that shrinks from three pages to two would
// otherwise leave `_3.png` behind, and the golden file sets would not match.
//
// It returns **a nil slice** and a nil error when `dont_generate_png` is set or
// when typstPath is empty. A nil slice and an empty non-nil slice are different
// values in Go, and the nil one is the "not generated" case.
func PNG(doc bridge.Document, typstPath string, options Options) ([]string, error) {
	if doc.Settings.RenderCommand.DontGeneratePNG || typstPath == "" {
		return nil, nil
	}
	path, err := ResolvePath(orDefault(options.PNGPath, DefaultPNGPath), options.PathInput)
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
		FontDirs:   []string{design.Join(options.InputDir, "fonts")},
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

func writeArtifact(template string, input PathInput, content string) (string, error) {
	path, err := ResolvePath(template, input)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

// orDefault is the absent-flag rule: an empty template means the format's
// default, which is what an omitted CLI flag and an omitted API option both
// produce.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
