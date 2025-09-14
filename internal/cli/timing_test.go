package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// TestRenderPanelHasNoTrailingNewline is the trailing-newline half of the
// two blocker findings a fresh-context verifier caught with no test gating
// either. `rich.live.Live.stop()` only calls `console.line()` — the blank
// line after a panel — when the console is a terminal; piped, upstream's
// last byte is the panel's own closing corner. A version of this fix that
// reverted to `fmt.Fprint(stdout, Panel(...))` passed every other test in
// the package while getting this wrong.
func TestRenderPanelHasNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	if strings.HasSuffix(stdout.String(), "\n") {
		t.Errorf("stdout ends with a trailing newline; upstream's render panel never does")
	}
}

// TestErrorPanelHasNoTrailingNewline is the same check for the other two
// panels `writeLivePanel` covers — the plain `Error` panel and the
// validation-error table both go through the same `rich.live.Live`
// mechanism as the success panel.
func TestErrorPanelHasNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does_not_exist.yaml")

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: missing, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}
	if strings.HasSuffix(stdout.String(), "\n") {
		t.Errorf("stdout ends with a trailing newline; upstream's error panel never does")
	}
}

// timingPattern matches one panel row's duration column, e.g. "1709 ms".
var timingPattern = regexp.MustCompile(`✓\s+(\d+) ms`)

// TestStepTimingsAreNotCumulative is the duration half of the two blocker
// findings: `started` used to be set once before every step and reused, so
// each row reported elapsed time *since the render began* rather than that
// step's own duration — upstream times each step independently
// (`run_rendercv.py:54-57`). Needs a real PDF/PNG render (the slow steps) to
// show: without one, every step is fast enough that a cumulative bug and a
// per-step one look the same.
func TestStepTimingsAreNotCumulative(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	matches := timingPattern.FindAllStringSubmatch(stdout.String(), -1)
	if len(matches) != 5 {
		t.Fatalf("found %d timing rows, want 5 (typst, pdf, png, markdown, html):\n%s",
			len(matches), stdout.String())
	}

	// **Markdown and HTML come after PDF and PNG** — the two steps a WASI
	// typst compile makes slow (seconds, not milliseconds) — so a cumulative
	// bug inherits that duration on every row after it. A per-step timer
	// does not: Markdown and HTML render in memory with no external process,
	// so their own duration stays well under what PDF's compile takes.
	pdfMs := parseMs(t, matches[1][1])
	markdownMs := parseMs(t, matches[3][1])
	htmlMs := parseMs(t, matches[4][1])

	if markdownMs >= pdfMs {
		t.Errorf("markdown timing (%d ms) >= pdf timing (%d ms); looks cumulative, not per-step",
			markdownMs, pdfMs)
	}
	if htmlMs >= pdfMs {
		t.Errorf("html timing (%d ms) >= pdf timing (%d ms); looks cumulative, not per-step",
			htmlMs, pdfMs)
	}
}

func parseMs(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("parsing timing %q: %v", s, err)
	}
	return n
}
