package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// upstreamCreateThemeTemplates is the file set `copy_templates("typst", …)`
// leaves on disk (`cli/copy_templates.py`), measured against the vendored
// Python: thirteen files, four top-level fragments and the nine entry
// templates. `__init__.py` is not in it — upstream writes that afterwards, in
// `create_init_file_for_theme`, and the port writes `init.lua` there (D-008).
var upstreamCreateThemeTemplates = []string{
	"Header.j2.typ",
	"Preamble.j2.typ",
	"SectionBeginning.j2.typ",
	"SectionEnding.j2.typ",
	"entries/BulletEntry.j2.typ",
	"entries/EducationEntry.j2.typ",
	"entries/ExperienceEntry.j2.typ",
	"entries/NormalEntry.j2.typ",
	"entries/NumberedEntry.j2.typ",
	"entries/OneLineEntry.j2.typ",
	"entries/PublicationEntry.j2.typ",
	"entries/ReversedNumberedEntry.j2.typ",
	"entries/TextEntry.j2.typ",
}

// A rejected theme name still leaves the copied templates on disk, because
// upstream copies before it validates.
//
// `create_theme_command.py:32-39`: the `exists()` guard raises at `:34`,
// `copy_templates` runs at `:36`, and the name pattern is not looked at until
// `create_init_file_for_theme` at `:39`. So the copy is already done when the
// name is rejected. Measured against the vendored CLI in an empty directory:
// `create-theme MyTheme` exits 1 and leaves the thirteen files below — and
// leaves `__init__.py` out, since the raise happens before it is written.
//
// The port validated first and left nothing, which is a filesystem-side-effect
// divergence the parity contract does not sanction (AGENTS.md §2).
func TestCreateThemeCopiesTemplatesBeforeRejectingTheName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "MyTheme"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}
	if flat := flatten(stdout.String()); !strings.Contains(flat, "lowercase letters and digits") {
		t.Errorf("stdout = %q, want upstream's name-pattern message", stdout.String())
	}

	got := filesUnder(t, filepath.Join(dir, "MyTheme"))
	if !slices.Equal(got, slices.Sorted(slices.Values(upstreamCreateThemeTemplates))) {
		t.Errorf("files on disk = %v, want upstream's thirteen %v",
			got, upstreamCreateThemeTemplates)
	}
	if slices.Contains(got, "init.lua") {
		t.Error("init.lua was written; upstream raises before __init__.py exists")
	}
}

// A relative theme name that climbs out of the working directory writes there,
// because upstream's copy is not confined to the working directory either.
//
// Measured: `create-theme ../escaped` upstream exits 1 (the name is rejected by
// the same pattern) and leaves all thirteen templates in the PARENT directory.
// This is upstream behaviour, deliberately reproduced, not an accident — see
// the note on `CreateTheme`.
func TestCreateThemeWritesOutsideTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)

	var stdout, stderr bytes.Buffer
	code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "../escaped"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}

	if inside := filesUnder(t, work); len(inside) != 0 {
		t.Errorf("files inside the working directory = %v, want none", inside)
	}
	got := filesUnder(t, filepath.Join(root, "escaped"))
	if !slices.Equal(got, slices.Sorted(slices.Values(upstreamCreateThemeTemplates))) {
		t.Errorf("files beside the working directory = %v, want upstream's thirteen %v",
			got, upstreamCreateThemeTemplates)
	}
}

// filesUnder lists dir's files as slash-separated paths relative to it, sorted.
// A missing directory is an empty list, which is what "the port wrote nothing"
// looks like.
func filesUnder(t *testing.T, dir string) []string {
	t.Helper()

	var names []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	slices.Sort(names)
	return names
}
