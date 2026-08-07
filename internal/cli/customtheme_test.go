package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// A custom theme whose folder is missing is reported before anything renders —
// upstream's behavior and upstream's wording (`design.py:72-79`). The port used
// to render happily, which a fresh-context verifier measured.
func TestRenderReportsAMissingThemeFolder(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte(
		"cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code == 0 {
		t.Errorf("exit code = 0, want a failure")
	}
	if !strings.Contains(stderr.String(), "does not exist") {
		t.Errorf("stderr = %q, want upstream's folder message", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing written", stdout.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "out")); len(entries) != 0 {
		t.Errorf("artifacts were written despite the error")
	}
}

// A built-in theme needs no folder, so the check must not fire for one.
func TestRenderAcceptsABuiltinTheme(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte(
		"cv:\n  name: John Doe\ndesign:\n  theme: sb2nov\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr); code != 0 {
		t.Errorf("exit = %d, stderr = %q", code, stderr.String())
	}
}
