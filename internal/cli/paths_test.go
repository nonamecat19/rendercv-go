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
	input := PathInput{Name: "John Doe", OutputFolder: "rendercv_output"}

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
