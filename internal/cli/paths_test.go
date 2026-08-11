package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// Spec 012 §2 behaviors 5 and 7, and `path_resolver.py:40-109`.
func TestResolvePath(t *testing.T) {
	// ResolvePath creates the parent directories, so the test runs in a scratch
	// directory rather than the package's.
	t.Chdir(t.TempDir())
	name := "John Doe"
	input := PathInput{Name: &name, OutputFolder: "rendercv_output"}

	cases := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "the default typst path",
			template: "OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ",
			want:     "rendercv_output/John_Doe_CV.typ",
		},
		{
			// The seven name spellings are one substitution pass, so the longest
			// placeholder has to win over the prefixes it contains — `NAME` is a
			// prefix of every other one.
			name:     "the longer name placeholders win",
			template: "OUTPUT_FOLDER/NAME_IN_LOWER_KEBAB_CASE.md",
			want:     "rendercv_output/john-doe.md",
		},
		{
			// `render_custom_paths` writes into a directory that does not exist.
			name:     "a path with no OUTPUT_FOLDER is left alone",
			template: "out/nested/custom.md",
			want:     "out/nested/custom.md",
		},
	}

	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			got, err := ResolvePath(row.template, input)
			if err != nil {
				t.Fatalf("ResolvePath: %v", err)
			}
			if filepath.ToSlash(got) != row.want {
				t.Errorf("= %q, want %q", filepath.ToSlash(got), row.want)
			}
		})
	}
}

// **A nameless CV keeps the placeholder literal**, because upstream filters the
// `None` values out of the substitution table rather than substituting an empty
// string — the difference between `John_Doe_CV.typ` and `_CV.typ`.
func TestResolvePathWithNoName(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := ResolvePath("OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ",
		PathInput{OutputFolder: "rendercv_output"})
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(got), "NAME_IN_SNAKE_CASE_CV.typ") {
		t.Errorf("= %q, want the placeholder kept", got)
	}
}

// An **empty-string** name is not an absent one, and the two write different
// files. Only the six derived spellings are guarded by `if cv.name`
// (`path_resolver.py:77-102`); plain `NAME` is the value itself (`:76`), and
// `""` is not `None`, so it stays in the table and substitutes to nothing —
// leaving the rest of the longer placeholder's literal text behind. Measured
// against the vendored Python: `cv.name: ""` writes
// `rendercv_output/_IN_SNAKE_CASE_CV.typ`.
func TestResolvePathWithAnEmptyName(t *testing.T) {
	t.Chdir(t.TempDir())
	empty := ""
	got, err := ResolvePath("OUTPUT_FOLDER/NAME_IN_SNAKE_CASE_CV.typ",
		PathInput{Name: &empty, OutputFolder: "rendercv_output"})
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if want := "rendercv_output/_IN_SNAKE_CASE_CV.typ"; filepath.ToSlash(got) != want {
		t.Errorf("= %q, want %q", filepath.ToSlash(got), want)
	}
}

// TestOutputFolderResolvesAgainstInputDir is G-8: upstream types
// `output_folder` `PlannedPathRelativeToInput`
// (`schema/models/settings/render_command.py:30`), so `render sub/cv.yaml -o
// pyO` writes `sub/pyO/`, not `./pyO/`. Measured before the fix: the port
// wrote beside the working directory instead.
func TestOutputFolderResolvesAgainstInputDir(t *testing.T) {
	cases := []struct {
		name    string
		options RenderOptions
		want    string
	}{
		{
			name:    "default folder, relative input",
			options: RenderOptions{InputPath: "sub/cv.yaml"},
			want:    filepath.Join("sub", DefaultOutputFolder),
		},
		{
			name:    "explicit relative folder",
			options: RenderOptions{InputPath: "sub/cv.yaml", OutputFolder: "pyO"},
			want:    filepath.Join("sub", "pyO"),
		},
		{
			name:    "input in the working directory",
			options: RenderOptions{InputPath: "cv.yaml", OutputFolder: "out"},
			want:    "out",
		},
		{
			name:    "an absolute folder is taken as given",
			options: RenderOptions{InputPath: "sub/cv.yaml", OutputFolder: "/tmp/abs-out"},
			want:    "/tmp/abs-out",
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := outputFolderFor(row.options); filepath.ToSlash(got) != filepath.ToSlash(row.want) {
				t.Errorf("= %q, want %q", got, row.want)
			}
		})
	}
}

// `OUTPUT_FOLDER` is a path **component**, so a file whose name merely contains
// the word is not rewritten.
func TestOutputFolderIsAComponent(t *testing.T) {
	if got := resolveOutputFolder("a/MY_OUTPUT_FOLDER.typ", "out"); got != "a/MY_OUTPUT_FOLDER.typ" {
		t.Errorf("= %q, want it untouched", got)
	}
	if got := resolveOutputFolder("a/OUTPUT_FOLDER/b.typ", "out"); got != "a/out/b.typ" {
		t.Errorf("= %q, want the component replaced", got)
	}
}
