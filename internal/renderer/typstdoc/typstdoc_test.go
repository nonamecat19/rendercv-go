package typstdoc_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/typstdoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/models"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/valctx"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

var now = time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)

func render(t *testing.T, document string) string {
	t.Helper()
	node, err := yamlreader.ReadString(document)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}

	model, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("the document did not validate: %v", errs)
	}

	out, err := typstdoc.Render(bridge.Resolve(model, now), typstdoc.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

const document = `
cv:
  name: John Doe
  email: john@example.com
  sections:
    education:
      - institution: MIT
        area: CS
        degree: BS
        start_date: 2000-09
        end_date: 2005-05
        location: Cambridge, MA
        highlights:
          - GPA 4.0
    notes:
      - A note.
`

// The document has every part `render_full_template` assembles, in its order.
// This is not the parity gate — the corpus `.typ` diff is — but it is what says
// the pieces are wired to each other at all.
func TestRenderAssemblesTheWholeDocument(t *testing.T) {
	out := render(t, document)

	for _, want := range []string{
		`#import "@preview/rendercv:0.3.0": *`, // the preamble
		`= John Doe`,                           // the header
		`#connections(`,
		`== Education`, // a section beginning, from the section title
		`#education-entry(`,
		`== Notes`,
		`A note.`, // a TextEntry renders as itself
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the document does not contain %q", want)
		}
	}
}

// The entry templates expanded before the fragment rendered, which is what makes
// `main_column` exist at all — and the Typst conversion ran over the result, so
// the template's own `**` became `#strong[…]` rather than being escaped.
func TestEntryTemplatesReachTheFragment(t *testing.T) {
	out := render(t, document)

	if !strings.Contains(out, "#strong[MIT], CS") {
		t.Errorf("the education entry's main column is missing from:\n%s", out)
	}
	if !strings.Contains(out, "Sept 2000 – May 2005") {
		t.Errorf("the education entry's date range is missing from:\n%s", out)
	}
}

// The locale reaches the preamble as a code and a direction, not as a name.
func TestTheLocaleReachesThePreamble(t *testing.T) {
	out := render(t, strings.Replace(document, "cv:", "locale:\n  language: arabic\ncv:", 1))

	if !strings.Contains(out, `locale-catalog-language: "ar"`) {
		t.Errorf("the preamble does not carry the Arabic code")
	}
	if !strings.Contains(out, "text-direction: rtl") {
		t.Errorf("the preamble does not carry the right-to-left direction")
	}
}

// A URL photo is **reported**, not rendered.
//
// Downloading it is spec §4 behavior 15, which this iteration does not port. The
// code used to hardcode the photo as falsy, so a document with one rendered a
// header missing its whole `#grid` — 157 bytes of upstream output gone, exit 0,
// no warning. A missing feature that says so is not a divergence; a silent
// corruption is.
func TestAURLPhotoIsReported(t *testing.T) {
	node, err := yamlreader.ReadString(`
cv:
  name: John Doe
  photo: https://example.com/me.png
  sections:
    notes:
      - A note.
`)
	if err != nil {
		t.Fatalf("reading the document: %v", err)
	}

	model, errs := models.Validate(node,
		&valctx.ValidationContext{CurrentDate: now}, schemaerr.SourceMain)
	if len(errs) > 0 {
		t.Fatalf("the document did not validate: %v", errs)
	}

	out, err := typstdoc.Render(bridge.Resolve(model, now), typstdoc.Options{})
	if !errors.Is(err, typstdoc.ErrPhotoDownloadUnsupported) {
		t.Fatalf("Render = %q, %v; want ErrPhotoDownloadUnsupported", out, err)
	}
}
