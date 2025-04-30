//go:build conformance

package yamlreader_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/lexer"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

// Dealias exists for one shape: a `*` a user meant literally, which YAML would
// otherwise resolve as an anchor reference. On every real document it must
// change nothing at all.
//
// Nothing in the suite guarded that. The transform runs inside the reader on
// every parse, so a bug in it would corrupt documents that contain no alias —
// which is all of them — and the shaped cases in noalias_test.go would still
// pass, because they only exercise inputs that *do* carry a `*`.
//
// Identity is asserted on the token stream rather than on the parsed tree. It
// is the stronger statement and the one the transform actually claims: equal
// token streams parse to equal trees, while equal trees would tolerate a
// transform that rewrites tokens and happens to parse the same.
func TestDealiasIsANoOpOnTheCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "third_party", "rendercv"))
	if err != nil {
		t.Fatalf("resolving the submodule: %v", err)
	}

	walked := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The virtualenv is vendored dependencies, not RenderCV's corpus.
			if d.Name() == ".venv" || d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		walked++

		relative, _ := filepath.Rel(root, path)
		before := lexer.Tokenize(string(source))
		after := yamlreader.Dealias(before)

		if len(after) != len(before) {
			t.Errorf("%s: Dealias changed the token count, %d -> %d",
				relative, len(before), len(after))
			return nil
		}
		for i := range before {
			if after[i] != before[i] {
				t.Errorf("%s: Dealias rewrote token %d: %q (%v) -> %q (%v)",
					relative, i, before[i].Origin, before[i].Type,
					after[i].Origin, after[i].Type)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the submodule: %v", err)
	}

	// A walk that found nothing would pass for the wrong reason — the submodule
	// carries 64 YAML files.
	if walked < 60 {
		t.Fatalf("walked %d YAML files, want the whole submodule corpus", walked)
	}
}
