package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/renderer/generate"
)

// The whole file turns on one distinction.
//
// Upstream derives every input-relative path from `pathlib`'s
// **`PurePath.parent`** and `PurePath.__truediv__`, both of which are *purely
// lexical*: they parse the string, drop `.` and empty segments, and leave a
// `..` segment exactly where it was. `Path('./bb/../bb/CV.yaml').parent` is
// `bb/../bb`.
//
// Go's `filepath.Dir`, `filepath.Join`, `filepath.Abs` and `filepath.Rel` all
// call `Clean`, which **resolves `..` against its neighbour** — `bb/../bb`
// becomes `bb`. On an ordinary tree the two name the same directory, so the
// difference is invisible. **Through a symlink they do not**: with `work/bb`
// pointing at `other/real` and `other/bb` a different real directory,
// `bb/../bb` is `other/bb` to the kernel while `Clean`'s `bb` is `other/real`.
//
// So `filepath.Dir` is the idiomatic call and it is the wrong one here. Anyone
// "simplifying" these back to it reintroduces a silent, exit-0 divergence that
// writes a different document into a different directory.

// symlinkTree builds the demonstration layout and returns the root, the
// lexical directory (`other/bb`), the cleaned one (`other/real`) and the input
// path a user would type.
//
// **The input path is assembled by hand**, because `filepath.Join` would clean
// it: the `..` segment is the whole point of the vector, and `Join` collapses
// it before the code under test ever sees it.
func symlinkTree(t *testing.T) (root, lexical, cleaned, input string) {
	t.Helper()

	root = t.TempDir()
	work := filepath.Join(root, "work")
	lexical = filepath.Join(root, "other", "bb")
	cleaned = filepath.Join(root, "other", "real")
	for _, dir := range []string{work, lexical, cleaned} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(cleaned, filepath.Join(work, "bb")); err != nil {
		t.Skipf("this filesystem does not do symlinks: %v", err)
	}
	return root, lexical, cleaned, work + "/bb/../bb/CV.yaml"
}

// TestOutputFolderIsLexical is site 1: `output_folder` is a
// `PlannedPathRelativeToInput`, resolved at `schema/models/path.py:39-41` as
// `input_file_path.parent / path` — `PurePath.parent` and `PurePath`'s `/`,
// neither of which cleans. `outputFolderFor` used `filepath.Join(filepath.Dir(
// …))`, two `Clean`s in one line.
func TestOutputFolderIsLexical(t *testing.T) {
	_, _, _, input := symlinkTree(t)

	got := generate.OutputFolderFor(input, "")
	want := strings.TrimSuffix(input, "/CV.yaml") + "/" + DefaultOutputFolder
	if got != want {
		t.Errorf("outputFolderFor = %q, want %q", got, want)
	}
}

// TestOutputFolderKeepsAnAbsoluteFolderAsGiven pins the branch the fix must not
// disturb: an absolute `--output-folder` is taken as it stands.
func TestOutputFolderKeepsAnAbsoluteFolderAsGiven(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "out")
	if got := generate.OutputFolderFor("sub/cv.yaml", absolute); got != absolute {
		t.Errorf("outputFolderFor = %q, want %q", got, absolute)
	}
}

// TestResolvePathKeepsTheLexicalParent is site 2, and it is what would silently
// undo site 1: `path_resolver.py:108` is `file_path.parent / file_name`, the
// lexical pair again, and `ResolvePath` used `filepath.Join`, which cleans the
// `..` back out of the folder it was just handed.
func TestResolvePathKeepsTheLexicalParent(t *testing.T) {
	_, lexical, cleaned, input := symlinkTree(t)

	folder := strings.TrimSuffix(input, "/CV.yaml") + "/rendercv_output"
	name := "John Doe"
	got, err := ResolvePath(DefaultTypstPath, PathInput{OutputFolder: folder, Name: &name})
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if want := folder + "/John_Doe_CV.typ"; got != want {
		t.Errorf("ResolvePath = %q, want %q", got, want)
	}

	// The lexical spelling must also *reach* the lexical directory: the
	// parent-creating half of `:109` runs on the uncleaned path.
	if _, err := os.Stat(filepath.Join(lexical, "rendercv_output")); err != nil {
		t.Errorf("the output folder was not created under %s: %v", lexical, err)
	}
	if _, err := os.Stat(filepath.Join(cleaned, "rendercv_output")); err == nil {
		t.Errorf("the output folder was created under the cleaned path %s", cleaned)
	}
}

