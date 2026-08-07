//go:build conformance

package document_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/document"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// probeDate is tools/typprobe's `CurrentDate`. Both sides render with it, so the
// preamble's `datetime(...)` and the top note's month are fixed rather than
// today's — which is the only reason this can be a byte comparison at all.
var probeDate = time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)

// **Axis 1.** Every corpus input rendered by the vendored Python, against the
// same input rendered by this port, byte for byte.
//
// This is what spec 009 §6's last two criteria mean and what `AGENTS.md` §10.6
// means by "parity is what the suite prints". Nothing above it — not the
// fragment differential, not the twenty-one unit criteria of iteration 8 — can
// catch a wrong entry template, a wrong separator or a missing newline in a
// whole document; those all passed while this was unwritten.
func TestCorpusTypstIsByteIdentical(t *testing.T) {
	root := filepath.Join("testdata", "documents")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v — regenerate it with `just docprobe`", root, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s is empty; the fixture is not measuring anything", root)
	}

	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			dir := filepath.Join(root, entry.Name())
			path := filepath.Join(dir, "cv.yaml")
			input := readFixture(t, path)

			markdown := ""
			for _, artifact := range []struct {
				name   string
				format templater.Format
			}{
				{"expected.typ", templater.FormatTypst},
				{"expected.md", templater.FormatMarkdown},
			} {
				want := readFixture(t, filepath.Join(dir, artifact.name))
				got := renderFixture(t, input, path, artifact.format)
				if artifact.format == templater.FormatMarkdown {
					markdown = got
				}
				if got == want {
					continue
				}
				t.Errorf("the rendered %s differs from upstream's:\n%s",
					artifact.name, firstDifference(want, got))
			}

			// **The HTML is built from the Markdown this run produced**, not
			// from the fixture's — so a `.md` regression shows up here too,
			// which is upstream's own coupling (`html.py:31-33`).
			want := readFixture(t, filepath.Join(dir, "expected.html"))
			if got := renderHTMLFixture(t, input, path, markdown); got != want {
				t.Errorf("the rendered expected.html differs from upstream's:\n%s",
					firstDifference(want, got))
			}
		})
	}
}

// renderFixture validates and renders one case. The input **path** is threaded
// through because two things resolve against it: `cv.photo`'s existence check,
// and the loader's search for a user template override.
func resolveFixture(t *testing.T, input, path string) bridge.Document {
	t.Helper()
	node, err := yamlreader.ReadString(input)
	if err != nil {
		t.Fatalf("reading the input: %v", err)
	}

	model, errs := models.Validate(node, &valctx.ValidationContext{
		CurrentDate:   probeDate,
		InputFilePath: path,
	}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("the input did not validate: %v", errs)
	}
	return bridge.Resolve(model, probeDate)
}

func renderFixture(t *testing.T, input, path string, format templater.Format) string {
	t.Helper()
	out, err := document.Render(resolveFixture(t, input, path), format, document.Options{
		InputDir: filepath.Dir(path),
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

func renderHTMLFixture(t *testing.T, input, path, markdown string) string {
	t.Helper()
	out, err := document.RenderHTML(resolveFixture(t, input, path), markdown,
		document.Options{InputDir: filepath.Dir(path)})
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	return out
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// firstDifference reports the first differing line with a little context, which
// is what a byte diff on a 4 KB document needs to be readable. A whole-document
// dump says "these two files differ" in four hundred lines.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		wantLine, gotLine := lineAt(wantLines, i), lineAt(gotLines, i)
		if wantLine == gotLine {
			continue
		}

		var out strings.Builder
		for j := max(0, i-2); j < i; j++ {
			out.WriteString("  " + lineAt(wantLines, j) + "\n")
		}
		out.WriteString("- want: " + wantLine + "\n")
		out.WriteString("+ got:  " + gotLine + "\n")
		return out.String()
	}
	return "the documents differ only in their trailing bytes"
}

func lineAt(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return "<end of document>"
	}
	return lines[i]
}
