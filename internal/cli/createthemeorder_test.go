package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// An existing folder is reported before a bad name, because upstream checks it
// first.
//
// `create_theme_command.py:32-39` runs in one order and one only: the
// `new_theme_folder.exists()` guard raises at `:34`, `copy_templates` runs at
// `:36`, and the name pattern is not looked at until
// `create_init_file_for_theme` at `:39`. So `create-theme Bad` in a directory
// that already holds `Bad` is upstream's **already-exists** error, not its
// name-pattern one.
//
// The port checked the name first and reported the name error for the same
// input — measured against the vendored CLI, which reports
// `The theme folder "Bad" already exists!`.
//
// The other half of the same ordering — that the copy happens BEFORE the name
// is validated, so a rejected name still leaves thirteen files on disk — is
// covered by createthemesideeffect_test.go.
func TestCreateThemeReportsAnExistingFolderBeforeABadName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Mkdir(filepath.Join(dir, "Bad"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.CreateTheme(cli.CreateThemeOptions{ThemeName: "Bad"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("exit code = 0, want a failure")
	}

	flat := flatten(stdout.String())
	if !strings.Contains(flat, "already exists") {
		t.Errorf("stdout = %q, want upstream's already-exists message", stdout.String())
	}
	if strings.Contains(flat, "lowercase letters and digits") {
		t.Errorf("stdout = %q, want the name check to come second", stdout.String())
	}
}
