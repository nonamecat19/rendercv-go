package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDocumentGatesEveryFormatWithoutAFlag is G-6, which nothing gated:
// reverting the fix — reading the CLI options where `render.go` reads
// `doc.Settings.RenderCommand` — left `go test ./...` and the conformance suite
// entirely green, because **no corpus case puts a `render_command` block in its
// document.**
//
// Upstream gates inside each generator on the *merged* model
// (`renderer/pdf_png.py:33,63`, `typst.py:23`, `markdown.py:22`, `html.py:26`),
// so `settings.render_command.dont_generate_markdown: true` in the document is
// the same switch `--dont-generate-markdown` is, and needs no flag beside it.
// The port used to read the flag alone and generate all five formats for the
// document below.
//
// The four suppressed formats are set in the **document only** — that is the
// whole vector. A flag anywhere in this test would make it pass under the
// reverted code too.
func TestDocumentGatesEveryFormatWithoutAFlag(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	input := filepath.Join(dir, "cv.yaml")
	document := "cv:\n  name: John Doe\nsettings:\n  render_command:\n" +
		"    dont_generate_markdown: true\n" +
		"    dont_generate_html: true\n" +
		"    dont_generate_pdf: true\n" +
		"    dont_generate_png: true\n"
	if err := os.WriteFile(input, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := Render(RenderOptions{InputPath: input, OutputFolder: out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}

	// The `.typ` is the one format the document left on, so its presence is
	// what distinguishes "the gate works" from "nothing rendered at all".
	if got := outputNames(t, out); !slices.Equal(got, []string{"John_Doe_CV.typ"}) {
		t.Errorf("output folder = %v, want only the Typst file", got)
	}
}

// TestDocumentGateAndFlagAgreeOnTheSameFormat is the pair G-6 is really about:
// the two sources of truth are one setting, so the flag alone and the document
// alone must produce the same file set.
func TestDocumentGateAndFlagAgreeOnTheSameFormat(t *testing.T) {
	render := func(t *testing.T, document string, options RenderOptions) []string {
		t.Helper()

		dir := t.TempDir()
		options.InputPath = filepath.Join(dir, "cv.yaml")
		options.OutputFolder = filepath.Join(dir, "out")
		if err := os.WriteFile(options.InputPath, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		if code := Render(options, &stdout, &stderr); code != 0 {
			t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
		}
		return outputNames(t, options.OutputFolder)
	}

	// PDF and PNG stay off on both sides throughout: they are a real WASI
	// compile, and the format under test is Markdown and HTML either way.
	plain := "cv:\n  name: John Doe\nsettings:\n  render_command:\n" +
		"    dont_generate_pdf: true\n    dont_generate_png: true\n"

	byFlag := render(t, plain, RenderOptions{NoMarkdown: true, NoHTML: true})
	byDocument := render(t, plain+"    dont_generate_markdown: true\n    dont_generate_html: true\n", RenderOptions{})

	if !slices.Equal(byFlag, byDocument) {
		t.Errorf("document gate produced %v, the flag produced %v — they are one setting", byDocument, byFlag)
	}

	// And with neither, both formats are written — without which the two
	// comparisons above would agree on an empty folder for the wrong reason.
	ungated := render(t, plain, RenderOptions{})
	for _, want := range []string{"John_Doe_CV.md", "John_Doe_CV.html"} {
		if !slices.Contains(ungated, want) {
			t.Errorf("ungated render = %v, want it to contain %q", ungated, want)
		}
	}
}

// outputNames is the sorted contents of the output folder, or nothing when the
// render wrote no folder at all.
func outputNames(t *testing.T, folder string) []string {
	t.Helper()

	entries, err := os.ReadDir(folder)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names
}
