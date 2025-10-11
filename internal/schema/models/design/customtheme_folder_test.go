package design_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/design"
)

// The two folder checks of `design.py:72-86`. The name check that precedes them
// shipped in iteration 6 and is tested there. A verifier found the port
// rendering happily where upstream reports both of these.
func TestValidateCustomThemeFolder(t *testing.T) {
	dir := t.TempDir()

	// A theme folder with a template, which is the passing case.
	good := filepath.Join(dir, "mytheme", "entries")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatal(err)
	}
	// **Recursive**: the only template is a subdirectory down, and upstream's
	// `rglob` still finds it.
	if err := os.WriteFile(filepath.Join(good, "TextEntry.j2.typ"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A folder with no template at all.
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	// **A regular file still `exists()` upstream.** `pathlib.Path.exists()`
	// is true for a file, not just a directory, so a theme name that
	// resolves to a plain file falls through to the *.j2.typ check and gets
	// that message — not "does not exist", which `os.Stat(...).IsDir()`
	// used to report here. Found by a fresh-context verifier (iteration
	// 14's seventeenth re-verification).
	if err := os.WriteFile(filepath.Join(dir, "filetheme"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// **A symlinked theme folder must still be scanned through.**
	// `filepath.WalkDir` `Lstat`s its root, so a symlinked root reads as
	// "not a directory" and is never descended into — upstream's `rglob`
	// resolves symlinks by default. Found by a fresh-context verifier
	// (iteration 14's non-colour-design-slice sweep).
	if err := os.Symlink(good, filepath.Join(dir, "symlinktheme")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	cases := []struct{ name, theme, want string }{
		{"a good folder passes", "mytheme", ""},
		{"a missing folder is reported", "absent", "does not exist"},
		{"a folder with no template is reported", "empty", "*.j2.typ files"},
		{"a regular file is reported as no templates, not missing", "filetheme", "*.j2.typ files"},
		{"a symlinked folder is scanned through", "symlinktheme", ""},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			err := design.ValidateCustomThemeFolder(row.theme, dir)
			if row.want == "" {
				if err != nil {
					t.Errorf("= %v, want no error", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), row.want) {
				t.Errorf("= %v, want it to mention %q", err, row.want)
			}
		})
	}
}

// **The message's path must keep a `..` unresolved, not collapse it.**
// `filepath.Abs` calls `Clean`; upstream's `Path.absolute()` only prepends
// the working directory and leaves the rest of the path exactly as
// written. Found by a fresh-context verifier (iteration 14's
// non-colour-design-slice sweep).
func TestCustomThemeFolderMessageKeepsDotDotUnresolved(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	err = design.ValidateCustomThemeFolder("mytheme", "../elsewhere")
	if err == nil {
		t.Fatal("= nil, want a missing-folder error")
	}
	want := sub + "/../elsewhere/mytheme"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("= %v, want it to contain the unresolved path %q", err, want)
	}
}
