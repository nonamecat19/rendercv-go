package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOverlayFlagsAreRead is spec 012 §2 behavior 7.
//
// **The three overlay options were declared and never read.** `BuildArguments`
// has carried `SettingsYaml`, `DesignYaml` and `LocaleYaml` since iteration 2
// and `buildArguments` left all three empty, so `--design d.yaml` parsed, was
// stored, and changed nothing about the document.
//
// The overlay file is a **whole document with the field as its top-level key**,
// not the field's own body: `-d` on a file reading `theme: moderncv` makes
// upstream die with `KeyError: 'design'`, and one reading `design: {theme:
// moderncv}` renders moderncv. Measured against the vendored CLI.
func TestOverlayFlagsAreRead(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	design := write("design.yaml", "design:\n  theme: moderncv\n")
	locale := write("locale.yaml", "locale:\n  language: french\n")
	settings := write("settings.yaml", "settings:\n  current_date: 2025-03-05\n")

	args, err := buildArguments(RenderOptions{
		DesignPath:   design,
		LocalePath:   locale,
		SettingsPath: settings,
	})
	if err != nil {
		t.Fatalf("buildArguments: %v", err)
	}

	for _, row := range []struct{ name, got, want string }{
		{"DesignYaml", args.DesignYaml, "design:\n  theme: moderncv\n"},
		{"LocaleYaml", args.LocaleYaml, "locale:\n  language: french\n"},
		{"SettingsYaml", args.SettingsYaml, "settings:\n  current_date: 2025-03-05\n"},
	} {
		if row.got != row.want {
			t.Errorf("%s = %q, want %q", row.name, row.got, row.want)
		}
	}
}

// TestUnsuppliedOverlayIsEmpty pins the other half: an option nobody passed must
// leave the overlay unset rather than reading some default file. Every corpus
// case takes this path, so a regression here breaks all 24.
func TestUnsuppliedOverlayIsEmpty(t *testing.T) {
	args, err := buildArguments(RenderOptions{})
	if err != nil {
		t.Fatalf("buildArguments: %v", err)
	}
	if args.DesignYaml != "" || args.LocaleYaml != "" || args.SettingsYaml != "" {
		t.Errorf("overlays = %q/%q/%q, want all empty",
			args.DesignYaml, args.LocaleYaml, args.SettingsYaml)
	}
}

// TestMissingOverlayIsReported covers a shape upstream does not handle: `-d`
// naming a file that is not there reaches `pathlib.read_text` unguarded and
// dies with a `FileNotFoundError` traceback, the same way a missing *input*
// file does. The port answers both with the one message it already had.
func TestMissingOverlayIsReported(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nosuch.yaml")

	_, err := buildArguments(RenderOptions{DesignPath: missing})
	if err == nil {
		t.Fatal("buildArguments returned no error for a missing overlay")
	}
	want := errMissingFile(missing).Error()
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
