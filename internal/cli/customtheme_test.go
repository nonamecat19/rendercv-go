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
	// **Errors are a panel on stdout**, and stderr stays empty — every `err_*`
	// golden is shaped that way. This test asserted the reverse until the
	// goldens were read.
	//
	// The message is read through `flatten` because a panel wraps its rows at
	// the console width, so any phrase long enough to matter is split across
	// lines by the border.
	if !strings.Contains(flatten(stdout.String()), "does not exist") {
		t.Errorf("stdout = %q, want upstream's folder message", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing written", stderr.String())
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

// flatten strips a panel's borders and collapses the wrapping, so a test can
// assert on a message rather than on where the box happened to break it.
func flatten(panel string) string {
	var out []string
	for _, line := range strings.Split(panel, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "│") {
			continue
		}
		out = append(out, strings.TrimSpace(strings.Trim(line, "│")))
	}
	return strings.Join(out, " ")
}
