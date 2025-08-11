package typstc_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/typstc"
)

// TestCompilePDF is the smallest end-to-end proof that the embedded compiler
// runs: a two-line document with no packages and no theme.
func TestCompilePDF(t *testing.T) {
	dir := t.TempDir()
	input := write(t, dir, "doc.typ", "#set page(width: 200pt, height: 100pt)\nhello\n")
	output := filepath.Join(dir, "doc.pdf")

	result, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  input,
		OutputPath: output,
		Today:      time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if result.Pages != 1 {
		t.Errorf("pages = %d, want 1", result.Pages)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading the PDF: %v", err)
	}
	if !strings.HasPrefix(string(raw), "%PDF-") {
		t.Errorf("output is not a PDF: starts %q", raw[:min(8, len(raw))])
	}
}

// TestCompilePNGNamesPagesOneBased pins `<out>_<n>.png`, which is upstream's
// naming (`pdf_png.py:86-89`) and what the golden file sets expect.
func TestCompilePNGNamesPagesOneBased(t *testing.T) {
	dir := t.TempDir()
	input := write(t, dir, "doc.typ", "#set page(width: 100pt, height: 50pt)\na\n#pagebreak()\nb\n")

	result, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  input,
		OutputPath: filepath.Join(dir, "doc"),
		Format:     typstc.FormatPNG,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if result.Pages != 2 {
		t.Errorf("pages = %d, want 2", result.Pages)
	}

	for _, name := range []string{"doc_1.png", "doc_2.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "doc_0.png")); err == nil {
		t.Error("doc_0.png exists: page numbering is zero-based")
	}
}

// TestCompileReportsDiagnostics checks that a compilation error arrives as the
// compiler's own text, not as a Go-phrased wrapper. The CLI's error panel
// prints it verbatim, so its shape is part of axis 4.
func TestCompileReportsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	input := write(t, dir, "bad.typ", "#panic(\"deliberate\")\n")

	_, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  input,
		OutputPath: filepath.Join(dir, "bad.pdf"),
	})
	if err == nil {
		t.Fatal("Compile succeeded on a document that panics")
	}

	var compileErr *typstc.Error
	if !asError(err, &compileErr) {
		t.Fatalf("error is %T, want *typstc.Error", err)
	}
	if !strings.Contains(compileErr.Diagnostics, "deliberate") {
		t.Errorf("diagnostics = %q, want the compiler's own message", compileErr.Diagnostics)
	}
}

// TestFontsResolveWithoutAnyHostFonts is the one that matters for D-007: the
// vendored tree alone must satisfy a theme font, with nothing supplied by the
// caller and nothing found on the host.
func TestFontsResolveWithoutAnyHostFonts(t *testing.T) {
	dir := t.TempDir()
	input := write(t, dir, "font.typ",
		"#set page(width: 300pt, height: 100pt)\n#set text(font: \"Source Sans 3\")\nSource Sans\n")

	if _, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  input,
		OutputPath: filepath.Join(dir, "font.pdf"),
	}); err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

// TestOutputMustLiveBesideTheInput pins the mount boundary. Only the document's
// own directory is visible to the compiler, so an output elsewhere is refused
// here rather than failing inside WASI with an unreadable error.
func TestOutputMustLiveBesideTheInput(t *testing.T) {
	dir := t.TempDir()
	input := write(t, dir, "doc.typ", "hello\n")

	_, err := typstc.Compile(context.Background(), typstc.Request{
		InputPath:  input,
		OutputPath: filepath.Join(t.TempDir(), "elsewhere.pdf"),
	})
	if err == nil {
		t.Fatal("Compile accepted an output path outside the document's directory")
	}
	if !strings.Contains(err.Error(), "outside the document's directory") {
		t.Errorf("error = %v, want the boundary message", err)
	}
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// asError is errors.As, spelled out so the test file needs no import that the
// assertion above would otherwise hide.
func asError(err error, target **typstc.Error) bool {
	for err != nil {
		if hit, ok := err.(*typstc.Error); ok {
			*target = hit
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
