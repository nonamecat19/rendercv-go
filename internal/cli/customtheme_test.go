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

// TestCreateThemeWritesTheFileSet is G-5: `create-theme` was unregistered and
// exited 2 with "No such command". Pins D-008's fourteen files — the thirteen
// pongo2-transformed `.typ` templates plus `init.lua` in place of upstream's
// `__init__.py` — so a regression that drops a file or reverts to writing
// `__init__.py` fails a test rather than waiting for another verifier pass.
func TestCreateThemeWritesTheFileSet(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	if code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "mytheme"},
		&stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	want := []string{
		"Header.j2.typ", "Preamble.j2.typ", "SectionBeginning.j2.typ", "SectionEnding.j2.typ",
		"init.lua",
		"entries/BulletEntry.j2.typ", "entries/EducationEntry.j2.typ",
		"entries/ExperienceEntry.j2.typ", "entries/NormalEntry.j2.typ",
		"entries/NumberedEntry.j2.typ", "entries/OneLineEntry.j2.typ",
		"entries/PublicationEntry.j2.typ", "entries/ReversedNumberedEntry.j2.typ",
		"entries/TextEntry.j2.typ",
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(dir, "mytheme", rel)); err != nil {
			t.Errorf("mytheme/%s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mytheme", "__init__.py")); err == nil {
		t.Error("mytheme/__init__.py exists — D-008 says init.lua replaces it, not sits beside it")
	}
}

// TestCreateThemeRoundTrips is D-008's central claim made executable: a theme
// `create-theme` writes must be a theme `render` can load. Renders a document
// naming the freshly-written folder and requires it to succeed — the same
// check `TestRenderReportsAMissingThemeFolder` exists to fail without.
func TestCreateThemeRoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var ctOut, ctErr bytes.Buffer
	if code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "mytheme"},
		&ctOut, &ctErr); code != 0 {
		t.Fatalf("create-theme exit = %d, stderr = %q", code, ctErr.String())
	}

	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte(
		"cv:\n  name: John Doe\ndesign:\n  theme: mytheme\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("render exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

// TestCreateThemeRejectsBadName pins upstream's `custom_theme_name_pattern`
// message (`design.py:60`), and TestCreateThemeRejectsExistingFolder pins the
// other guard `create_theme_command.py` runs before writing anything.
func TestCreateThemeRejectsBadName(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "Bad_Name"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}
	if !strings.Contains(flatten(stdout.String()), "lowercase letters and digits") {
		t.Errorf("stdout = %q, want upstream's name-pattern message", stdout.String())
	}
}

func TestCreateThemeRejectsExistingFolder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(filepath.Join(dir, "mytheme"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "mytheme"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}
	if !strings.Contains(flatten(stdout.String()), "already exists") {
		t.Errorf("stdout = %q, want upstream's already-exists message", stdout.String())
	}
}

// TestRenderReportsAMissingInputFile pins the shape D-011 says the port
// produces instead of upstream's `err_missing_file` traceback: an `Error`
// panel on stdout naming the file, exit 1, nothing on stderr. The corpus case
// itself stays red forever (it compares against a Python stack trace), so
// this is what actually gates the port's own behavior on that vector.
func TestRenderReportsAMissingInputFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does_not_exist.yaml")

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: missing, OutputFolder: filepath.Join(dir, "out"),
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code == 0 {
		t.Error("exit code = 0, want a failure")
	}
	if !strings.Contains(flatten(stdout.String()), "does not exist") {
		t.Errorf("stdout = %q, want a does-not-exist message", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing written", stderr.String())
	}
}

// TestRenderReportsAnUnknownOverrideKey pins the shape D-011 says the port
// produces instead of upstream's `err_bad_override_key` traceback: the same
// validation-error table every other unknown-field case gets, exit 1.
func TestRenderReportsAnUnknownOverrideKey(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "cv.yaml")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: filepath.Join(dir, "out"),
		Extras: []string{"--no_such_field", "value"},
		NoPDF:  true, NoPNG: true,
	}, &stdout, &stderr)

	if code == 0 {
		t.Error("exit code = 0, want a failure")
	}
	if !strings.Contains(flatten(stdout.String()), "This field is unknown for this object") {
		t.Errorf("stdout = %q, want the unknown-field validation table", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing written", stderr.String())
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
