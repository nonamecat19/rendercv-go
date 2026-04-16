package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/cli"
)

// unreachableFileMessages are `read_yaml`'s two file-level messages
// (`schema/yaml_reader.py:36`, `:42-46`), spec 013 §4.11.
//
// **They cannot be reached from upstream's CLI at all.** `read_yaml` takes a
// `pathlib.Path | str` (`yaml_reader.py:11`) and only the `Path` branch checks
// existence and the extension whitelist; the render path's only caller
// (`schema/rendercv_model_builder.py:85`) passes the file's **contents** as a
// string, so neither check ever runs. A port that implements either message
// reachably is stricter than upstream and diverges (spec 013 behavior 43).
var unreachableFileMessages = []string{
	"doesn't exist!",
	"The input file should have one of the following extensions",
}

// TestRenderAcceptsAnyExtension is behavior 43, measured against the vendored
// CLI: a valid CV named `ok.txt` renders at **exit 0**, because the extension
// whitelist is on a branch the CLI never takes.
func TestRenderAcceptsAnyExtension(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(input, []byte("cv:\n  name: John Doe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")

	var stdout, stderr bytes.Buffer
	code := cli.Render(cli.RenderOptions{
		InputPath: input, OutputFolder: out,
		NoPDF: true, NoPNG: true,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr = %q", code, stderr.String())
	}
	if entries, _ := os.ReadDir(out); len(entries) == 0 {
		t.Error("no artifacts written")
	}
}

// TestUnreachableFileMessagesAreAbsent is spec 013 §8's textual criterion:
// neither §4.11 string appears anywhere in the port's source.
//
// A reachability test alone cannot hold this. The messages sat in
// `internal/schema/yamlreader.ReadFile` — a function only tests call — where no
// invocation could surface them and no invocation could prove them gone either.
// Scanning the source is the only assertion that fails if some later caller
// makes that branch reachable again.
func TestUnreachableFileMessagesAreAbsent(t *testing.T) {
	root := repoRoot(t)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The vendored Python is the specification and owns both strings.
			// .claude holds this repo's own worktree copies (agent isolation,
			// .claude/worktrees/*) — scanning them double-reports whatever this
			// test finds in the real tree, or reports whatever an in-flight
			// agent's tree currently contains.
			switch d.Name() {
			case "third_party", ".git", "testdata", ".claude":
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || path == thisFile(t) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, message := range unreachableFileMessages {
			if strings.Contains(string(content), message) {
				t.Errorf("%s contains the unreachable message %q", rel, message)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// thisFile is the one file allowed to name the messages, since it is what
// asserts their absence.
func thisFile(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "reachability_test.go")
}
