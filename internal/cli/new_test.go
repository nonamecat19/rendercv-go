package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// TestNewCreateTypstTemplates is G-9's other half: `--create-typst-templates`
// already parsed, but nothing gated it. Pins that it writes the theme folder
// (defaulting to "classic", upstream's own default) and that the panel grows
// an "Also created" section naming it.
func TestNewCreateTypstTemplates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.New(cli.NewOptions{Name: "John Doe", CreateTypstTemplates: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "classic", "Header.j2.typ")); err != nil {
		t.Errorf("classic/Header.j2.typ: %v", err)
	}
	if !strings.Contains(stdout.String(), "Also created:") ||
		!strings.Contains(stdout.String(), "Typst templates: ./classic") {
		t.Errorf("stdout missing the Typst templates row:\n%s", stdout.String())
	}
}

// TestNewCreateMarkdownTemplates is G-9: the flag parsed and did nothing.
// Pins the fixed "markdown" folder name (`new_command.py:87` — not derived
// from any flag) and the twelve files `copy_templates("markdown", …)` writes.
func TestNewCreateMarkdownTemplates(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.New(cli.NewOptions{Name: "John Doe", CreateMarkdownTemplates: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	want := []string{
		"Header.j2.md", "SectionBeginning.j2.md", "SectionEnding.j2.md",
		"entries/BulletEntry.j2.md", "entries/EducationEntry.j2.md",
		"entries/ExperienceEntry.j2.md", "entries/NormalEntry.j2.md",
		"entries/NumberedEntry.j2.md", "entries/OneLineEntry.j2.md",
		"entries/PublicationEntry.j2.md", "entries/ReversedNumberedEntry.j2.md",
		"entries/TextEntry.j2.md",
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(dir, "markdown", rel)); err != nil {
			t.Errorf("markdown/%s: %v", rel, err)
		}
	}
	if !strings.Contains(stdout.String(), "Markdown templates: ./markdown") {
		t.Errorf("stdout missing the Markdown templates row:\n%s", stdout.String())
	}
}

// TestNewTemplatesReportsExistingFolders is the panel-shape half of G-9's fix:
// upstream's `new_command.py:117-166` collects every requested template kind
// into one created/existing list before printing, so one flag creating a
// folder and the other finding one already there must land in the same panel
// with both an "Also created" and a "Not modified" section.
func TestNewTemplatesReportsExistingFolders(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(filepath.Join(dir, "markdown"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.New(cli.NewOptions{
		Name: "John Doe", CreateTypstTemplates: true, CreateMarkdownTemplates: true,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Also created:") || !strings.Contains(out, "Typst templates: ./classic") {
		t.Errorf("stdout missing the created Typst row:\n%s", out)
	}
	if !strings.Contains(out, "Not modified (already exist):") ||
		!strings.Contains(out, "Markdown templates: ./markdown") {
		t.Errorf("stdout missing the existing Markdown row:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "classic")); err != nil {
		t.Errorf("classic/: %v", err)
	}
}