// TestDisplayKeepsTheLexicalPath is site 6, which is independent of where the
// file lands: the panel prints `path.relative_to(pathlib.Path.cwd())`
// (`cli/render_command/progress_panel.py:97`), and `PurePath.relative_to` is a
// lexical prefix strip that keeps the `..`. `filepath.Rel` cleans both of its
// arguments, so the panel would lose the segment even once the file is written
// to the right place.
func TestDisplayKeepsTheLexicalPath(t *testing.T) {
	root, _, _, input := symlinkTree(t)
	t.Chdir(root)

	path := strings.TrimSuffix(input, "/CV.yaml") + "/rendercv_output/John_Doe_CV.typ"
	want := "./work/bb/../bb/rendercv_output/John_Doe_CV.typ"
	if got := display(path); got != want {
		t.Errorf("display = %q, want %q", got, want)
	}
}

// TestDocumentNamedOverlayResolvesLikeUpstream is site 5, and its target is not
// site 1's. `run_rendercv.py:120,122` is
// `(input_file_path.parent / rc["design"]).resolve()` — the lexical parent and
// join, **and then a full `resolve()`**, which follows symlinks. So this one
// site ends at the real path.
//
// The file opened is the same either way, because the kernel resolves the
// lexical spelling at open time; the string is what differs, and the string is
// what the watch set holds and what a message would name.
func TestDocumentNamedOverlayResolvesLikeUpstream(t *testing.T) {
	_, lexical, _, input := symlinkTree(t)

	for _, name := range []string{"mydesign.yaml", "mylocale.yaml"} {
		if err := os.WriteFile(filepath.Join(lexical, name), []byte("design: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	options := RenderOptions{InputPath: input}
	raw := []byte("cv:\n  name: John Doe\nsettings:\n  render_command:\n" +
		"    design: mydesign.yaml\n    locale: mylocale.yaml\n")
	if err := resolveNamedOverlays(&options, raw); err != nil {
		t.Fatalf("resolveNamedOverlays: %v", err)
	}

	// `Path.resolve()`'s answer, computed the same way the fix must compute it.
	wantDesign, err := filepath.EvalSymlinks(filepath.Join(lexical, "mydesign.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	wantLocale, err := filepath.EvalSymlinks(filepath.Join(lexical, "mylocale.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if options.DesignPath != wantDesign {
		t.Errorf("DesignPath = %q, want %q", options.DesignPath, wantDesign)
	}
	if options.LocalePath != wantLocale {
		t.Errorf("LocalePath = %q, want %q", options.LocalePath, wantLocale)
	}
}

// TestRenderWritesToTheLexicalOutputFolder is the end-to-end vector, measured
// against the vendored binary: `render ./bb/../bb/CV.yaml` writes
// `other/bb/rendercv_output/John_Doe_CV.typ` upstream and wrote
// `other/real/rendercv_output/John_Doe_CV.typ` here, both at exit 0 with
// nothing warning the user. The panel line differs with it.
func TestRenderWritesToTheLexicalOutputFolder(t *testing.T) {
	root, lexical, cleaned, input := symlinkTree(t)
	t.Chdir(root)

	if err := os.WriteFile(filepath.Join(lexical, "CV.yaml"), []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if code := Render(RenderOptions{InputPath: input, NoPDF: true, NoPNG: true}, &out, &out); code != 0 {
		t.Fatalf("Render = %d, output %q", code, out.String())
	}

	if _, err := os.Stat(filepath.Join(lexical, "rendercv_output", "John_Doe_CV.typ")); err != nil {
		t.Errorf("nothing was written under the lexical path %s: %v", lexical, err)
	}
	if _, err := os.Stat(filepath.Join(cleaned, "rendercv_output")); err == nil {
		t.Errorf("the render wrote under the cleaned path %s", cleaned)
	}
	if want := "./work/bb/../bb/rendercv_output/John_Doe_CV.typ"; !strings.Contains(out.String(), want) {
		t.Errorf("panel = %q, want it to name %q", out.String(), want)
	}
}
